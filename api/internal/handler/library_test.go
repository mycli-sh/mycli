package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

type mockLibraryStore struct {
	GetUserByIDFn                      func(ctx context.Context, id string) (*model.User, error)
	SearchPublicLibrariesFn            func(ctx context.Context, query string, limit, offset int) ([]model.Library, int, error)
	GetOwnerNameFn                     func(ctx context.Context, ownerID string) (string, error)
	GetLibraryByOwnerUsernameAndSlugFn func(ctx context.Context, ownerName, slug string) (*model.Library, error)
	GetLibraryBySlugFn                 func(ctx context.Context, slug string) (*model.Library, error)
	ListCommandsByLibraryFn            func(ctx context.Context, libraryID string) ([]store.LibraryCommand, error)
	IsLibraryInstalledFn               func(ctx context.Context, userID, libraryID string) bool
	GetCommandByLibraryAndSlugFn       func(ctx context.Context, libraryID, slug string) (*model.Command, error)
	CreateCommandForLibraryFn          func(ctx context.Context, ownerID, libraryID, name, slug, description string, tags json.RawMessage) (*model.Command, error)
	UpdateCommandMetaFn                func(ctx context.Context, id, name, description string, tags json.RawMessage) error
	GetLatestHashByCommandFn           func(ctx context.Context, commandID string) (string, error)
	GetLatestVersionByCommandFn        func(ctx context.Context, commandID string) (*model.CommandVersion, error)
	CreateVersionFn                    func(ctx context.Context, commandID string, version int, specJSON json.RawMessage, specHash, message, createdBy string) (*model.CommandVersion, error)
	ListVersionsByCommandFn            func(ctx context.Context, commandID string) ([]model.CommandVersion, error)
	CreateOrUpdateLibraryFn            func(ctx context.Context, ownerID, slug, name, description string, gitURL *string) (*model.Library, error)
	LibraryReleaseExistsFn             func(ctx context.Context, libraryID, version string) (bool, error)
	CreateLibraryReleaseFn             func(ctx context.Context, libraryID, version, tag, commitHash string, commandCount int, releasedBy string) (*model.LibraryRelease, error)
	UpdateLibraryLatestVersionFn       func(ctx context.Context, libraryID, version string) error
	InstallLibraryFn                   func(ctx context.Context, userID, libraryID string) error
	UninstallLibraryFn                 func(ctx context.Context, userID, libraryID string) error
	ListLibraryReleasesFn              func(ctx context.Context, libraryID string) ([]model.LibraryRelease, error)
	GetLibraryReleaseFn                func(ctx context.Context, libraryID, version string) (*model.LibraryRelease, error)
}

