package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
	"mycli.sh/pkg/spec"
)

// canonSpec returns canonical spec bytes for a spec identified by slug —
// tests use these bytes both as the request payload and to compute the
// expected library hash, matching the CLI ↔ API contract.
func canonSpec(t *testing.T, slug string) []byte {
	t.Helper()
	src := []byte(`{
        "schemaVersion": 1,
        "kind": "command",
        "metadata": {"name": "` + slug + `", "slug": "` + slug + `"},
        "steps": [{"name": "run", "run": "echo hello"}]
    }`)
	c, err := spec.CanonicalSpecBytes(src)
	if err != nil {
		t.Fatalf("canonicalize %q: %v", slug, err)
	}
	return c
}

// libHash computes a per-library content hash for tests using the same
// package function the CLI uses.
func libHash(slug, name, description string, aliases []string, specs [][2]string) string {
	entries := make([]spec.SpecHashEntry, 0, len(specs))
	for _, s := range specs {
		entries = append(entries, spec.SpecHashEntry{Slug: s[0], Bytes: []byte(s[1])})
	}
	return spec.LibraryReleaseHash(spec.LibraryReleaseHashInput{
		Slug: slug, Name: name, Description: description, Aliases: aliases, Specs: entries,
	})
}

func TestReleaseHandler_CreateBundled_FreshRelease(t *testing.T) {
	specBytes := canonSpec(t, "deploy")
	hash := libHash("kubernetes", "K8s", "desc", nil, [][2]string{{"deploy", string(specBytes)}})

	var capturedHash *string
	ms := newReleaseMock(t)
	ms.CreateLibraryReleaseFn = func(_ context.Context, libID uuid.UUID, version, tag, commit string, contentSHA256 *string, count int, by uuid.UUID) (*model.LibraryRelease, error) {
		capturedHash = contentSHA256
		return &model.LibraryRelease{
			ID:        uuid.MustParse("00000000-0000-4000-8000-000000000060"),
			LibraryID: libID, Version: version, Tag: tag, CommitHash: commit,
			CommandCount: count, ReleasedBy: by, ReleasedAt: time.Now(),
		}, nil
	}

	rec := postBundled(t, ms, testUser2, map[string]any{
		"tag":         "v1.0.0",
		"commit_hash": "abc123",
		"libraries": []map[string]any{
			{
				"slug":           "kubernetes",
				"name":           "K8s",
				"description":    "desc",
				"content_sha256": hash,
				"commands":       []json.RawMessage{specBytes},
			},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Tag       string               `json:"tag"`
		Libraries []bundledLibraryResp `json:"libraries"`
	}
	decodeJSON(t, rec, &resp)
	if len(resp.Libraries) != 1 || resp.Libraries[0].Status != "created" {
		t.Errorf("expected one created library, got %+v", resp.Libraries)
	}
	if capturedHash == nil || *capturedHash != hash {
		t.Errorf("stored content_sha256 = %v, want %q", capturedHash, hash)
	}
}

func TestReleaseHandler_CreateBundled_IdempotentReplay(t *testing.T) {
	specBytes := canonSpec(t, "deploy")
	hash := libHash("kubernetes", "K8s", "", nil, [][2]string{{"deploy", string(specBytes)}})

	ms := newReleaseMock(t)
	ms.GetLibraryReleaseFn = func(context.Context, uuid.UUID, string) (*model.LibraryRelease, error) {
		h := hash
		return &model.LibraryRelease{
			ID:      uuid.MustParse("00000000-0000-4000-8000-000000000060"),
			Version: "1.0.0", Tag: "v1.0.0", CommitHash: "abc", CommandCount: 1,
			ContentSHA256: &h, ReleasedAt: time.Now(),
		}, nil
	}
	// CreateLibraryReleaseFn must NOT be called on the idempotent path.
	ms.CreateLibraryReleaseFn = func(context.Context, uuid.UUID, string, string, string, *string, int, uuid.UUID) (*model.LibraryRelease, error) {
		t.Fatal("CreateLibraryRelease called on idempotent replay")
		return nil, nil
	}
	ms.UpdateLibraryLatestVersionFn = func(context.Context, uuid.UUID, string) error {
		t.Fatal("UpdateLibraryLatestVersion called on idempotent replay")
		return nil
	}

	rec := postBundled(t, ms, testUser2, bundledBody("v1.0.0", "kubernetes", "K8s", hash, [][]byte{specBytes}))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Libraries []bundledLibraryResp `json:"libraries"`
	}
	decodeJSON(t, rec, &resp)
	if resp.Libraries[0].Status != "idempotent" {
		t.Errorf("expected idempotent status, got %q", resp.Libraries[0].Status)
	}
}

func TestReleaseHandler_CreateBundled_ContentMismatch(t *testing.T) {
	specBytes := canonSpec(t, "deploy")
	newHash := libHash("kubernetes", "K8s", "", nil, [][2]string{{"deploy", string(specBytes)}})

	ms := newReleaseMock(t)
	ms.GetLibraryReleaseFn = func(context.Context, uuid.UUID, string) (*model.LibraryRelease, error) {
		old := "sha256:deadbeef" // different from newHash
		return &model.LibraryRelease{
			ID:      uuid.MustParse("00000000-0000-4000-8000-000000000060"),
			Version: "1.0.0", ContentSHA256: &old, ReleasedAt: time.Now(),
		}, nil
	}

	rec := postBundled(t, ms, testUser2, bundledBody("v1.0.0", "kubernetes", "K8s", newHash, [][]byte{specBytes}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("got status %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	errMap := resp["error"].(map[string]any)
	if errMap["code"] != "RELEASE_CONTENT_MISMATCH" {
		t.Errorf("got error code %q, want RELEASE_CONTENT_MISMATCH", errMap["code"])
	}
}

func TestReleaseHandler_CreateBundled_LegacyRow(t *testing.T) {
	specBytes := canonSpec(t, "deploy")
	hash := libHash("kubernetes", "K8s", "", nil, [][2]string{{"deploy", string(specBytes)}})

	ms := newReleaseMock(t)
	ms.GetLibraryReleaseFn = func(context.Context, uuid.UUID, string) (*model.LibraryRelease, error) {
		return &model.LibraryRelease{
			ID:      uuid.MustParse("00000000-0000-4000-8000-000000000060"),
			Version: "1.0.0", ContentSHA256: nil, ReleasedAt: time.Now(),
		}, nil
	}

	rec := postBundled(t, ms, testUser2, bundledBody("v1.0.0", "kubernetes", "K8s", hash, [][]byte{specBytes}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("got status %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp errorResponse
	decodeJSON(t, rec, &resp)
	if resp.Error.Code != "RELEASE_EXISTS" {
		t.Errorf("got error code %q, want RELEASE_EXISTS", resp.Error.Code)
	}
}

func TestReleaseHandler_CreateBundled_StaleVersion(t *testing.T) {
	specBytes := canonSpec(t, "deploy")
	hash := libHash("kubernetes", "K8s", "", nil, [][2]string{{"deploy", string(specBytes)}})

	ms := newReleaseMock(t)
	ms.CreateOrUpdateLibraryFn = func(context.Context, uuid.UUID, string, string, string, *string, []string) (*model.Library, error) {
		latest := "2.0.0"
		return &model.Library{ID: testLib1, Slug: "kubernetes", LatestVersion: &latest}, nil
	}

	rec := postBundled(t, ms, testUser2, bundledBody("v1.0.0", "kubernetes", "K8s", hash, [][]byte{specBytes}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("got status %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rec, &resp)
	errMap := resp["error"].(map[string]any)
	if errMap["code"] != "RELEASE_STALE" {
		t.Errorf("got error code %q, want RELEASE_STALE", errMap["code"])
	}
}

func TestReleaseHandler_CreateBundled_HashMismatch(t *testing.T) {
	specBytes := canonSpec(t, "deploy")

	ms := newReleaseMock(t)

	rec := postBundled(t, ms, testUser2, bundledBody("v1.0.0", "kubernetes", "K8s", "sha256:wrong", [][]byte{specBytes}))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp errorResponse
	decodeJSON(t, rec, &resp)
	if resp.Error.Code != "HASH_MISMATCH" {
		t.Errorf("got error code %q, want HASH_MISMATCH", resp.Error.Code)
	}
}

func TestReleaseHandler_CreateBundled_EmptyLibraryRejected(t *testing.T) {
	ms := newReleaseMock(t)
	rec := postBundled(t, ms, testUser2, map[string]any{
		"tag":         "v1.0.0",
		"commit_hash": "abc",
		"libraries": []map[string]any{
			{
				"slug":           "kubernetes",
				"name":           "K8s",
				"content_sha256": "sha256:anything",
				"commands":       []json.RawMessage{},
			},
		},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp errorResponse
	decodeJSON(t, rec, &resp)
	if resp.Error.Code != "EMPTY_LIBRARY" {
		t.Errorf("got error code %q, want EMPTY_LIBRARY", resp.Error.Code)
	}
}

func TestReleaseHandler_CreateBundled_SoftDeleteErrorRollsBack(t *testing.T) {
	specBytes := canonSpec(t, "deploy")
	hash := libHash("kubernetes", "K8s", "", nil, [][2]string{{"deploy", string(specBytes)}})
	staleCmd := uuid.MustParse("00000000-0000-4000-8000-000000000099")

	var createReleaseCalled bool
	ms := newReleaseMock(t)
	ms.ListCommandsByLibraryFn = func(context.Context, uuid.UUID) ([]store.LibraryCommand, error) {
		return []store.LibraryCommand{
			{CommandID: testCmd1, Slug: "deploy"},
			{CommandID: staleCmd, Slug: "old"}, // will be soft-deleted; error propagates
		}, nil
	}
	ms.SoftDeleteCommandFn = func(context.Context, uuid.UUID) error {
		return errFake("db down")
	}
	ms.CreateLibraryReleaseFn = func(context.Context, uuid.UUID, string, string, string, *string, int, uuid.UUID) (*model.LibraryRelease, error) {
		createReleaseCalled = true
		return &model.LibraryRelease{ID: uuid.MustParse("00000000-0000-4000-8000-000000000060")}, nil
	}

	rec := postBundled(t, ms, testUser2, bundledBody("v1.0.0", "kubernetes", "K8s", hash, [][]byte{specBytes}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500 (body=%s)", rec.Code, rec.Body.String())
	}
	if createReleaseCalled {
		t.Error("CreateLibraryRelease was called after soft-delete failed — expected rollback before release insert")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newReleaseMock(t *testing.T) *mockLibraryStore {
	t.Helper()
	return &mockLibraryStore{
		CreateOrUpdateLibraryFn: func(context.Context, uuid.UUID, string, string, string, *string, []string) (*model.Library, error) {
			return &model.Library{ID: testLib1, Slug: "kubernetes"}, nil
		},
		GetLibraryReleaseFn: func(context.Context, uuid.UUID, string) (*model.LibraryRelease, error) {
			return nil, store.ErrNotFound
		},
		GetCommandByLibraryAndSlugFn: func(context.Context, uuid.UUID, string) (*model.Command, error) {
			return nil, store.ErrNotFound
		},
		CreateCommandForLibraryFn: func(_ context.Context, _, _ uuid.UUID, name, slug, _ string, _ json.RawMessage) (*model.Command, error) {
			return &model.Command{ID: testCmd1, Name: name, Slug: slug}, nil
		},
		GetLatestHashByCommandFn: func(context.Context, uuid.UUID) (string, error) {
			return "", store.ErrNotFound
		},
		GetLatestVersionByCommandFn: func(context.Context, uuid.UUID) (*model.CommandVersion, error) {
			return nil, store.ErrNotFound
		},
		CreateVersionFn: func(_ context.Context, cmdID uuid.UUID, ver int, _ json.RawMessage, hash, _ string, _ uuid.UUID) (*model.CommandVersion, error) {
			return &model.CommandVersion{ID: testCV1, CommandID: cmdID, Version: ver, SpecHash: hash}, nil
		},
		CreateLibraryReleaseFn: func(_ context.Context, libID uuid.UUID, version, tag, commit string, _ *string, count int, by uuid.UUID) (*model.LibraryRelease, error) {
			return &model.LibraryRelease{
				ID:        uuid.MustParse("00000000-0000-4000-8000-000000000060"),
				LibraryID: libID, Version: version, Tag: tag, CommitHash: commit,
				CommandCount: count, ReleasedBy: by, ReleasedAt: time.Now(),
			}, nil
		},
		UpdateLibraryLatestVersionFn: func(context.Context, uuid.UUID, string) error {
			return nil
		},
	}
}

func bundledBody(tag, slug, name, hash string, specs [][]byte) map[string]any {
	cmds := make([]json.RawMessage, len(specs))
	for i, s := range specs {
		cmds[i] = s
	}
	return map[string]any{
		"tag":         tag,
		"commit_hash": "abc123",
		"libraries": []map[string]any{
			{
				"slug":           slug,
				"name":           name,
				"content_sha256": hash,
				"commands":       cmds,
			},
		},
	}
}

func postBundled(t *testing.T, ms *mockLibraryStore, userID uuid.UUID, body any) *httptest.ResponseRecorder {
	t.Helper()
	h := NewReleaseHandler(&config.Config{}, ms)
	r := chi.NewRouter()
	r.Post("/v1/releases", h.CreateBundled)
	req := requestWithUser("POST", "/v1/releases", body, userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

type errFake string

func (e errFake) Error() string { return string(e) }
