package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/email"
	"mycli.sh/api/internal/store"
)

type AuthHandler struct {
	cfg   *config.Config
	store store.AuthStore
	email email.Sender
}

func NewAuthHandler(cfg *config.Config, s store.AuthStore, emailSender email.Sender) *AuthHandler {
	return &AuthHandler{
		cfg:   cfg,
		store: s,
		email: emailSender,
	}
}

func (h *AuthHandler) StartDeviceFlow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "email is required")
		return
	}
	if !validEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "invalid email format")
		return
	}

	deviceCode := generateCode(32)
	userCode := generateUserCode()
	ctx := r.Context()

	// Opportunistic cleanup of expired sessions
	_ = h.store.DeleteExpiredDeviceSessions(ctx)

	expiresAt := time.Now().Add(15 * time.Minute)

	// Create device session and magic link atomically
	magicToken := generateCode(32)
	tokenHash := hashToken(magicToken)
	otp := generateOTP()
	otpHash := hashToken(otp)

	if err := h.store.WithTx(ctx, func(tx store.AuthStore) error {
		if err := tx.CreateDeviceSession(ctx, deviceCode, userCode, req.Email, expiresAt); err != nil {
			return err
		}
		if _, err := tx.CreateMagicLink(ctx, req.Email, tokenHash, deviceCode, &otpHash, expiresAt); err != nil {
			return err
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create device session")
		return
	}

	// Send email outside tx (external side effect)
	verifyURL := h.cfg.BaseURL + "/v1/auth/verify?token=" + magicToken
	emailSent := true
	if err := h.email.SendVerification(email.EmailParams{
		To:        req.Email,
		VerifyURL: verifyURL,
		OTPCode:   otp,
	}); err != nil {
		emailSent = false
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":      deviceCode,
		"user_code":        userCode,
		"verification_uri": h.cfg.BaseURL + "/device",
		"expires_in":       900,
		"interval":         5,
		"email_sent":       emailSent,
	})
}

func (h *AuthHandler) PollDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	ctx := r.Context()
	session, err := h.store.GetDeviceSessionByCode(ctx, req.DeviceCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "device code expired or invalid")
		return
	}
	if !session.Authorized || session.UserID == nil {
		writeError(w, http.StatusBadRequest, "AUTHORIZATION_PENDING", "waiting for user authorization")
		return
	}
	userID := *session.UserID

	// Consume the device session
	if err := h.store.DeleteDeviceSession(ctx, req.DeviceCode); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to consume device session")
		return
	}

	// Generate tokens
	accessToken, err := h.generateJWT(userID, "access", time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	refreshToken, err := h.generateJWT(userID, "refresh", 30*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	// Create session
	var sessionID string
	refreshTokenHash := hashToken(refreshToken)
	userAgent := r.UserAgent()
	ipAddress := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ipAddress = strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}
	if sess, err := h.store.CreateSession(ctx, userID, refreshTokenHash, userAgent, ipAddress, time.Now().Add(30*24*time.Hour)); err == nil {
		sessionID = sess.ID
	}

	resp := map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    3600,
	}
	if sessionID != "" {
		resp["session_id"] = sessionID
	}

	// Check if user needs to set a username
	if user, err := h.store.GetUserByID(ctx, userID); err == nil {
		resp["needs_username"] = user.Username == nil
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	token, err := jwt.Parse(req.RefreshToken, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid refresh token")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid claims")
		return
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "not a refresh token")
		return
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token subject")
		return
	}

	// Validate refresh token against sessions table
	refreshTokenHash := hashToken(req.RefreshToken)
	sess, err := h.store.GetSessionByTokenHash(r.Context(), refreshTokenHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "session not found")
		return
	}
	if sess.RevokedAt != nil {
		writeError(w, http.StatusUnauthorized, "SESSION_REVOKED", "session has been revoked")
		return
	}
	_ = h.store.UpdateSessionLastUsed(r.Context(), sess.ID)

	accessToken, err := h.generateJWT(sub, "access", time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"expires_in":   3600,
	})
}

// Device verification HTML pages

