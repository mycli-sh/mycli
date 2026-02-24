package handler

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"mycli.sh/api/internal/authservice"
	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/email"
	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/store"
)

type AuthHandler struct {
	cfg   *config.Config
	store store.AuthStore
	email email.Sender
	auth  *authservice.Service
}

func NewAuthHandler(cfg *config.Config, s store.AuthStore, emailSender email.Sender, authSvc *authservice.Service) *AuthHandler {
	return &AuthHandler{
		cfg:   cfg,
		store: s,
		email: emailSender,
		auth:  authSvc,
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
	if !authservice.ValidEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "invalid email format")
		return
	}

	deviceCode := authservice.GenerateCode(32)
	ctx := r.Context()

	// Opportunistic cleanup of expired magic links
	_ = h.store.DeleteExpiredMagicLinks(ctx)

	expiresAt := time.Now().Add(15 * time.Minute)

	magicToken := authservice.GenerateCode(32)
	tokenHash := authservice.HashToken(magicToken)
	otp := authservice.GenerateOTP()
	otpHash := authservice.HashToken(otp)

	if _, err := h.store.CreateMagicLink(ctx, req.Email, tokenHash, deviceCode, &otpHash, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start device flow")
		return
	}

	verifyURL := h.cfg.BaseURL + "/v1/auth/verify?token=" + magicToken
	emailSent := true
	if err := h.email.SendVerification(email.EmailParams{
		To:        req.Email,
		VerifyURL: verifyURL,
		OTPCode:   otp,
	}); err != nil {
		emailSent = false
	}

	slog.Info("auth.device_flow.start",
		"email", redactEmail(req.Email),
		"ip", r.RemoteAddr,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"device_code": deviceCode,
		"expires_in":  authservice.AccessTokenTTL,
		"interval":    5,
		"email_sent":  emailSent,
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
	ml, err := h.store.GetMagicLinkByDeviceCode(ctx, req.DeviceCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "device code expired or invalid")
		return
	}
	if !ml.Authorized || ml.UserID == nil {
		writeError(w, http.StatusBadRequest, "AUTHORIZATION_PENDING", "waiting for user authorization")
		return
	}
	userID := *ml.UserID

	// Generate tokens (pure functions, no DB)
	accessToken, refreshToken, err := h.auth.GenerateTokenPair(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	meta := authservice.ExtractRequestMeta(r)
	refreshTokenHash := authservice.HashToken(refreshToken)

	// Atomically consume magic links + revoke old device session + create new session
	sess, err := h.store.ConsumeAuthorizedDeviceCode(ctx, req.DeviceCode, userID, refreshTokenHash, meta.UserAgent, meta.IPAddress, meta.DeviceID, meta.DeviceName, time.Now().Add(authservice.RefreshTokenDuration))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to consume device session")
		return
	}

	// Look up user for needs_username (non-critical)
	needsUsername := false
	if user, err := h.store.GetUserByID(ctx, userID); err == nil {
		needsUsername = user.Username == nil
	}

	slog.Info("auth.login.success",
		"user_id", userID,
		"device_id", meta.DeviceID,
		"ip", meta.IPAddress,
	)

	resp := map[string]any{
		"access_token":   accessToken,
		"refresh_token":  refreshToken,
		"expires_in":     authservice.AccessTokenTTL,
		"needs_username": needsUsername,
	}
	if sess != nil {
		resp["session_id"] = sess.ID
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
	refreshTokenHash := authservice.HashToken(req.RefreshToken)
	sess, err := h.store.GetSessionByTokenHash(r.Context(), refreshTokenHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "session not found")
		return
	}
	if sess.RevokedAt != nil {
		writeError(w, http.StatusUnauthorized, "SESSION_REVOKED", "session has been revoked")
		return
	}
	if time.Now().After(sess.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session has expired")
		return
	}
	_ = h.store.UpdateSessionLastUsed(r.Context(), sess.ID)

	accessToken, newRefreshToken, err := h.auth.GenerateTokenPair(sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}
	newRefreshTokenHash := authservice.HashToken(newRefreshToken)
	_ = h.store.UpdateSessionRefreshTokenHash(r.Context(), sess.ID, newRefreshTokenHash)

	slog.Info("auth.refresh",
		"user_id", sub,
		"session_id", sess.ID,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":       accessToken,
		"expires_in":         authservice.AccessTokenTTL,
		"refresh_token":      newRefreshToken,
		"refresh_expires_in": authservice.RefreshTokenTTL,
	})
}

