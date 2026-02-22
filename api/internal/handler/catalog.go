package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"

	"mycli.sh/api/internal/middleware"
)

type CatalogHandler struct {
	store CatalogStore
}

func NewCatalogHandler(s CatalogStore) *CatalogHandler {
	return &CatalogHandler{store: s}
}

type catalogItem struct {
	CommandID    string `json:"command_id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Version      int    `json:"version"`
	SpecHash     string `json:"spec_hash"`
	UpdatedAt    string `json:"updated_at"`
	Library      string `json:"library,omitempty"`
	LibraryOwner string `json:"library_owner,omitempty"`
}

func (h *CatalogHandler) GetCatalog(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	// Fetch all commands for the user (paginate through all pages)
	var items []catalogItem
	cursor := ""
	for {
		commands, nextCursor, err := h.store.ListCommandsByOwner(r.Context(), userID, cursor, 200, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list commands")
			return
		}

		for _, cmd := range commands {
			latest, err := h.store.GetLatestVersionByCommand(r.Context(), cmd.ID)
			if err != nil {
				continue // skip commands with no versions
			}

			items = append(items, catalogItem{
				CommandID:   cmd.ID,
				Slug:        cmd.Slug,
				Name:        cmd.Name,
				Description: cmd.Description,
				Version:     latest.Version,
				SpecHash:    latest.SpecHash,
				UpdatedAt:   cmd.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			})
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	// Fetch installed library commands
	installedLibs, err := h.store.GetInstalledLibraries(r.Context(), userID)
	if err == nil {
		for _, lib := range installedLibs {
			ownerName := ""
			if lib.OwnerID != nil {
				ownerName, _ = h.store.GetOwnerName(r.Context(), *lib.OwnerID)
			}

			libCmds, err := h.store.ListCommandsByLibrary(r.Context(), lib.ID)
			if err != nil {
				continue
			}
			for _, lc := range libCmds {
				latest, err := h.store.GetLatestVersionByCommand(r.Context(), lc.CommandID)
				if err != nil {
					continue
				}
				items = append(items, catalogItem{
					CommandID:    lc.CommandID,
					Slug:         lc.Slug,
					Name:         lc.Name,
					Description:  lc.Description,
					Version:      latest.Version,
					SpecHash:     latest.SpecHash,
					UpdatedAt:    lc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
					Library:      lib.Slug,
					LibraryOwner: ownerName,
				})
			}
		}
	}

	if items == nil {
		items = []catalogItem{}
	}

	// Compute ETag from catalog content
	data, _ := json.Marshal(items)
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(data))

	// Check If-None-Match
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", etag)
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}