func (h *AuthHandler) DevicePage(w http.ResponseWriter, r *http.Request) {
	_ = devicePageTmpl.Execute(w, map[string]string{"BaseURL": h.cfg.BaseURL})
}

func (h *AuthHandler) DeviceSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	userCode := r.FormValue("user_code")
	userEmail := r.FormValue("email")

	if userCode == "" || userEmail == "" {
		http.Error(w, "user code and email are required", http.StatusBadRequest)
		return
	}

	if !validEmail(userEmail) {
		http.Error(w, "invalid email format", http.StatusBadRequest)
		return
	}

	// Find device session by user code
	ctx := r.Context()
	ds, err := h.store.GetDeviceSessionByUserCode(ctx, userCode)
	if err != nil {
		http.Error(w, "invalid or expired user code", http.StatusBadRequest)
		return
	}

	// Generate magic link token + OTP and persist
	magicToken := generateCode(32)
	tokenHash := hashToken(magicToken)
	otp := generateOTP()
	otpHash := hashToken(otp)

	_, err = h.store.CreateMagicLink(ctx, userEmail, tokenHash, ds.DeviceCode, &otpHash, time.Now().Add(15*time.Minute))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Send verification email
	verifyURL := h.cfg.BaseURL + "/v1/auth/verify?token=" + magicToken
	if err := h.email.SendVerification(email.EmailParams{
		To:        userEmail,
		VerifyURL: verifyURL,
		OTPCode:   otp,
	}); err != nil {
		http.Error(w, "failed to send verification email", http.StatusInternalServerError)
		return
	}

	_ = deviceSentTmpl.Execute(w, nil)
}

func (h *AuthHandler) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	tokenHash := hashToken(rawToken)
	ctx := r.Context()

	ml, err := h.store.GetMagicLinkByTokenHash(ctx, tokenHash)
	if err != nil {
		http.Error(w, "invalid or expired link", http.StatusBadRequest)
		return
	}

	if ml.UsedAt != nil || time.Now().After(ml.ExpiresAt) {
		http.Error(w, "link already used or expired", http.StatusBadRequest)
		return
	}

	if err := h.store.MarkMagicLinkUsed(ctx, ml.ID); err != nil {
		http.Error(w, "link already used or expired", http.StatusBadRequest)
		return
	}

	// Find or create user by email
	user, err := h.store.GetUserByEmail(ctx, ml.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			user, err = h.store.CreateUser(ctx, ml.Email)
			if err != nil {
				http.Error(w, "failed to create user", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	// Authorize the device session
	if err := h.store.AuthorizeDeviceSession(ctx, ml.DeviceCode, user.ID); err != nil {
		http.Error(w, "failed to authorize device session", http.StatusInternalServerError)
		return
	}

	_ = verifiedTmpl.Execute(w, nil)
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceCode string `json:"device_code"`
		Code       string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.DeviceCode == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "device_code and code are required")
		return
	}

	ctx := r.Context()

	// Check device session exists
	session, err := h.store.GetDeviceSessionByCode(ctx, req.DeviceCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "device code expired or invalid")
		return
	}

	// Brute-force protection: max 5 OTP attempts per device_code
	if session.OTPAttempts >= 5 {
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "too many verification attempts")
		return
	}

	// Look up the magic link by OTP hash
	otpHash := hashToken(req.Code)
	ml, err := h.store.GetMagicLinkByOTPHash(ctx, otpHash)
	if err != nil {
		_, _ = h.store.IncrementDeviceOTPAttempts(ctx, req.DeviceCode)
		writeError(w, http.StatusBadRequest, "INVALID_CODE", "invalid or expired code")
		return
	}

	// Verify the magic link belongs to this device code
	if ml.DeviceCode != req.DeviceCode {
		_, _ = h.store.IncrementDeviceOTPAttempts(ctx, req.DeviceCode)
		writeError(w, http.StatusBadRequest, "INVALID_CODE", "invalid or expired code")
		return
	}

	// Mark the magic link as used
	if err := h.store.MarkMagicLinkUsed(ctx, ml.ID); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CODE", "code already used or expired")
		return
	}

	// Find or create user by email
	user, err := h.store.GetUserByEmail(ctx, ml.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			user, err = h.store.CreateUser(ctx, ml.Email)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create user")
				return
			}
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
			return
		}
	}

	// Authorize device session
	if err := h.store.AuthorizeDeviceSession(ctx, req.DeviceCode, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to authorize device session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"authorized": true})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceCode string `json:"device_code"`
		Email      string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.DeviceCode == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "device_code and email are required")
		return
	}

	if !validEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "invalid email format")
		return
	}

	ctx := r.Context()

	// Check device session exists
	_, err := h.store.GetDeviceSessionByCode(ctx, req.DeviceCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "device code expired or invalid")
		return
	}

	// Reset OTP attempts and extend expiry
	if err := h.store.ResetDeviceOTPAndExtend(ctx, req.DeviceCode, time.Now().Add(15*time.Minute)); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to reset verification")
		return
	}

	// Generate new magic link + OTP
	magicToken := generateCode(32)
	tokenHash := hashToken(magicToken)
	otp := generateOTP()
	otpHash := hashToken(otp)

	_, err = h.store.CreateMagicLink(ctx, req.Email, tokenHash, req.DeviceCode, &otpHash, time.Now().Add(15*time.Minute))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create magic link")
		return
	}

	verifyURL := h.cfg.BaseURL + "/v1/auth/verify?token=" + magicToken
	if err := h.email.SendVerification(email.EmailParams{
		To:        req.Email,
		VerifyURL: verifyURL,
		OTPCode:   otp,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "EMAIL_ERROR", "failed to send verification email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sent":       true,
		"expires_in": 900,
	})
}