func (h *AuthHandler) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		renderVerifyError(w, http.StatusBadRequest, "Missing token")
		return
	}

	tokenHash := authservice.HashToken(rawToken)
	ctx := r.Context()

	ml, err := h.store.GetMagicLinkByTokenHash(ctx, tokenHash)
	if err != nil {
		renderVerifyError(w, http.StatusBadRequest, "This link is invalid or has expired.")
		return
	}

	if ml.UsedAt != nil || time.Now().After(ml.ExpiresAt) {
		renderVerifyError(w, http.StatusBadRequest, "This link has already been used or has expired.")
		return
	}

	if err := h.store.MarkMagicLinkUsed(ctx, ml.ID); err != nil {
		renderVerifyError(w, http.StatusBadRequest, "This link has already been used or has expired.")
		return
	}

	// Find or create user by email
	user, err := h.auth.FindOrCreateUser(ctx, ml.Email)
	if err != nil {
		renderVerifyError(w, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}

	// Authorize the magic link (marks all magic links for this device code)
	if err := h.store.AuthorizeMagicLinkByDeviceCode(ctx, ml.DeviceCode, user.ID); err != nil {
		renderVerifyError(w, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}

	_ = verifiedTmpl.Execute(w, nil)
}

func renderVerifyError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = errorTmpl.Execute(w, message)
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

	// Look up latest magic link for this device code
	latestML, err := h.store.GetMagicLinkByDeviceCode(ctx, req.DeviceCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "device code expired or invalid")
		return
	}

	// Brute-force protection: check aggregate attempts across all magic links for this device code
	if h.otpRateLimited(ctx, req.DeviceCode, latestML.OTPAttempts) {
		slog.Warn("auth.otp.max_attempts",
			"device_code", req.DeviceCode,
			"ip", r.RemoteAddr,
		)
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "too many verification attempts")
		return
	}

	// Look up the magic link by OTP hash
	otpHash := authservice.HashToken(req.Code)
	ml, err := h.store.GetMagicLinkByOTPHash(ctx, otpHash)
	if err != nil {
		cnt, _ := h.store.IncrementMagicLinkOTPAttempts(ctx, latestML.ID)
		slog.Warn("auth.otp.failed",
			"device_code", req.DeviceCode,
			"attempt", cnt,
			"ip", r.RemoteAddr,
		)
		writeError(w, http.StatusBadRequest, "INVALID_CODE", "invalid or expired code")
		return
	}

	// Verify the magic link belongs to this device code
	if ml.DeviceCode != req.DeviceCode {
		cnt, _ := h.store.IncrementMagicLinkOTPAttempts(ctx, latestML.ID)
		slog.Warn("auth.otp.failed",
			"device_code", req.DeviceCode,
			"attempt", cnt,
			"ip", r.RemoteAddr,
		)
		writeError(w, http.StatusBadRequest, "INVALID_CODE", "invalid or expired code")
		return
	}

	// Mark the magic link as used
	if err := h.store.MarkMagicLinkUsed(ctx, ml.ID); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CODE", "code already used or expired")
		return
	}

	// Find or create user by email
	user, err := h.auth.FindOrCreateUser(ctx, ml.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create user")
		return
	}

	// Authorize magic link for this device code
	if err := h.store.AuthorizeMagicLinkByDeviceCode(ctx, req.DeviceCode, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to authorize device")
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

	if !authservice.ValidEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "invalid email format")
		return
	}

	ctx := r.Context()

	// Check that a magic link exists for this device code
	_, err := h.store.GetMagicLinkByDeviceCode(ctx, req.DeviceCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "device code expired or invalid")
		return
	}

	// Check aggregate OTP attempts across all magic links for this device code
	if h.otpRateLimited(ctx, req.DeviceCode, 0) {
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "too many verification attempts")
		return
	}

	// Generate new magic link + OTP
	magicToken := authservice.GenerateCode(32)
	tokenHash := authservice.HashToken(magicToken)
	otp := authservice.GenerateOTP()
	otpHash := authservice.HashToken(otp)

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
		"expires_in": authservice.AccessTokenTTL,
	})
}

