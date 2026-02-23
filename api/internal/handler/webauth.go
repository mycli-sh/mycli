package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/email"
	"mycli.sh/api/internal/store"
)

type WebAuthHandler struct {
	cfg   *config.Config
	store WebAuthStore
	email email.Sender
}

func NewWebAuthHandler(cfg *config.Config, s WebAuthStore, emailSender email.Sender) *WebAuthHandler {
	return &WebAuthHandler{
		cfg:   cfg,
		store: s,
		email: emailSender,
	}
}

// Login sends a magic link email for web login.
func (h *WebAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Email == "" || !validEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "valid email is required")
		return
	}

	// Create a magic link with OTP and a special "web" device code prefix
	magicToken := generateCode(32)
	tokenHash := hashToken(magicToken)
	deviceCode := "web_" + generateCode(16)
	otp := generateOTP()
	otpHash := hashToken(otp)

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

// Verify validates the magic link token and returns JWT tokens + session.
func (h *WebAuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
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

	tokenHash := hashToken(req.Token)
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

	// Generate tokens
	accessToken, err := h.generateJWT(user.ID, "access", time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	refreshToken, err := h.generateJWT(user.ID, "refresh", 30*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to generate token")
		return
	}

	// Create session
	refreshTokenHash := hashToken(refreshToken)
	userAgent := r.UserAgent()
	ipAddress := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ipAddress = strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}

	session, err := h.store.CreateSession(ctx, user.ID, refreshTokenHash, userAgent, ipAddress, time.Now().Add(30*24*time.Hour))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":   accessToken,
		"refresh_token":  refreshToken,
		"session_id":     session.ID,
		"expires_in":     3600,
		"needs_username": user.Username == nil,
	})
}

func (h *WebAuthHandler) generateJWT(userID, tokenType string, duration time.Duration) (string, error) {
	// Reuse the same JWT generation as AuthHandler
	return generateJWTToken(h.cfg.JWTSecret, userID, tokenType, duration)
}
