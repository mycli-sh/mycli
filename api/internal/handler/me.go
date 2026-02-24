package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/store"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	reservedNames = map[string]bool{
		"admin": true, "api": true, "system": true, "www": true,
		"help": true, "support": true, "mycli": true, "root": true,
	}
)

type MeHandler struct {
	store store.MeStore
}

func NewMeHandler(s store.MeStore) *MeHandler {
	return &MeHandler{store: s}
}

func (h *MeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get user")
		return
	}

	resp := map[string]any{
		"id":             user.ID,
		"email":          user.Email,
		"needs_username": user.Username == nil,
	}
	if user.Username != nil {
		resp["username"] = *user.Username
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *MeHandler) SetUsername(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)

	if reason := validateUsername(username); reason != "" {
		writeError(w, http.StatusBadRequest, "INVALID_USERNAME", reason)
		return
	}

	// Check availability
	taken, err := h.store.IsUsernameTaken(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check username")
		return
	}
	if taken {
		writeError(w, http.StatusConflict, "USERNAME_TAKEN", "username is already taken")
		return
	}

	// Set username (DB enforces WHERE username IS NULL)
	if err := h.store.SetUsername(r.Context(), userID, username); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusConflict, "USERNAME_ALREADY_SET", "username has already been set")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to set username")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"username": username,
	})
}

func (h *MeHandler) CheckUsernameAvailable(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	if reason := validateUsername(username); reason != "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false,
			"reason":    reason,
		})
		return
	}

	taken, err := h.store.IsUsernameTaken(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check username")
		return
	}

	if taken {
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false,
			"reason":    "username is already taken",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"available": true,
	})
}

func validateUsername(username string) string {
	if len(username) < 3 {
		return "username must be at least 3 characters"
	}
	if len(username) > 39 {
		return "username must be at most 39 characters"
	}
	if !usernameRegex.MatchString(username) {
		return "username must start with a letter and contain only lowercase letters, numbers, and hyphens"
	}
	if reservedNames[username] {
		return "username is reserved"
	}
	return ""
}

func (h *MeHandler) SyncSummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	userCommandCount, err := h.store.CountCommandsByOwner(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to count commands")
		return
	}

	installedLibs, err := h.store.GetInstalledLibraries(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get installed libraries")
		return
	}

	type libSummary struct {
		Slug         string `json:"slug"`
		Name         string `json:"name"`
		CommandCount int    `json:"command_count"`
	}

	libSummaries := make([]libSummary, 0, len(installedLibs))
	totalCommands := userCommandCount

	for _, lib := range installedLibs {
		cmds, err := h.store.ListCommandsByLibrary(ctx, lib.ID)
		if err != nil {
			continue
		}
		cmdCount := len(cmds)
		totalCommands += cmdCount

		ownerName := ""
		if lib.OwnerID != nil {
			ownerName, _ = h.store.GetOwnerName(ctx, *lib.OwnerID)
		}
		slug := lib.Slug
		if ownerName != "" && !strings.EqualFold(ownerName, "system") {
			slug = ownerName + "/" + lib.Slug
		}

		libSummaries = append(libSummaries, libSummary{
			Slug:         slug,
			Name:         lib.Name,
			CommandCount: cmdCount,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_commands_count": userCommandCount,
		"installed_libraries": libSummaries,
		"total_commands":      totalCommands,
	})
}