func (h *AuthHandler) generateJWT(userID, tokenType string, duration time.Duration) (string, error) {
	return generateJWTToken(h.cfg.JWTSecret, userID, tokenType, duration)
}

func generateJWTToken(secret, userID, tokenType string, duration time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"type": tokenType,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(duration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func generateCode(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateOTP() string {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		code[i] = digits[n.Int64()]
	}
	return string(code)
}

func generateUserCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 8)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		code[i] = chars[n.Int64()]
	}
	return string(code[:4]) + "-" + string(code[4:])
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func validEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	domain := email[at+1:]
	if len(domain) < 3 || !strings.Contains(domain, ".") {
		return false
	}
	return true
}

var (
	devicePageTmpl = template.Must(template.New("device").Parse(devicePageHTML))
	deviceSentTmpl = template.Must(template.New("sent").Parse(deviceSentHTML))
	verifiedTmpl   = template.Must(template.New("verified").Parse(verifiedHTML))
)

const devicePageHTML = `<!DOCTYPE html>
<html><head><title>mycli - Device Login</title>
<style>body{font-family:system-ui;max-width:400px;margin:80px auto;padding:0 20px}
input{display:block;width:100%;padding:8px;margin:8px 0;box-sizing:border-box;border:1px solid #ccc;border-radius:4px}
button{background:#000;color:#fff;border:none;padding:10px 20px;border-radius:4px;cursor:pointer;width:100%}
h1{font-size:1.5em}</style></head>
<body><h1>mycli — Device Login</h1>
<p>Enter your code and email. We'll send a verification link to complete login.</p>
<form method="POST" action="/device">
<label>User Code<input name="user_code" placeholder="XXXX-XXXX" required></label>
<label>Email<input name="email" type="email" placeholder="you@example.com" required></label>
<button type="submit">Send verification link</button>
</form></body></html>`

const deviceSentHTML = `<!DOCTYPE html>
<html><head><title>mycli - Check Your Email</title>
<style>body{font-family:system-ui;max-width:400px;margin:80px auto;padding:0 20px}
h1{font-size:1.5em}</style></head>
<body><h1>Check your email</h1>
<p>We sent a verification link. Click it to complete login.</p>
<p>Once verified, your terminal session will continue automatically.</p></body></html>`

const verifiedHTML = `<!DOCTYPE html>
<html><head><title>mycli - Verified</title>
<style>body{font-family:system-ui;max-width:400px;margin:80px auto;padding:0 20px}
h1{font-size:1.5em}</style></head>
<body><h1>Email verified!</h1>
<p>You are now logged in. You can close this tab and return to your terminal.</p></body></html>`
