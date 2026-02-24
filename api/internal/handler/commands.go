package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/store"
	"mycli.sh/pkg/spec"
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type CommandHandler struct {
	store store.CommandStore
}

func NewCommandHandler(s store.CommandStore) *CommandHandler {
	return &CommandHandler{store: s}
}

func (h *CommandHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req struct {
		Name        string   `json:"name"`
		Slug        string   `json:"slug"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "name and slug are required")
		return
	}

	if !slugPattern.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "INVALID_SLUG", "slug must match ^[a-z][a-z0-9-]*$")
		return
	}

	tags, _ := json.Marshal(req.Tags)
	if req.Tags == nil {
		tags = []byte("[]")
	}

	cmd, err := h.store.CreateCommand(r.Context(), userID, req.Name, req.Slug, req.Description, tags)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "SLUG_EXISTS", "a command with this slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create command")
		return
	}

	writeJSON(w, http.StatusCreated, cmd)
}

func (h *CommandHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Direct slug lookup — returns single-element result if found
	if slug := r.URL.Query().Get("slug"); slug != "" {
		cmd, err := h.store.GetCommandByOwnerAndSlug(r.Context(), userID, slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusOK, map[string]any{
					"commands":    []any{},
					"next_cursor": "",
				})
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get command")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"commands":    []any{cmd},
			"next_cursor": "",
		})
		return
	}

	cursor := r.URL.Query().Get("cursor")
	query := r.URL.Query().Get("q")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	commands, nextCursor, err := h.store.ListCommandsByOwner(r.Context(), userID, cursor, limit, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list commands")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"commands":    commands,
		"next_cursor": nextCursor,
	})
}

func (h *CommandHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	cmd, err := h.store.GetCommandByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get command")
		return
	}

	if cmd.OwnerUserID != userID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
		return
	}

	// Get latest version info
	var latestVersion int
	if latest, err := h.store.GetLatestVersionByCommand(r.Context(), id); err == nil {
		latestVersion = latest.Version
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":             cmd.ID,
		"name":           cmd.Name,
		"slug":           cmd.Slug,
		"description":    cmd.Description,
		"tags":           cmd.Tags,
		"latest_version": latestVersion,
		"created_at":     cmd.CreatedAt,
		"updated_at":     cmd.UpdatedAt,
	})
}

func (h *CommandHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	cmd, err := h.store.GetCommandByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get command")
		return
	}

	if cmd.OwnerUserID != userID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
		return
	}

	if err := h.store.SoftDeleteCommand(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete command")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CommandHandler) PublishVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	cmd, err := h.store.GetCommandByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get command")
		return
	}

	if cmd.OwnerUserID != userID {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
		return
	}

	var req struct {
		SpecJSON json.RawMessage `json:"spec_json"`
		Message  string          `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	// Validate spec
	if err := spec.Validate(req.SpecJSON); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SPEC", err.Error())
		return
	}

	// Compute hash
	parsedSpec, err := spec.Parse(req.SpecJSON)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SPEC", err.Error())
		return
	}

	hash, err := spec.Hash(parsedSpec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute spec hash")
		return
	}

	// Check for identical spec
	existingHash, err := h.store.GetLatestHashByCommand(r.Context(), id)
	if err == nil && existingHash == hash {
		writeError(w, http.StatusConflict, "SPEC_IDENTICAL", "spec is identical to the latest version")
		return
	}

	// Determine next version number
	nextVersion := 1
	if latest, err := h.store.GetLatestVersionByCommand(r.Context(), id); err == nil {
		nextVersion = latest.Version + 1
	}

	version, err := h.store.CreateVersion(r.Context(), id, nextVersion, req.SpecJSON, hash, req.Message, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create version")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        version.ID,
		"version":   version.Version,
		"spec_hash": version.SpecHash,
	})
}

func (h *CommandHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	versionStr := chi.URLParam(r, "version")

	cmd, err := h.store.GetCommandByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get command")
		return
	}

	if cmd.OwnerUserID != userID {
		if cmd.LibraryID == nil || !h.store.IsLibraryInstalled(r.Context(), userID, *cmd.LibraryID) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "command not found")
			return
		}
	}

	versionNum, err := strconv.Atoi(versionStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid version number")
		return
	}

	version, err := h.store.GetVersionByCommandAndVersion(r.Context(), id, versionNum)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "version not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get version")
		return
	}

	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, version.SpecHash))
	writeJSON(w, http.StatusOK, version)
}
