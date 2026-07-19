package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/middleware"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
	"mycli.sh/pkg/spec"
)

// ReleaseHandler owns the atomic bundled release endpoint (POST /v1/releases).
// One request describes every library in a manifest at one tag; the handler
// publishes them all inside a single transaction so a partial publish is
// impossible.
type ReleaseHandler struct {
	cfg   *config.Config
	store store.LibraryStore
}

func NewReleaseHandler(cfg *config.Config, s store.LibraryStore) *ReleaseHandler {
	return &ReleaseHandler{cfg: cfg, store: s}
}

type bundledLibraryReq struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Aliases       []string          `json:"aliases"`
	ContentSHA256 string            `json:"content_sha256"`
	Commands      []json.RawMessage `json:"commands"`
}

type bundledReleaseReq struct {
	Tag        string              `json:"tag"`
	CommitHash string              `json:"commit_hash"`
	GitURL     string              `json:"git_url"`
	Namespace  string              `json:"namespace"`
	Libraries  []bundledLibraryReq `json:"libraries"`
}

type bundledLibraryResp struct {
	Slug           string `json:"slug"`
	ReleaseID      string `json:"release_id"`
	PublishedCount int    `json:"published_count"`
	// Status is "created" or "idempotent".
	Status string `json:"status"`
}