func (m *mockLibraryStore) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	if m.GetUserByIDFn != nil {
		return m.GetUserByIDFn(ctx, id)
	}
	return &model.User{ID: id, Email: "test@example.com"}, nil
}
func (m *mockLibraryStore) SearchPublicLibraries(ctx context.Context, query string, limit, offset int) ([]model.Library, int, error) {
	return m.SearchPublicLibrariesFn(ctx, query, limit, offset)
}
func (m *mockLibraryStore) GetOwnerName(ctx context.Context, ownerID string) (string, error) {
	return m.GetOwnerNameFn(ctx, ownerID)
}
func (m *mockLibraryStore) GetLibraryByOwnerUsernameAndSlug(ctx context.Context, ownerName, slug string) (*model.Library, error) {
	return m.GetLibraryByOwnerUsernameAndSlugFn(ctx, ownerName, slug)
}
func (m *mockLibraryStore) GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error) {
	return m.GetLibraryBySlugFn(ctx, slug)
}
func (m *mockLibraryStore) ListCommandsByLibrary(ctx context.Context, libraryID string) ([]store.LibraryCommand, error) {
	return m.ListCommandsByLibraryFn(ctx, libraryID)
}
func (m *mockLibraryStore) IsLibraryInstalled(ctx context.Context, userID, libraryID string) bool {
	if m.IsLibraryInstalledFn != nil {
		return m.IsLibraryInstalledFn(ctx, userID, libraryID)
	}
	return false
}
func (m *mockLibraryStore) GetCommandByLibraryAndSlug(ctx context.Context, libraryID, slug string) (*model.Command, error) {
	return m.GetCommandByLibraryAndSlugFn(ctx, libraryID, slug)
}
func (m *mockLibraryStore) CreateCommandForLibrary(ctx context.Context, ownerID, libraryID, name, slug, description string, tags json.RawMessage) (*model.Command, error) {
	return m.CreateCommandForLibraryFn(ctx, ownerID, libraryID, name, slug, description, tags)
}
func (m *mockLibraryStore) UpdateCommandMeta(ctx context.Context, id, name, description string, tags json.RawMessage) error {
	if m.UpdateCommandMetaFn != nil {
		return m.UpdateCommandMetaFn(ctx, id, name, description, tags)
	}
	return nil
}
func (m *mockLibraryStore) GetLatestHashByCommand(ctx context.Context, commandID string) (string, error) {
	return m.GetLatestHashByCommandFn(ctx, commandID)
}
func (m *mockLibraryStore) GetLatestVersionByCommand(ctx context.Context, commandID string) (*model.CommandVersion, error) {
	return m.GetLatestVersionByCommandFn(ctx, commandID)
}
func (m *mockLibraryStore) CreateVersion(ctx context.Context, commandID string, version int, specJSON json.RawMessage, specHash, message, createdBy string) (*model.CommandVersion, error) {
	return m.CreateVersionFn(ctx, commandID, version, specJSON, specHash, message, createdBy)
}
func (m *mockLibraryStore) ListVersionsByCommand(ctx context.Context, commandID string) ([]model.CommandVersion, error) {
	return m.ListVersionsByCommandFn(ctx, commandID)
}
func (m *mockLibraryStore) CreateOrUpdateLibrary(ctx context.Context, ownerID, slug, name, description string, gitURL *string) (*model.Library, error) {
	return m.CreateOrUpdateLibraryFn(ctx, ownerID, slug, name, description, gitURL)
}
func (m *mockLibraryStore) LibraryReleaseExists(ctx context.Context, libraryID, version string) (bool, error) {
	return m.LibraryReleaseExistsFn(ctx, libraryID, version)
}
func (m *mockLibraryStore) CreateLibraryRelease(ctx context.Context, libraryID, version, tag, commitHash string, commandCount int, releasedBy string) (*model.LibraryRelease, error) {
	return m.CreateLibraryReleaseFn(ctx, libraryID, version, tag, commitHash, commandCount, releasedBy)
}
func (m *mockLibraryStore) UpdateLibraryLatestVersion(ctx context.Context, libraryID, version string) error {
	return m.UpdateLibraryLatestVersionFn(ctx, libraryID, version)
}
func (m *mockLibraryStore) InstallLibrary(ctx context.Context, userID, libraryID string) error {
	return m.InstallLibraryFn(ctx, userID, libraryID)
}
func (m *mockLibraryStore) UninstallLibrary(ctx context.Context, userID, libraryID string) error {
	return m.UninstallLibraryFn(ctx, userID, libraryID)
}
func (m *mockLibraryStore) ListLibraryReleases(ctx context.Context, libraryID string) ([]model.LibraryRelease, error) {
	return m.ListLibraryReleasesFn(ctx, libraryID)
}
func (m *mockLibraryStore) GetLibraryRelease(ctx context.Context, libraryID, version string) (*model.LibraryRelease, error) {
	return m.GetLibraryReleaseFn(ctx, libraryID, version)
}

