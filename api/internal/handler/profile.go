package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type ProfileHandler struct {
	store store.ProfileStore
}

func NewProfileHandler(s store.ProfileStore) *ProfileHandler {
	return &ProfileHandler{store: s}
}

func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.Slug == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "slug is required")
		return
	}
	if !slugRegex.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "slug must match ^[a-z][a-z0-9-]*$ (lowercase letters, digits, hyphens)")
		return
	}
	if req.Name == "" {
		req.Name = req.Slug
	}

	profile, err := h.store.CreateProfile(r.Context(), userID, req.Slug, req.Name, req.Description)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "PROFILE_EXISTS", "a profile with that slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create profile")
		return
	}

	writeJSON(w, http.StatusCreated, profile)
}

func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	profiles, err := h.store.ListProfilesByOwner(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list profiles")
		return
	}

	if profiles == nil {
		profiles = []model.Profile{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	slug := chi.URLParam(r, "slug")

	profile, err := h.store.GetProfileByOwnerAndSlug(r.Context(), userID, slug)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get profile")
		return
	}

	libs, _ := h.store.ListProfileLibraries(r.Context(), profile.ID)
	if libs == nil {
		libs = []model.Library{}
	}

	// Build library items with owner names and commands
	type libraryItem struct {
		model.Library
		Owner    string                 `json:"owner"`
		Commands []store.LibraryCommand `json:"commands,omitempty"`
	}
	items := make([]libraryItem, 0, len(libs))
	for _, lib := range libs {
		item := libraryItem{Library: lib}
		if lib.OwnerID != nil {
			item.Owner, _ = h.store.GetOwnerName(r.Context(), *lib.OwnerID)
		}
		cmds, _ := h.store.ListCommandsByLibrary(r.Context(), lib.ID)
		item.Commands = cmds
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"profile":   profile,
		"libraries": items,
	})
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	slug := chi.URLParam(r, "slug")

	profile, err := h.store.GetProfileByOwnerAndSlug(r.Context(), userID, slug)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get profile")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	name := profile.Name
	desc := profile.Description
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		desc = *req.Description
	}

	updated, err := h.store.UpdateProfile(r.Context(), profile.ID, name, desc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (h *ProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	slug := chi.URLParam(r, "slug")

	profile, err := h.store.GetProfileByOwnerAndSlug(r.Context(), userID, slug)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get profile")
		return
	}

	if profile.IsDefault {
		writeError(w, http.StatusBadRequest, "CANNOT_DELETE_DEFAULT", "cannot delete the default profile")
		return
	}

	// Check for scoped API tokens; require ?force=true to proceed
	tokenCount, err := h.store.CountTokensByProfile(r.Context(), profile.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check scoped tokens")
		return
	}
	if tokenCount > 0 && r.URL.Query().Get("force") != "true" {
		writeError(w, http.StatusConflict, "HAS_SCOPED_TOKENS",
			fmt.Sprintf("profile has %d scoped API token(s) that will also be deleted; use force to confirm", tokenCount))
		return
	}

	if err := h.store.DeleteProfile(r.Context(), profile.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ProfileHandler) AddLibrary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	slug := chi.URLParam(r, "slug")

	profile, err := h.store.GetProfileByOwnerAndSlug(r.Context(), userID, slug)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get profile")
		return
	}

	var req struct {
		Library string `json:"library"` // "owner/slug" or "slug"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Library == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "library identifier is required")
		return
	}

	lib, err := resolveLibraryIdentifier(r.Context(), h.store, req.Library)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
		return
	}

	if err := h.store.AddLibraryToProfile(r.Context(), profile.ID, lib.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to add library to profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (h *ProfileHandler) RemoveLibrary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	profileSlug := chi.URLParam(r, "slug")
	libOwner := chi.URLParam(r, "owner")
	libSlug := chi.URLParam(r, "libSlug")

	profile, err := h.store.GetProfileByOwnerAndSlug(r.Context(), userID, profileSlug)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get profile")
		return
	}

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), libOwner, libSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
		return
	}

	if err := h.store.RemoveLibraryFromProfile(r.Context(), profile.ID, lib.ID); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "library not in this profile")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove library from profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *ProfileHandler) ListLibraries(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	slug := chi.URLParam(r, "slug")

	profile, err := h.store.GetProfileByOwnerAndSlug(r.Context(), userID, slug)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get profile")
		return
	}

	libs, err := h.store.ListProfileLibraries(r.Context(), profile.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list libraries")
		return
	}
	if libs == nil {
		libs = []model.Library{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"libraries": libs})
}

// resolveLibraryIdentifier resolves "owner/slug" or "slug" to a library.
func resolveLibraryIdentifier(ctx context.Context, s interface {
	GetLibraryByOwnerUsernameAndSlug(ctx context.Context, ownerName, slug string) (*model.Library, error)
	GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error)
}, identifier string) (*model.Library, error) {
	if strings.Contains(identifier, "/") {
		parts := strings.SplitN(identifier, "/", 2)
		return s.GetLibraryByOwnerUsernameAndSlug(ctx, parts[0], parts[1])
	}
	return s.GetLibraryBySlug(ctx, identifier)
}
