package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/store"
)

type SessionHandler struct {
	store store.SessionStore
}

func NewSessionHandler(s store.SessionStore) *SessionHandler {
	return &SessionHandler{store: s}
}

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	sessions, err := h.store.ListSessionsByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list sessions")
		return
	}

	// Build response without exposing refresh_token_hash
	type sessionResponse struct {
		ID         string `json:"id"`
		UserAgent  string `json:"user_agent"`
		IPAddress  string `json:"ip_address"`
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
		LastUsedAt string `json:"last_used_at"`
		ExpiresAt  string `json:"expires_at"`
		CreatedAt  string `json:"created_at"`
	}

	result := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, sessionResponse{
			ID:         s.ID,
			UserAgent:  s.UserAgent,
			IPAddress:  s.IPAddress,
			DeviceID:   s.DeviceID,
			DeviceName: s.DeviceName,
			LastUsedAt: s.LastUsedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt:  s.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedAt:  s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"sessions": result})
}

func (h *SessionHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "id")

	// Verify session belongs to user by listing their sessions
	sessions, err := h.store.ListSessionsByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify session ownership")
		return
	}

	owned := false
	for _, s := range sessions {
		if s.ID == sessionID {
			owned = true
			break
		}
	}
	if !owned {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}

	if err := h.store.RevokeSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke session")
		return
	}

	slog.Info("auth.session.revoked",
		"user_id", userID,
		"session_id", sessionID,
		"by", "self",
	)

	writeJSON(w, http.StatusOK, map[string]any{"revoked": true})
}

func (h *SessionHandler) RevokeAll(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Get current session ID from query param
	currentSessionID := r.URL.Query().Get("current_session_id")
	if currentSessionID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PARAM", "current_session_id query parameter is required")
		return
	}

	count, err := h.store.RevokeAllSessionsExcept(r.Context(), userID, currentSessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke sessions")
		return
	}

	slog.Info("auth.session.revoked",
		"user_id", userID,
		"by", "self",
		"count", count,
	)

	writeJSON(w, http.StatusOK, map[string]any{"revoked_count": count})
}