func TestLibraryHandler_Search(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		setupStore func(*mockLibraryStore)
		wantCode   int
		wantCount  int
	}{
		{
			name:  "returns results",
			query: "?q=kube",
			setupStore: func(ms *mockLibraryStore) {
				ownerID := "usr_1"
				ms.SearchPublicLibrariesFn = func(context.Context, string, int, int) ([]model.Library, int, error) {
					return []model.Library{
						{ID: "lib_1", Slug: "kubernetes", Name: "Kubernetes", OwnerID: &ownerID},
					}, 1, nil
				}
				ms.GetOwnerNameFn = func(context.Context, string) (string, error) {
					return "alice", nil
				}
			},
			wantCode:  http.StatusOK,
			wantCount: 1,
		},
		{
			name:  "empty results",
			query: "?q=nonexistent",
			setupStore: func(ms *mockLibraryStore) {
				ms.SearchPublicLibrariesFn = func(context.Context, string, int, int) ([]model.Library, int, error) {
					return nil, 0, nil
				}
			},
			wantCode:  http.StatusOK,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockLibraryStore{}
			tt.setupStore(ms)
			h := NewLibraryHandler(&config.Config{}, ms)

			r := chi.NewRouter()
			r.Get("/libraries", h.Search)

			req := requestWithUser("GET", "/libraries"+tt.query, nil, "")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}

			var resp map[string]any
			decodeJSON(t, rec, &resp)
			libs := resp["libraries"].([]any)
			if len(libs) != tt.wantCount {
				t.Errorf("got %d libraries, want %d", len(libs), tt.wantCount)
			}
		})
	}
}

func TestLibraryHandler_GetDetail(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		owner       string
		slug        string
		setupStore  func(*mockLibraryStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:  "found by owner/slug",
			owner: "alice",
			slug:  "kubernetes",
			setupStore: func(ms *mockLibraryStore) {
				ownerID := "usr_1"
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return &model.Library{ID: "lib_1", Slug: "kubernetes", OwnerID: &ownerID}, nil
				}
				ms.ListCommandsByLibraryFn = func(context.Context, string) ([]store.LibraryCommand, error) {
					return []store.LibraryCommand{
						{CommandID: "cmd_1", Slug: "deploy", UpdatedAt: now},
					}, nil
				}
				ms.GetOwnerNameFn = func(context.Context, string) (string, error) {
					return "alice", nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:  "fallback to slug-only lookup",
			owner: "system",
			slug:  "kubernetes",
			setupStore: func(ms *mockLibraryStore) {
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return nil, store.ErrNotFound
				}
				ms.GetLibraryBySlugFn = func(context.Context, string) (*model.Library, error) {
					return &model.Library{ID: "lib_1", Slug: "kubernetes"}, nil
				}
				ms.ListCommandsByLibraryFn = func(context.Context, string) ([]store.LibraryCommand, error) {
					return nil, nil
				}
				ms.GetOwnerNameFn = func(context.Context, string) (string, error) {
					return "", nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:  "not found",
			owner: "nobody",
			slug:  "nonexistent",
			setupStore: func(ms *mockLibraryStore) {
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return nil, store.ErrNotFound
				}
				ms.GetLibraryBySlugFn = func(context.Context, string) (*model.Library, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockLibraryStore{}
			tt.setupStore(ms)
			h := NewLibraryHandler(&config.Config{}, ms)

			r := chi.NewRouter()
			r.Get("/libraries/{owner}/{slug}", h.GetDetail)

			req := requestWithUser("GET", "/libraries/"+tt.owner+"/"+tt.slug, nil, "usr_viewer")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantErrCode != "" {
				var resp errorResponse
				decodeJSON(t, rec, &resp)
				if resp.Error.Code != tt.wantErrCode {
					t.Errorf("got error code %q, want %q", resp.Error.Code, tt.wantErrCode)
				}
			}
		})
	}
}

func TestLibraryHandler_Install(t *testing.T) {
	tests := []struct {
		name        string
		owner       string
		slug        string
		setupStore  func(*mockLibraryStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:  "success",
			owner: "alice",
			slug:  "kubernetes",
			setupStore: func(ms *mockLibraryStore) {
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return &model.Library{ID: "lib_1"}, nil
				}
				ms.InstallLibraryFn = func(context.Context, string, string) error { return nil }
			},
			wantCode: http.StatusOK,
		},
		{
			name:  "library not found",
			owner: "nobody",
			slug:  "nonexistent",
			setupStore: func(ms *mockLibraryStore) {
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return nil, store.ErrNotFound
				}
				ms.GetLibraryBySlugFn = func(context.Context, string) (*model.Library, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockLibraryStore{}
			tt.setupStore(ms)
			h := NewLibraryHandler(&config.Config{}, ms)

			r := chi.NewRouter()
			r.Post("/libraries/{owner}/{slug}/install", h.Install)

			req := requestWithUser("POST", "/libraries/"+tt.owner+"/"+tt.slug+"/install", nil, "usr_alice")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantErrCode != "" {
				var resp errorResponse
				decodeJSON(t, rec, &resp)
				if resp.Error.Code != tt.wantErrCode {
					t.Errorf("got error code %q, want %q", resp.Error.Code, tt.wantErrCode)
				}
			}
		})
	}
}

