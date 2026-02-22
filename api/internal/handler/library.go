package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
	"mycli.sh/pkg/spec"
)

const systemUserID = "usr_system"

var tagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

type LibraryHandler struct {
	cfg   *config.Config
	store LibraryStore
}

func NewLibraryHandler(cfg *config.Config, s LibraryStore) *LibraryHandler {
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
	if userID := middleware.GetUserID(r.Context()); userID != "" {
		installed = h.store.IsLibraryInstalled(r.Context(), userID, lib.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"library":   lib,
		"owner":     ownerName,
		"commands":  cmds,
		"installed": installed,
	})
}

// publishCommands publishes command specs to a library, creating or updating commands as needed.
func (h *LibraryHandler) publishCommands(ctx context.Context, userID, libraryID string, commands []json.RawMessage) (int, error) {
	published := 0
	for _, cmdJSON := range commands {
		parsed, err := spec.Parse(cmdJSON)
		if err != nil {
			continue
		}

		cmdSlug := parsed.Metadata.Slug
		tags, _ := json.Marshal(parsed.Metadata.Tags)
		if parsed.Metadata.Tags == nil {
			tags = []byte("[]")
		}

		// Get or create the command under this library
		cmd, err := h.store.GetCommandByLibraryAndSlug(ctx, libraryID, cmdSlug)
		if err != nil {
			cmd, err = h.store.CreateCommandForLibrary(ctx, userID, libraryID, parsed.Metadata.Name, cmdSlug, parsed.Metadata.Description, tags)
			if err != nil {
				continue
			}
		} else {
			// Sync metadata from latest spec
			_ = h.store.UpdateCommandMeta(ctx, cmd.ID, parsed.Metadata.Name, parsed.Metadata.Description, tags)
		}

		hash, err := spec.Hash(parsed)
		if err != nil {
			continue
		}

		// Skip if identical to latest
		if existingHash, err := h.store.GetLatestHashByCommand(ctx, cmd.ID); err == nil && existingHash == hash {
			published++
			continue
		}

		nextVersion := 1
		if latest, err := h.store.GetLatestVersionByCommand(ctx, cmd.ID); err == nil {
			nextVersion = latest.Version + 1
		}

		if _, err := h.store.CreateVersion(ctx, cmd.ID, nextVersion, cmdJSON, hash, "Published via library release", userID); err != nil {
			continue
		}
		published++
	}
	return published, nil
}

// CreateRelease handles POST /v1/libraries/{slug}/releases
func (h *LibraryHandler) CreateRelease(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	userID := middleware.GetUserID(r.Context())

	var req struct {
		Tag         string            `json:"tag"`
		CommitHash  string            `json:"commit_hash"`
		Namespace   string            `json:"namespace"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		GitURL      string            `json:"git_url"`
		Commands    []json.RawMessage `json:"commands"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Tag == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "tag and name are required")
		return
	}

	if !tagPattern.MatchString(req.Tag) {
		writeError(w, http.StatusBadRequest, "INVALID_TAG", "tag must match vX.Y.Z format")
		return
	}

	if !slugPattern.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "INVALID_SLUG", "slug must match ^[a-z][a-z0-9-]*$")
		return
	}

	// Validate all command specs
	for i, cmdJSON := range req.Commands {
		if err := spec.Validate(cmdJSON); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SPEC", "command "+strconv.Itoa(i)+": "+err.Error())
			return
		}
	}

	// Determine the owner ID for the library
	ownerID := userID
	if req.Namespace == "system" {
		user, err := h.store.GetUserByID(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to look up user")
			return
		}
		if !h.cfg.IsSystemAdmin(user.Email) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "not authorized to release to system namespace")
			return
		}
		ownerID = systemUserID
	}

	version := strings.TrimPrefix(req.Tag, "v")

	var gitURL *string
	if req.GitURL != "" {
		gitURL = &req.GitURL
	}

	// Upsert library
	lib, err := h.store.CreateOrUpdateLibrary(r.Context(), ownerID, slug, req.Name, req.Description, gitURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create library")
		return
	}

	// Check if release already exists
	exists, err := h.store.LibraryReleaseExists(r.Context(), lib.ID, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check release")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "RELEASE_EXISTS", "version "+version+" already exists")
		return
	}

	// Publish commands (use ownerID for library ownership, userID for audit trail)
	published, err := h.publishCommands(r.Context(), ownerID, lib.ID, req.Commands)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to publish commands")
		return
	}

	// Create release record (releasedBy = real user for audit trail)
	release, err := h.store.CreateLibraryRelease(r.Context(), lib.ID, version, req.Tag, req.CommitHash, published, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create release")
		return
	}

	// Update latest version
	if err := h.store.UpdateLibraryLatestVersion(r.Context(), lib.ID, version); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update latest version")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"release":   release,
		"published": published,
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

// Install handles POST /v1/libraries/{owner}/{slug}/install
func (h *LibraryHandler) Install(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	userID := middleware.GetUserID(r.Context())

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), owner, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Fallback: try slug-only lookup for system libraries (e.g., "kubernetes")
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

	if err := h.store.InstallLibrary(r.Context(), userID, lib.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to install")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "installed"})
}

// Uninstall handles DELETE /v1/libraries/{owner}/{slug}/install
func (h *LibraryHandler) Uninstall(w http.ResponseWriter, r *http.Request) {
	owner := chi.URLParam(r, "owner")
	slug := chi.URLParam(r, "slug")
	userID := middleware.GetUserID(r.Context())

	lib, err := h.store.GetLibraryByOwnerUsernameAndSlug(r.Context(), owner, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
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

	if err := h.store.UninstallLibrary(r.Context(), userID, lib.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to uninstall")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