// CreateBundled handles POST /v1/releases — the only publish path.
func (h *ReleaseHandler) CreateBundled(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req bundledReleaseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Tag == "" || len(req.Libraries) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "tag and libraries are required")
		return
	}
	if !tagPattern.MatchString(req.Tag) {
		writeError(w, http.StatusBadRequest, "INVALID_TAG", "tag must match vX.Y.Z format")
		return
	}
	version := strings.TrimPrefix(req.Tag, "v")

	// Per-library upfront validation. Anything the client can obviously get wrong
	// (bad slug, empty library, invalid spec, hash mismatch) is rejected before
	// we start a transaction.
	for i, lib := range req.Libraries {
		if !slugPattern.MatchString(lib.Slug) {
			writeError(w, http.StatusBadRequest, "INVALID_SLUG", fmt.Sprintf("libraries[%d]: slug must match ^[a-z][a-z0-9-]*$", i))
			return
		}
		if lib.Name == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", fmt.Sprintf("libraries[%d]: name is required", i))
			return
		}
		if len(lib.Commands) == 0 {
			writeError(w, http.StatusBadRequest, "EMPTY_LIBRARY", fmt.Sprintf("libraries[%d] (%s): no commands", i, lib.Slug))
			return
		}
		if lib.ContentSHA256 == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", fmt.Sprintf("libraries[%d] (%s): content_sha256 is required", i, lib.Slug))
			return
		}
		for j, cmdJSON := range lib.Commands {
			if err := spec.Validate(cmdJSON); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_SPEC", fmt.Sprintf("libraries[%d] (%s) command[%d]: %s", i, lib.Slug, j, err.Error()))
				return
			}
		}
		// Server hashes received bytes as-is — no re-marshaling. This is the
		// contract that makes CLI-side canonicalization the single source of truth.
		gotHash, err := computeLibraryHash(lib)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SPEC", fmt.Sprintf("libraries[%d] (%s): %v", i, lib.Slug, err))
			return
		}
		if gotHash != lib.ContentSHA256 {
			writeError(w, http.StatusBadRequest, "HASH_MISMATCH", fmt.Sprintf("libraries[%d] (%s): client hash %s does not match server-recomputed hash %s", i, lib.Slug, lib.ContentSHA256, gotHash))
			return
		}
	}

	// Namespace authorization. "system" requires an admin email; anything else
	// defaults to the caller's own username.
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

	var gitURL *string
	if req.GitURL != "" {
		gitURL = &req.GitURL
	}

	// txErr signals the outer handler to write a 500. Anything user-facing
	// (409, 400) is written inside the tx via txWroteResponse and the tx is
	// rolled back by returning a sentinel.
	var txWroteResponse bool
	var results []bundledLibraryResp
	var errTxAbort = errors.New("tx: response written")

	err := h.store.WithTx(r.Context(), func(tx store.LibraryStore) error {
		results = make([]bundledLibraryResp, 0, len(req.Libraries))

		for _, lib := range req.Libraries {
			libModel, err := tx.CreateOrUpdateLibrary(r.Context(), ownerID, lib.Slug, lib.Name, lib.Description, gitURL, lib.Aliases)
			if err != nil {
				return fmt.Errorf("create/update library %q: %w", lib.Slug, err)
			}

			// Version freshness. Reject strictly older releases; equal is fine
			// because the idempotency path handles it below.
			if libModel.LatestVersion != nil {
				cmp, err := compareSemver(*libModel.LatestVersion, version)
				if err == nil && cmp > 0 {
					writeReleaseStale(w, lib.Slug, *libModel.LatestVersion, version)
					txWroteResponse = true
					return errTxAbort
				}
			}

			// Four-way idempotency:
			//   no row → create
			//   NULL hash (legacy) → RELEASE_EXISTS (opaque)
			//   matching hash → idempotent no-op
			//   different hash → RELEASE_CONTENT_MISMATCH
			existing, err := tx.GetLibraryRelease(r.Context(), libModel.ID, version)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("lookup existing release for %q: %w", lib.Slug, err)
			}
			if existing != nil {
				if existing.ContentSHA256 == nil {
					writeError(w, http.StatusConflict, "RELEASE_EXISTS", fmt.Sprintf("library %q: version %s already exists (legacy release, no content check possible)", lib.Slug, version))
					txWroteResponse = true
					return errTxAbort
				}
				if *existing.ContentSHA256 == lib.ContentSHA256 {
					results = append(results, bundledLibraryResp{
						Slug:           lib.Slug,
						ReleaseID:      existing.ID.String(),
						PublishedCount: int(existing.CommandCount),
						Status:         "idempotent",
					})
					continue
				}
				writeReleaseContentMismatch(w, lib.Slug, version, *existing.ContentSHA256, lib.ContentSHA256)
				txWroteResponse = true
				return errTxAbort
			}

			// Publish commands, prune removed slugs, record the release.
			published, publishedSlugs, err := publishCommands(r.Context(), tx, ownerID, libModel.ID, lib.Commands)
			if err != nil {
				return fmt.Errorf("publish commands for %q: %w", lib.Slug, err)
			}

			if err := softDeleteRemoved(r.Context(), tx, libModel.ID, publishedSlugs); err != nil {
				return fmt.Errorf("soft-delete stale commands for %q: %w", lib.Slug, err)
			}

			hashCopy := lib.ContentSHA256
			release, err := tx.CreateLibraryRelease(r.Context(), libModel.ID, version, req.Tag, req.CommitHash, &hashCopy, published, userID)
			if err != nil {
				return fmt.Errorf("create release for %q: %w", lib.Slug, err)
			}
			if err := tx.UpdateLibraryLatestVersion(r.Context(), libModel.ID, version); err != nil {
				return fmt.Errorf("update latest version for %q: %w", lib.Slug, err)
			}

			results = append(results, bundledLibraryResp{
				Slug:           lib.Slug,
				ReleaseID:      release.ID.String(),
				PublishedCount: published,
				Status:         "created",
			})
		}
		return nil
	})

	if txWroteResponse {
		return
	}
	if err != nil {
		log.Printf("bundled release: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create release")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tag":       req.Tag,
		"libraries": results,
	})
}

// publishCommands mirrors the library handler's version — split out so both
// handlers share it without stuttering. Returns published count and slug list.
func publishCommands(ctx context.Context, s store.LibraryStore, userID, libraryID uuid.UUID, commands []json.RawMessage) (int, []string, error) {
	published := 0
	var slugs []string
	for _, cmdJSON := range commands {
		parsed, err := spec.Parse(cmdJSON)
		if err != nil {
			return 0, nil, fmt.Errorf("parse spec: %w", err)
		}

		cmdSlug := parsed.Metadata.Slug
		tags, _ := json.Marshal(parsed.Metadata.Tags)
		if parsed.Metadata.Tags == nil {
			tags = []byte("[]")
		}

		cmd, err := s.GetCommandByLibraryAndSlug(ctx, libraryID, cmdSlug)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				return 0, nil, fmt.Errorf("get command %q: %w", cmdSlug, err)
			}
			cmd, err = s.CreateCommandForLibrary(ctx, userID, libraryID, parsed.Metadata.Name, cmdSlug, parsed.Metadata.Description, tags)
			if err != nil {
				return 0, nil, fmt.Errorf("create command %q: %w", cmdSlug, err)
			}
		} else if err := s.UpdateCommandMeta(ctx, cmd.ID, parsed.Metadata.Name, parsed.Metadata.Description, tags); err != nil {
			return 0, nil, fmt.Errorf("update command meta %q: %w", cmdSlug, err)
		}

		hash, err := spec.Hash(parsed)
		if err != nil {
			return 0, nil, fmt.Errorf("hash spec %q: %w", cmdSlug, err)
		}

		if existingHash, err := s.GetLatestHashByCommand(ctx, cmd.ID); err == nil && existingHash == hash {
			published++
			slugs = append(slugs, cmdSlug)
			continue
		}

		nextVersion := 1
		if latest, err := s.GetLatestVersionByCommand(ctx, cmd.ID); err == nil {
			nextVersion = latest.Version + 1
		}

		if _, err := s.CreateVersion(ctx, cmd.ID, nextVersion, cmdJSON, hash, "Published via library release", userID); err != nil {
			return 0, nil, fmt.Errorf("create version for %q: %w", cmdSlug, err)
		}
		published++
		slugs = append(slugs, cmdSlug)
	}
	return published, slugs, nil
}