func TestLibraryHandler_Uninstall(t *testing.T) {
	ms := &mockLibraryStore{
		GetLibraryByOwnerUsernameAndSlugFn: func(context.Context, string, string) (*model.Library, error) {
			return &model.Library{ID: "lib_1"}, nil
		},
		UninstallLibraryFn: func(context.Context, string, string) error { return nil },
	}
	h := NewLibraryHandler(&config.Config{}, ms)

	r := chi.NewRouter()
	r.Delete("/libraries/{owner}/{slug}/install", h.Uninstall)

	req := requestWithUser("DELETE", "/libraries/alice/kubernetes/install", nil, "usr_alice")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("got status %d, want 204 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestLibraryHandler_CreateRelease(t *testing.T) {
	validSpec := json.RawMessage(`{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "deploy", "slug": "deploy"},
		"steps": [{"name": "run", "run": "echo hello"}]
	}`)

	tests := []struct {
		name        string
		slug        string
		body        any
		setupStore  func(*mockLibraryStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name: "success",
			slug: "kubernetes",
			body: map[string]any{
				"tag":         "v1.0.0",
				"commit_hash": "abc123",
				"name":        "Kubernetes",
				"description": "K8s commands",
				"commands":    []json.RawMessage{validSpec},
			},
			setupStore: func(ms *mockLibraryStore) {
				ms.CreateOrUpdateLibraryFn = func(context.Context, string, string, string, string, *string) (*model.Library, error) {
					return &model.Library{ID: "lib_1", Slug: "kubernetes"}, nil
				}
				ms.LibraryReleaseExistsFn = func(context.Context, string, string) (bool, error) {
					return false, nil
				}
				ms.GetCommandByLibraryAndSlugFn = func(context.Context, string, string) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateCommandForLibraryFn = func(_ context.Context, _, libID, name, slug, desc string, tags json.RawMessage) (*model.Command, error) {
					return &model.Command{ID: "cmd_1", Name: name, Slug: slug}, nil
				}
				ms.GetLatestHashByCommandFn = func(context.Context, string) (string, error) {
					return "", store.ErrNotFound
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, string) (*model.CommandVersion, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateVersionFn = func(_ context.Context, cmdID string, ver int, _ json.RawMessage, hash, _, _ string) (*model.CommandVersion, error) {
					return &model.CommandVersion{ID: "cv_1", CommandID: cmdID, Version: ver, SpecHash: hash}, nil
				}
				ms.CreateLibraryReleaseFn = func(_ context.Context, libID, version, tag, commit string, count int, by string) (*model.LibraryRelease, error) {
					return &model.LibraryRelease{
						ID: "lr_1", LibraryID: libID, Version: version, Tag: tag,
						CommitHash: commit, CommandCount: count, ReleasedBy: by,
						ReleasedAt: time.Now(),
					}, nil
				}
				ms.UpdateLibraryLatestVersionFn = func(context.Context, string, string) error {
					return nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name: "missing required fields",
			slug: "kubernetes",
			body: map[string]any{
				"name": "Kubernetes",
			},
			setupStore:  func(ms *mockLibraryStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name: "invalid tag format",
			slug: "kubernetes",
			body: map[string]any{
				"tag":  "1.0.0",
				"name": "Kubernetes",
			},
			setupStore:  func(ms *mockLibraryStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_TAG",
		},
		{
			name: "release already exists",
			slug: "kubernetes",
			body: map[string]any{
				"tag":      "v1.0.0",
				"name":     "Kubernetes",
				"commands": []json.RawMessage{},
			},
			setupStore: func(ms *mockLibraryStore) {
				ms.CreateOrUpdateLibraryFn = func(context.Context, string, string, string, string, *string) (*model.Library, error) {
					return &model.Library{ID: "lib_1"}, nil
				}
				ms.LibraryReleaseExistsFn = func(context.Context, string, string) (bool, error) {
					return true, nil
				}
			},
			wantCode:    http.StatusConflict,
			wantErrCode: "RELEASE_EXISTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockLibraryStore{}
			tt.setupStore(ms)
			h := NewLibraryHandler(&config.Config{}, ms)

			r := chi.NewRouter()
			r.Post("/libraries/{slug}/releases", h.CreateRelease)

			req := requestWithUser("POST", "/libraries/"+tt.slug+"/releases", tt.body, "usr_owner")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantErrCode != "" {
				var resp errorResponse
				decodeJSON(t, rec, &resp)
				if resp.Error.Code != tt.wantErrCode {
					t.Errorf("got error code %q, want %q", resp.Error.Code, tt.wantErrCode)
				}
			}
		})
	}
}

func TestLibraryHandler_GetCommand(t *testing.T) {
	tests := []struct {
		name        string
		owner       string
		slug        string
		cmdSlug     string
		setupStore  func(*mockLibraryStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:    "success",
			owner:   "alice",
			slug:    "kubernetes",
			cmdSlug: "deploy",
			setupStore: func(ms *mockLibraryStore) {
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return &model.Library{ID: "lib_1"}, nil
				}
				ms.GetCommandByLibraryAndSlugFn = func(context.Context, string, string) (*model.Command, error) {
					return &model.Command{ID: "cmd_1", Slug: "deploy"}, nil
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, string) (*model.CommandVersion, error) {
					return &model.CommandVersion{Version: 1}, nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:    "library not found",
			owner:   "nobody",
			slug:    "nonexistent",
			cmdSlug: "deploy",
			setupStore: func(ms *mockLibraryStore) {
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:    "command not found",
			owner:   "alice",
			slug:    "kubernetes",
			cmdSlug: "nonexistent",
			setupStore: func(ms *mockLibraryStore) {
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return &model.Library{ID: "lib_1"}, nil
				}
				ms.GetCommandByLibraryAndSlugFn = func(context.Context, string, string) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockLibraryStore{}
			tt.setupStore(ms)
			h := NewLibraryHandler(&config.Config{}, ms)

			r := chi.NewRouter()
			r.Get("/libraries/{owner}/{slug}/commands/{commandSlug}", h.GetCommand)

			req := requestWithUser("GET", "/libraries/"+tt.owner+"/"+tt.slug+"/commands/"+tt.cmdSlug, nil, "")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantErrCode != "" {
				var resp errorResponse
				decodeJSON(t, rec, &resp)
				if resp.Error.Code != tt.wantErrCode {
					t.Errorf("got error code %q, want %q", resp.Error.Code, tt.wantErrCode)
				}
			}
		})
	}
}

func TestLibraryHandler_ListReleases(t *testing.T) {
	ms := &mockLibraryStore{
		GetLibraryByOwnerUsernameAndSlugFn: func(context.Context, string, string) (*model.Library, error) {
			return &model.Library{ID: "lib_1"}, nil
		},
		ListLibraryReleasesFn: func(context.Context, string) ([]model.LibraryRelease, error) {
			return []model.LibraryRelease{
				{ID: "lr_1", Version: "1.0.0", Tag: "v1.0.0", ReleasedAt: time.Now()},
				{ID: "lr_2", Version: "1.1.0", Tag: "v1.1.0", ReleasedAt: time.Now()},
			}, nil
		},
	}
	h := NewLibraryHandler(&config.Config{}, ms)

	r := chi.NewRouter()
	r.Get("/libraries/{owner}/{slug}/releases", h.ListReleases)

	req := requestWithUser("GET", "/libraries/alice/kubernetes/releases", nil, "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	releases := resp["releases"].([]any)
	if len(releases) != 2 {
		t.Errorf("got %d releases, want 2", len(releases))
	}
}

func TestLibraryHandler_CreateRelease_SystemNamespace(t *testing.T) {
	validSpec := json.RawMessage(`{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "deploy", "slug": "deploy"},
		"steps": [{"name": "run", "run": "echo hello"}]
	}`)

	t.Run("admin can release to system namespace", func(t *testing.T) {
		var capturedOwnerID string
		ms := &mockLibraryStore{
			GetUserByIDFn: func(_ context.Context, id string) (*model.User, error) {
				return &model.User{ID: id, Email: "admin@example.com"}, nil
			},
			CreateOrUpdateLibraryFn: func(_ context.Context, ownerID, slug, name, desc string, gitURL *string) (*model.Library, error) {
				capturedOwnerID = ownerID
				return &model.Library{ID: "lib_1", Slug: slug}, nil
			},
			LibraryReleaseExistsFn: func(context.Context, string, string) (bool, error) {
				return false, nil
			},
			GetCommandByLibraryAndSlugFn: func(context.Context, string, string) (*model.Command, error) {
				return nil, store.ErrNotFound
			},
			CreateCommandForLibraryFn: func(_ context.Context, _, libID, name, slug, desc string, tags json.RawMessage) (*model.Command, error) {
				return &model.Command{ID: "cmd_1", Name: name, Slug: slug}, nil
			},
			GetLatestHashByCommandFn: func(context.Context, string) (string, error) {
				return "", store.ErrNotFound
			},
			GetLatestVersionByCommandFn: func(context.Context, string) (*model.CommandVersion, error) {
				return nil, store.ErrNotFound
			},
			CreateVersionFn: func(_ context.Context, cmdID string, ver int, _ json.RawMessage, hash, _, _ string) (*model.CommandVersion, error) {
				return &model.CommandVersion{ID: "cv_1", CommandID: cmdID, Version: ver, SpecHash: hash}, nil
			},
			CreateLibraryReleaseFn: func(_ context.Context, libID, version, tag, commit string, count int, by string) (*model.LibraryRelease, error) {
				return &model.LibraryRelease{
					ID: "lr_1", LibraryID: libID, Version: version, Tag: tag,
					CommitHash: commit, CommandCount: count, ReleasedBy: by,
					ReleasedAt: time.Now(),
				}, nil
			},
			UpdateLibraryLatestVersionFn: func(context.Context, string, string) error {
				return nil
			},
		}

		cfg := &config.Config{SystemAdminEmails: []string{"admin@example.com"}}
		h := NewLibraryHandler(cfg, ms)

		r := chi.NewRouter()
		r.Post("/libraries/{slug}/releases", h.CreateRelease)

		body := map[string]any{
			"tag":         "v1.0.0",
			"commit_hash": "abc123",
			"namespace":   "system",
			"name":        "Kubernetes",
			"commands":    []json.RawMessage{validSpec},
		}
		req := requestWithUser("POST", "/libraries/kubernetes/releases", body, "usr_admin")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if capturedOwnerID != "usr_system" {
			t.Errorf("ownerID = %q, want %q", capturedOwnerID, "usr_system")
		}
	})

	t.Run("non-admin rejected for system namespace", func(t *testing.T) {
		ms := &mockLibraryStore{
			GetUserByIDFn: func(_ context.Context, id string) (*model.User, error) {
				return &model.User{ID: id, Email: "regular@example.com"}, nil
			},
		}

		cfg := &config.Config{SystemAdminEmails: []string{"admin@example.com"}}
		h := NewLibraryHandler(cfg, ms)

		r := chi.NewRouter()
		r.Post("/libraries/{slug}/releases", h.CreateRelease)

		body := map[string]any{
			"tag":         "v1.0.0",
			"commit_hash": "abc123",
			"namespace":   "system",
			"name":        "Kubernetes",
			"commands":    []json.RawMessage{validSpec},
		}
		req := requestWithUser("POST", "/libraries/kubernetes/releases", body, "usr_regular")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("got status %d, want 403 (body=%s)", rec.Code, rec.Body.String())
		}
		var resp errorResponse
		decodeJSON(t, rec, &resp)
		if resp.Error.Code != "FORBIDDEN" {
			t.Errorf("got error code %q, want %q", resp.Error.Code, "FORBIDDEN")
		}
	})
}
