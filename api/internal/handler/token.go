package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/store"
)

const maxTokenNameLength = 100

type TokenHandler struct {
	store store.TokenStore
}

func NewTokenHandler(s store.TokenStore) *TokenHandler {
	return &TokenHandler{store: s}
}

func (h *TokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req struct {
		Name      string  `json:"name"`
		ExpiresIn *string `json:"expires_in,omitempty"` // e.g. "90d", "30d"
		ProfileID *string `json:"profile_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}
	if len(req.Name) > maxTokenNameLength {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", fmt.Sprintf("name must be %d characters or fewer", maxTokenNameLength))
		return
	}

	// Generate raw token: myc_ + 40 hex chars
	rawBytes := make([]byte, 20)
	if _, err := rand.Read(rawBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate token")
		return
	}
	rawToken := "myc_" + hex.EncodeToString(rawBytes)
	tokenPrefix := rawToken[:12] + "..."

	// Hash for storage
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := fmt.Sprintf("%x", hash)

	// Parse expiry
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn != "" {
		dur, err := parseDuration(*req.ExpiresIn)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid expires_in format (e.g. 30d, 90d)")
			return
		}
		t := time.Now().Add(dur)
		expiresAt = &t
	}

	// Parse and authorize profile ID — must belong to the requesting user
	var profileID *uuid.UUID
	if req.ProfileID != nil && *req.ProfileID != "" {
		id, err := uuid.Parse(*req.ProfileID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid profile_id")
			return
		}
		if _, err := h.store.GetProfileByOwner(r.Context(), userID, id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify profile")
			return
		}
		profileID = &id
	}

	token, err := h.store.CreateAPIToken(r.Context(), userID, req.Name, tokenHash, tokenPrefix, profileID, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":        rawToken, // shown only once
		"id":           token.ID,
		"name":         token.Name,
		"token_prefix": token.TokenPrefix,
		"profile_id":   token.ProfileID,
		"expires_at":   token.ExpiresAt,
		"created_at":   token.CreatedAt,
	})
}

func (h *TokenHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	tokens, err := h.store.ListAPITokens(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list tokens")
		return
	}

	type tokenResp struct {
		ID          uuid.UUID  `json:"id"`
		Name        string     `json:"name"`
		TokenPrefix string     `json:"token_prefix"`
		ProfileID   *uuid.UUID `json:"profile_id,omitempty"`
		LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
		ExpiresAt   *time.Time `json:"expires_at,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
	}

	items := make([]tokenResp, len(tokens))
	for i, t := range tokens {
		items[i] = tokenResp{
			ID:          t.ID,
			Name:        t.Name,
			TokenPrefix: t.TokenPrefix,
			ProfileID:   t.ProfileID,
			LastUsedAt:  t.LastUsedAt,
			ExpiresAt:   t.ExpiresAt,
			CreatedAt:   t.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"tokens": items})
}

func (h *TokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid token ID")
		return
	}

	if err := h.store.RevokeAPIToken(r.Context(), id, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// parseDuration parses a human-friendly duration string like "30d", "90d", "1y".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	switch unit {
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'y':
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported unit: %c", unit)
	}
}
