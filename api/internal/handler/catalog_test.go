package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

type mockCatalogStore struct {
	ListCommandsByOwnerFn       func(ctx context.Context, ownerID, cursor string, limit int, query string) ([]model.Command, string, error)
	GetLatestVersionByCommandFn func(ctx context.Context, commandID string) (*model.CommandVersion, error)
	GetInstalledLibrariesFn     func(ctx context.Context, userID string) ([]model.Library, error)
	GetOwnerNameFn              func(ctx context.Context, ownerID string) (string, error)
	ListCommandsByLibraryFn     func(ctx context.Context, libraryID string) ([]store.LibraryCommand, error)
}

func (m *mockCatalogStore) ListCommandsByOwner(ctx context.Context, ownerID, cursor string, limit int, query string) ([]model.Command, string, error) {
	return m.ListCommandsByOwnerFn(ctx, ownerID, cursor, limit, query)
}
func (m *mockCatalogStore) GetLatestVersionByCommand(ctx context.Context, commandID string) (*model.CommandVersion, error) {
	return m.GetLatestVersionByCommandFn(ctx, commandID)
}
func (m *mockCatalogStore) GetInstalledLibraries(ctx context.Context, userID string) ([]model.Library, error) {
	return m.GetInstalledLibrariesFn(ctx, userID)
}
func (m *mockCatalogStore) GetOwnerName(ctx context.Context, ownerID string) (string, error) {
	return m.GetOwnerNameFn(ctx, ownerID)
}
func (m *mockCatalogStore) ListCommandsByLibrary(ctx context.Context, libraryID string) ([]store.LibraryCommand, error) {
	return m.ListCommandsByLibraryFn(ctx, libraryID)
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
				ms.ListCommandsByOwnerFn = func(_ context.Context, _ string, _ string, _ int, _ string) ([]model.Command, string, error) {
					return []model.Command{
						{ID: "cmd_1", Slug: "deploy", Name: "Deploy", UpdatedAt: now},
					}, "", nil
				}
				ms.GetLatestVersionByCommandFn = func(_ context.Context, cmdID string) (*model.CommandVersion, error) {
					if cmdID == "cmd_1" {
						return &model.CommandVersion{Version: 2, SpecHash: "hash1"}, nil
					}
					return nil, store.ErrNotFound
				}
				ms.GetInstalledLibrariesFn = func(context.Context, string) ([]model.Library, error) {
					return nil, nil
				}
			},
			wantCode:  http.StatusOK,
			wantItems: 1,
		},
		{
			name: "empty catalog",
			setupStore: func(ms *mockCatalogStore) {
				ms.ListCommandsByOwnerFn = func(context.Context, string, string, int, string) ([]model.Command, string, error) {
					return nil, "", nil
				}
				ms.GetInstalledLibrariesFn = func(context.Context, string) ([]model.Library, error) {
					return nil, nil
				}
			},
			wantCode:  http.StatusOK,
			wantItems: 0,
		},
		{
			name: "includes library commands",
			setupStore: func(ms *mockCatalogStore) {
				ms.ListCommandsByOwnerFn = func(context.Context, string, string, int, string) ([]model.Command, string, error) {
					return nil, "", nil
				}
				ownerID := "usr_lib_owner"
				ms.GetInstalledLibrariesFn = func(context.Context, string) ([]model.Library, error) {
					return []model.Library{
						{ID: "lib_1", Slug: "kubernetes", OwnerID: &ownerID},
					}, nil
				}
				ms.GetOwnerNameFn = func(context.Context, string) (string, error) {
					return "kube-author", nil
				}
				ms.ListCommandsByLibraryFn = func(context.Context, string) ([]store.LibraryCommand, error) {
					return []store.LibraryCommand{
						{CommandID: "cmd_lib_1", Slug: "deploy-k8s", Name: "Deploy K8s", UpdatedAt: now},
					}, nil
				}
				ms.GetLatestVersionByCommandFn = func(_ context.Context, cmdID string) (*model.CommandVersion, error) {
					return &model.CommandVersion{Version: 1, SpecHash: "libhash"}, nil
				}
			},
			wantCode:  http.StatusOK,
			wantItems: 1,
		},
		{
			name: "skips commands without versions",
			setupStore: func(ms *mockCatalogStore) {
				ms.ListCommandsByOwnerFn = func(context.Context, string, string, int, string) ([]model.Command, string, error) {
					return []model.Command{
						{ID: "cmd_no_ver", Slug: "no-version", Name: "No Version", UpdatedAt: now},
					}, "", nil
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, string) (*model.CommandVersion, error) {
					return nil, store.ErrNotFound
				}
				ms.GetInstalledLibrariesFn = func(context.Context, string) ([]model.Library, error) {
					return nil, nil
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

			req := requestWithUser("GET", "/catalog", nil, "usr_owner")
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

				// Verify ETag header is set
				if rec.Header().Get("ETag") == "" {
					t.Error("expected ETag header")
				}
			}
		})
	}
}

func TestCatalogHandler_GetCatalog_ETag(t *testing.T) {
	now := time.Now()
	ms := &mockCatalogStore{
		ListCommandsByOwnerFn: func(context.Context, string, string, int, string) ([]model.Command, string, error) {
			return []model.Command{
				{ID: "cmd_1", Slug: "deploy", Name: "Deploy", UpdatedAt: now},
			}, "", nil
		},
		GetLatestVersionByCommandFn: func(context.Context, string) (*model.CommandVersion, error) {
			return &model.CommandVersion{Version: 1, SpecHash: "hash1"}, nil
		},
		GetInstalledLibrariesFn: func(context.Context, string) ([]model.Library, error) {
			return nil, nil
		},
	}
	h := NewCatalogHandler(ms)

	r := chi.NewRouter()
	r.Get("/catalog", h.GetCatalog)

	// First request — get the ETag
	req := requestWithUser("GET", "/catalog", nil, "usr_owner")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	// Second request — with matching ETag
	req = requestWithUser("GET", "/catalog", nil, "usr_owner")
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Errorf("got status %d, want 304", rec.Code)
	}
}