// WebLogin sends a magic link email for web login.
func (h *AuthHandler) WebLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Email == "" || !authservice.ValidEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "valid email is required")
		return
	}

	// Create a magic link with OTP and a special "web" device code prefix
	magicToken := authservice.GenerateCode(32)
	tokenHash := authservice.HashToken(magicToken)
	deviceCode := "web_" + authservice.GenerateCode(16)
	otp := authservice.GenerateOTP()
	otpHash := authservice.HashToken(otp)

	ctx := r.Context()
	_, err := h.store.CreateMagicLink(ctx, req.Email, tokenHash, deviceCode, &otpHash, time.Now().Add(15*time.Minute))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create magic link")
		return
	}

	verifyURL := h.cfg.WebBaseURL + "/auth/verify?token=" + magicToken
	if err := h.email.SendVerification(email.EmailParams{
		To:        req.Email,
		VerifyURL: verifyURL,
		OTPCode:   otp,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "EMAIL_ERROR", "failed to send verification email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sent": true,
	})
}

// WebVerify validates the magic link token and returns JWT tokens + session.
func (h *AuthHandler) WebVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TOKEN", "token is required")
		return
	}

	tokenHash := authservice.HashToken(req.Token)
	ctx := r.Context()

	ml, err := h.store.GetMagicLinkByTokenHash(ctx, tokenHash)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_TOKEN", "invalid or expired link")
		return
	}

	if ml.UsedAt != nil || time.Now().After(ml.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "link already used or expired")
		return
	}

	if err := h.store.MarkMagicLinkUsed(ctx, ml.ID); err != nil {
		writeError(w, http.StatusBadRequest, "EXPIRED_TOKEN", "link already used or expired")
		return
	}

	// Find or create user by email
	user, err := h.auth.FindOrCreateUser(ctx, ml.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create user")
		return
	}

	// Issue tokens and create session
	result, err := h.auth.IssueTokens(ctx, user.ID, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":   result.AccessToken,
		"refresh_token":  result.RefreshToken,
		"session_id":     result.SessionID,
		"expires_in":     authservice.AccessTokenTTL,
		"needs_username": result.NeedsUsername,
	})
}

// Logout revokes the current session. It uses the X-Device-ID header to identify
// the session to revoke, falling back to a refresh_token in the request body.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}

	ctx := r.Context()
	meta := authservice.ExtractRequestMeta(r)

	if meta.DeviceID != "" {
		_ = h.store.RevokeSessionByDeviceID(ctx, userID, meta.DeviceID)
	}

	// Also accept refresh_token in body as fallback
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.RefreshToken != "" {
		tokenHash := authservice.HashToken(req.RefreshToken)
		if sess, err := h.store.GetSessionByTokenHash(ctx, tokenHash); err == nil && sess.UserID == userID {
			_ = h.store.RevokeSession(ctx, sess.ID)
		}
	}

	slog.Info("auth.logout",
		"user_id", userID,
		"device_id", meta.DeviceID,
	)

	writeJSON(w, http.StatusOK, map[string]any{"logged_out": true})
}

const maxOTPAttempts = 5

func (h *AuthHandler) otpRateLimited(ctx context.Context, deviceCode string, fallback int) bool {
	total, err := h.store.CountOTPAttemptsByDeviceCode(ctx, deviceCode)
	if err != nil {
		total = fallback
	}
	return total >= maxOTPAttempts
}

// redactEmail returns "f***@example.com" style redaction for logging.
func redactEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

var verifiedTmpl = template.Must(template.New("verified").Parse(verifiedHTML))

const verifiedHTML = `<!DOCTYPE html>
<html><head><title>mycli - Verified</title>
<style>body{font-family:system-ui;max-width:400px;margin:80px auto;padding:0 20px}
h1{font-size:1.5em}</style></head>
<body><h1>Email verified!</h1>
<p>You are now logged in. You can close this tab and return to your terminal.</p></body></html>`

var errorTmpl = template.Must(template.New("error").Parse(errorHTML))

const errorHTML = `<!DOCTYPE html>
<html><head><title>mycli - Error</title>
<style>body{font-family:system-ui;max-width:400px;margin:80px auto;padding:0 20px}
h1{font-size:1.5em;color:#c00}</style></head>
<body><h1>Verification failed</h1>
<p>{{.}}</p></body></html>`
