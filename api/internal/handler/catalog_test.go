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

	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

var (
	testDefaultProfileID = uuid.MustParse("00000000-0000-7000-8000-000000000099")
)

type mockCatalogStore struct {
	ListCommandsByOwnerFn       func(ctx context.Context, ownerID uuid.UUID, cursor string, limit int, query string) ([]model.Command, string, error)
	GetLatestVersionByCommandFn func(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error)
	GetOwnerNameFn              func(ctx context.Context, ownerID uuid.UUID) (string, error)
	ListCommandsByLibraryFn     func(ctx context.Context, libraryID uuid.UUID) ([]store.LibraryCommand, error)
	ListProfileLibrariesFn      func(ctx context.Context, profileID uuid.UUID) ([]model.Library, error)
	GetDefaultProfileFn         func(ctx context.Context, ownerID uuid.UUID) (*model.Profile, error)
}

func (m *mockCatalogStore) ListCommandsByOwner(ctx context.Context, ownerID uuid.UUID, cursor string, limit int, query string) ([]model.Command, string, error) {
	return m.ListCommandsByOwnerFn(ctx, ownerID, cursor, limit, query)
}
func (m *mockCatalogStore) GetLatestVersionByCommand(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error) {
	return m.GetLatestVersionByCommandFn(ctx, commandID)
}
func (m *mockCatalogStore) GetOwnerName(ctx context.Context, ownerID uuid.UUID) (string, error) {
	if m.GetOwnerNameFn != nil {
		return m.GetOwnerNameFn(ctx, ownerID)
	}
	return "", nil
}
func (m *mockCatalogStore) ListCommandsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]store.LibraryCommand, error) {
	if m.ListCommandsByLibraryFn != nil {
		return m.ListCommandsByLibraryFn(ctx, libraryID)
	}
	return nil, nil
}
func (m *mockCatalogStore) GetProfileByOwnerAndSlug(_ context.Context, _ uuid.UUID, _ string) (*model.Profile, error) {
	return nil, store.ErrNotFound
}
func (m *mockCatalogStore) ListProfileLibraries(_ context.Context, profileID uuid.UUID) ([]model.Library, error) {
	if m.ListProfileLibrariesFn != nil {
		return m.ListProfileLibrariesFn(nil, profileID)
	}
	return nil, nil
}
func (m *mockCatalogStore) GetDefaultProfile(ctx context.Context, ownerID uuid.UUID) (*model.Profile, error) {
	if m.GetDefaultProfileFn != nil {
		return m.GetDefaultProfileFn(ctx, ownerID)
	}
	return &model.Profile{ID: testDefaultProfileID, Slug: "default", IsDefault: true, OwnerUserID: ownerID}, nil
}

func TestCatalogHandler_GetCatalog(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		ifNoneMatch string
		setupStore  func(*mockCatalogStore)
		wantCode    int
		wantItems   int
	}{
		{
			name: "returns user commands with versions",
			setupStore: func(ms *mockCatalogStore) {
				ms.ListCommandsByOwnerFn = func(_ context.Context, _ uuid.UUID, _ string, _ int, _ string) ([]model.Command, string, error) {
					return []model.Command{
						{ID: testCmd1, Slug: "deploy", Name: "Deploy", UpdatedAt: now},
					}, "", nil
				}
				ms.GetLatestVersionByCommandFn = func(_ context.Context, cmdID uuid.UUID) (*model.CommandVersion, error) {
					if cmdID == testCmd1 {
						return &model.CommandVersion{Version: 2, SpecHash: "hash1"}, nil
					}
					return nil, store.ErrNotFound
				}
			},
			wantCode:  http.StatusOK,
			wantItems: 1,
		},
		{
			name: "empty catalog",
			setupStore: func(ms *mockCatalogStore) {
				ms.ListCommandsByOwnerFn = func(context.Context, uuid.UUID, string, int, string) ([]model.Command, string, error) {
					return nil, "", nil
				}
			},
			wantCode:  http.StatusOK,
			wantItems: 0,
		},
		{
			name: "includes library commands from the user's default profile",
			setupStore: func(ms *mockCatalogStore) {
				ms.ListCommandsByOwnerFn = func(context.Context, uuid.UUID, string, int, string) ([]model.Command, string, error) {
					return nil, "", nil
				}
				ms.ListProfileLibrariesFn = func(_ context.Context, _ uuid.UUID) ([]model.Library, error) {
					return []model.Library{
						{ID: testLib1, Slug: "kubernetes", OwnerID: &testLibOwner},
					}, nil
				}
				ms.GetOwnerNameFn = func(context.Context, uuid.UUID) (string, error) {
					return "kube-author", nil
				}
				ms.ListCommandsByLibraryFn = func(context.Context, uuid.UUID) ([]store.LibraryCommand, error) {
					return []store.LibraryCommand{
						{CommandID: testCmdLib1, Slug: "deploy-k8s", Name: "Deploy K8s", UpdatedAt: now},
					}, nil
				}
				ms.GetLatestVersionByCommandFn = func(_ context.Context, cmdID uuid.UUID) (*model.CommandVersion, error) {
					return &model.CommandVersion{Version: 1, SpecHash: "libhash"}, nil
				}
			},
			wantCode:  http.StatusOK,
			wantItems: 1,
		},
		{
			name: "skips commands without versions",
			setupStore: func(ms *mockCatalogStore) {
				ms.ListCommandsByOwnerFn = func(context.Context, uuid.UUID, string, int, string) ([]model.Command, string, error) {
					return []model.Command{
						{ID: testCmd1, Slug: "no-version", Name: "No Version", UpdatedAt: now},
					}, "", nil
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, uuid.UUID) (*model.CommandVersion, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:  http.StatusOK,
			wantItems: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockCatalogStore{}
			tt.setupStore(ms)
			h := NewCatalogHandler(ms)

			r := chi.NewRouter()
			r.Get("/catalog", h.GetCatalog)

			req := requestWithUser("GET", "/catalog", nil, testUser2)
			if tt.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tt.ifNoneMatch)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}

			if tt.wantCode == http.StatusOK {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				items := resp["items"].([]any)
				if len(items) != tt.wantItems {
					t.Errorf("got %d items, want %d", len(items), tt.wantItems)
				}

				if rec.Header().Get("ETag") == "" {
					t.Error("expected ETag header")
				}
			}
		})
	}
}

