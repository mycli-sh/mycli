package handler

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

var systemUserID = uuid.MustParse("00000000-0000-7000-8000-000000000001")

var tagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

type LibraryHandler struct {
	cfg   *config.Config
	store store.LibraryStore
}

func NewLibraryHandler(cfg *config.Config, s store.LibraryStore) *LibraryHandler {
	return &LibraryHandler{cfg: cfg, store: s}
}

// Search handles GET /v1/libraries?q=&limit=&offset= (public, with optional auth)
func (h *LibraryHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	libs, total, err := h.store.SearchPublicLibraries(r.Context(), query, limit, offset)
	if err != nil {
		log.Printf("search public libraries: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to search libraries")
		return
	}
	if libs == nil {
		libs = []model.Library{}
	}

	// Attach owner names
	type libraryWithOwner struct {
		model.Library
		Owner string `json:"owner"`
	}
	results := make([]libraryWithOwner, 0, len(libs))
	for _, lib := range libs {
		owner := ""
		if lib.OwnerID != nil {
			owner, _ = h.store.GetOwnerName(r.Context(), *lib.OwnerID)
		}
		results = append(results, libraryWithOwner{Library: lib, Owner: owner})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"libraries": results,
		"total":     total,
	})
}

// GetDetail handles GET /v1/libraries/{owner}/{slug} (public, with optional auth)
func (h *LibraryHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), owner, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Fallback: try slug-only lookup for system libraries
			lib, err = h.store.GetLibraryBySlug(r.Context(), slug)
			if err != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
				return
			}
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get library")
			return
		}
	}

	cmds, err := h.store.ListCommandsByLibrary(r.Context(), lib.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list library commands")
		return
	}

	ownerName := owner
	if lib.OwnerID != nil {
		if n, err := h.store.GetOwnerName(r.Context(), *lib.OwnerID); err == nil {
			ownerName = n
		}
	}

	installed := false
	if userID := middleware.GetUserID(r.Context()); userID != uuid.Nil {
		installed = h.store.IsLibraryInstalled(r.Context(), userID, lib.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"library":   lib,
		"owner":     ownerName,
		"commands":  cmds,
		"installed": installed,
	})
}

// ListReleases handles GET /v1/libraries/{owner}/{slug}/releases
func (h *LibraryHandler) ListReleases(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), owner, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get library")
		return
	}

	releases, err := h.store.ListLibraryReleases(r.Context(), lib.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list releases")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"releases": releases,
	})
}

// GetRelease handles GET /v1/libraries/{owner}/{slug}/releases/{version}
func (h *LibraryHandler) GetRelease(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	version := chi.URLParam(r, "version")

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), owner, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get library")
		return
	}

	release, err := h.store.GetLibraryRelease(r.Context(), lib.ID, version)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "release not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get release")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"release": release,
	})
}

// GetCommand handles GET /v1/libraries/{owner}/{slug}/commands/{commandSlug}
func (h *LibraryHandler) GetCommand(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	commandSlug := chi.URLParam(r, "commandSlug")

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), owner, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get library")
		return
	}

	cmd, err := h.store.GetCommandByLibraryAndSlug(r.Context(), lib.ID, commandSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get command")
		return
	}

	latestVersion, _ := h.store.GetLatestVersionByCommand(r.Context(), cmd.ID)

	resp := map[string]any{
		"command": cmd,
	}
	if latestVersion != nil {
		resp["latest_version"] = latestVersion
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListCommandVersions handles GET /v1/libraries/{owner}/{slug}/commands/{commandSlug}/versions
func (h *LibraryHandler) ListCommandVersions(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	commandSlug := chi.URLParam(r, "commandSlug")

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), owner, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "library not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get library")
		return
	}

	cmd, err := h.store.GetCommandByLibraryAndSlug(r.Context(), lib.ID, commandSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get command")
		return
	}

	versions, err := h.store.ListVersionsByCommand(r.Context(), cmd.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list versions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"versions": versions,
	})
}