func softDeleteRemoved(ctx context.Context, s store.LibraryStore, libraryID uuid.UUID, keptSlugs []string) error {
	kept := make(map[string]bool, len(keptSlugs))
	for _, s := range keptSlugs {
		kept[s] = true
	}
	current, err := s.ListCommandsByLibrary(ctx, libraryID)
	if err != nil {
		return fmt.Errorf("list current commands: %w", err)
	}
	for _, cmd := range current {
		if kept[cmd.Slug] {
			continue
		}
		if err := s.SoftDeleteCommand(ctx, cmd.CommandID); err != nil {
			return fmt.Errorf("soft-delete %q: %w", cmd.Slug, err)
		}
	}
	return nil
}

func computeLibraryHash(lib bundledLibraryReq) (string, error) {
	entries := make([]spec.SpecHashEntry, 0, len(lib.Commands))
	for _, cmdJSON := range lib.Commands {
		parsed, err := spec.Parse(cmdJSON)
		if err != nil {
			return "", fmt.Errorf("parse spec: %w", err)
		}
		// Use the received bytes verbatim so the hash matches whatever the CLI
		// hashed. Do not re-marshal.
		entries = append(entries, spec.SpecHashEntry{
			Slug:  parsed.Metadata.Slug,
			Bytes: append([]byte(nil), cmdJSON...),
		})
	}
	return spec.LibraryReleaseHash(spec.LibraryReleaseHashInput{
		Slug:        lib.Slug,
		Name:        lib.Name,
		Description: lib.Description,
		Aliases:     lib.Aliases,
		Specs:       entries,
	}), nil
}

// writeReleaseContentMismatch writes 409 RELEASE_CONTENT_MISMATCH with both
// hashes so the client can surface the drift concretely.
func writeReleaseContentMismatch(w http.ResponseWriter, slug, version, existingHash, requestedHash string) {
	writeJSON(w, http.StatusConflict, map[string]any{
		"error": map[string]any{
			"code":           "RELEASE_CONTENT_MISMATCH",
			"message":        fmt.Sprintf("library %q version %s already exists with different content", slug, version),
			"library":        slug,
			"version":        version,
			"existing_hash":  existingHash,
			"requested_hash": requestedHash,
		},
	})
}

func writeReleaseStale(w http.ResponseWriter, slug, latestVersion, requestedVersion string) {
	writeJSON(w, http.StatusConflict, map[string]any{
		"error": map[string]any{
			"code":              "RELEASE_STALE",
			"message":           fmt.Sprintf("library %q latest version %s is newer than requested %s", slug, latestVersion, requestedVersion),
			"library":           slug,
			"latest_version":    latestVersion,
			"requested_version": requestedVersion,
		},
	})
}

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// compareSemver returns -1, 0, +1 comparing two "vX.Y.Z" or "X.Y.Z" strings.
func compareSemver(a, b string) (int, error) {
	ap, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	bp, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1, nil
		}
		if ap[i] > bp[i] {
			return +1, nil
		}
	}
	return 0, nil
}

func parseSemver(s string) ([3]int, error) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, fmt.Errorf("not semver: %q", s)
	}
	var out [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, err
		}
		out[i] = n
	}
	return out, nil
}

// LibraryReleaseModel is a re-export for tests that want to build one without
// depending on the model package.
type LibraryReleaseModel = model.LibraryRelease