func TestCatalogHandler_GetCatalog_IncludesPerCommandAliases(t *testing.T) {
	now := time.Now()
	specWithAliases := json.RawMessage(`{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Port Forward", "slug": "port-forward", "aliases": ["pf"]},
		"steps": [{"name": "run", "run": "echo hello"}]
	}`)
	specNoAliases := json.RawMessage(`{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Deploy", "slug": "deploy"},
		"steps": [{"name": "run", "run": "echo hello"}]
	}`)

	ms := &mockCatalogStore{
		ListCommandsByOwnerFn: func(context.Context, uuid.UUID, string, int, string) ([]model.Command, string, error) {
			return []model.Command{
				{ID: testCmd1, Slug: "port-forward", Name: "Port Forward", UpdatedAt: now},
				{ID: testCmd2, Slug: "deploy", Name: "Deploy", UpdatedAt: now},
			}, "", nil
		},
		GetLatestVersionByCommandFn: func(_ context.Context, cmdID uuid.UUID) (*model.CommandVersion, error) {
			switch cmdID {
			case testCmd1:
				return &model.CommandVersion{Version: 1, SpecHash: "h1", SpecJSON: specWithAliases}, nil
			case testCmd2:
				return &model.CommandVersion{Version: 1, SpecHash: "h2", SpecJSON: specNoAliases}, nil
			}
			return nil, store.ErrNotFound
		},
	}
	h := NewCatalogHandler(ms)

	r := chi.NewRouter()
	r.Get("/catalog", h.GetCatalog)
	req := requestWithUser("GET", "/catalog", nil, testUser2)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	var resp struct {
		Items []struct {
			Slug    string   `json:"slug"`
			Aliases []string `json:"aliases"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	got := make(map[string][]string)
	for _, it := range resp.Items {
		got[it.Slug] = it.Aliases
	}

	if len(got["port-forward"]) != 1 || got["port-forward"][0] != "pf" {
		t.Errorf("port-forward aliases = %v, want [pf]", got["port-forward"])
	}
	if len(got["deploy"]) != 0 {
		t.Errorf("deploy aliases = %v, want empty/omitted", got["deploy"])
	}
}

func TestCatalogHandler_GetCatalog_ETag(t *testing.T) {
	now := time.Now()
	ms := &mockCatalogStore{
		ListCommandsByOwnerFn: func(context.Context, uuid.UUID, string, int, string) ([]model.Command, string, error) {
			return []model.Command{
				{ID: testCmd1, Slug: "deploy", Name: "Deploy", UpdatedAt: now},
			}, "", nil
		},
		GetLatestVersionByCommandFn: func(context.Context, uuid.UUID) (*model.CommandVersion, error) {
			return &model.CommandVersion{Version: 1, SpecHash: "hash1"}, nil
		},
	}
	h := NewCatalogHandler(ms)

	r := chi.NewRouter()
	r.Get("/catalog", h.GetCatalog)

	req := requestWithUser("GET", "/catalog", nil, testUser2)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	req = requestWithUser("GET", "/catalog", nil, testUser2)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Errorf("got status %d, want 304", rec.Code)
	}
}
