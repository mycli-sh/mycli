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
)

type mockLibraryStore struct {
	GetUserByIDFn                      func(ctx context.Context, id uuid.UUID) (*model.User, error)
	SearchPublicLibrariesFn            func(ctx context.Context, query string, limit, offset int) ([]model.Library, int, error)
	GetOwnerNameFn                     func(ctx context.Context, ownerID uuid.UUID) (string, error)
	GetLibraryByOwnerUsernameAndSlugFn func(ctx context.Context, ownerName, slug string) (*model.Library, error)
	GetLibraryBySlugFn                 func(ctx context.Context, slug string) (*model.Library, error)
	ListCommandsByLibraryFn            func(ctx context.Context, libraryID uuid.UUID) ([]store.LibraryCommand, error)
	IsLibraryInstalledFn               func(ctx context.Context, userID, libraryID uuid.UUID) bool
	GetCommandByLibraryAndSlugFn       func(ctx context.Context, libraryID uuid.UUID, slug string) (*model.Command, error)
	SoftDeleteCommandFn                func(ctx context.Context, id uuid.UUID) error
	CreateCommandForLibraryFn          func(ctx context.Context, ownerID, libraryID uuid.UUID, name, slug, description string, tags json.RawMessage) (*model.Command, error)
	UpdateCommandMetaFn                func(ctx context.Context, id uuid.UUID, name, description string, tags json.RawMessage) error
	GetLatestHashByCommandFn           func(ctx context.Context, commandID uuid.UUID) (string, error)
	GetLatestVersionByCommandFn        func(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error)
	CreateVersionFn                    func(ctx context.Context, commandID uuid.UUID, version int, specJSON json.RawMessage, specHash, message string, createdBy uuid.UUID) (*model.CommandVersion, error)
	ListVersionsByCommandFn            func(ctx context.Context, commandID uuid.UUID) ([]model.CommandVersion, error)
	CreateOrUpdateLibraryFn            func(ctx context.Context, ownerID uuid.UUID, slug, name, description string, gitURL *string) (*model.Library, error)
	LibraryReleaseExistsFn             func(ctx context.Context, libraryID uuid.UUID, version string) (bool, error)
	CreateLibraryReleaseFn             func(ctx context.Context, libraryID uuid.UUID, version, tag, commitHash string, commandCount int, releasedBy uuid.UUID) (*model.LibraryRelease, error)
	UpdateLibraryLatestVersionFn       func(ctx context.Context, libraryID uuid.UUID, version string) error
	InstallLibraryFn                   func(ctx context.Context, userID, libraryID uuid.UUID) error
	UninstallLibraryFn                 func(ctx context.Context, userID, libraryID uuid.UUID) error
	ListLibraryReleasesFn              func(ctx context.Context, libraryID uuid.UUID) ([]model.LibraryRelease, error)
	GetLibraryReleaseFn                func(ctx context.Context, libraryID uuid.UUID, version string) (*model.LibraryRelease, error)
}

func (m *mockLibraryStore) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.GetUserByIDFn != nil {
		return m.GetUserByIDFn(ctx, id)
	}
	return &model.User{ID: id, Email: "test@example.com"}, nil
}
func (m *mockLibraryStore) SearchPublicLibraries(ctx context.Context, query string, limit, offset int) ([]model.Library, int, error) {
	return m.SearchPublicLibrariesFn(ctx, query, limit, offset)
}
func (m *mockLibraryStore) GetOwnerName(ctx context.Context, ownerID uuid.UUID) (string, error) {
	return m.GetOwnerNameFn(ctx, ownerID)
}
func (m *mockLibraryStore) GetLibraryByOwnerUsernameAndSlug(ctx context.Context, ownerName, slug string) (*model.Library, error) {
	return m.GetLibraryByOwnerUsernameAndSlugFn(ctx, ownerName, slug)
}
func (m *mockLibraryStore) GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error) {
	return m.GetLibraryBySlugFn(ctx, slug)
}
func (m *mockLibraryStore) ListCommandsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]store.LibraryCommand, error) {
	if m.ListCommandsByLibraryFn != nil {
		return m.ListCommandsByLibraryFn(ctx, libraryID)
	}
	return nil, nil
}
func (m *mockLibraryStore) IsLibraryInstalled(ctx context.Context, userID, libraryID uuid.UUID) bool {
	if m.IsLibraryInstalledFn != nil {
		return m.IsLibraryInstalledFn(ctx, userID, libraryID)
	}
	return false
}
func (m *mockLibraryStore) GetCommandByLibraryAndSlug(ctx context.Context, libraryID uuid.UUID, slug string) (*model.Command, error) {
	return m.GetCommandByLibraryAndSlugFn(ctx, libraryID, slug)
}
func (m *mockLibraryStore) SoftDeleteCommand(ctx context.Context, id uuid.UUID) error {
	if m.SoftDeleteCommandFn != nil {
		return m.SoftDeleteCommandFn(ctx, id)
	}
	return nil
}
func (m *mockLibraryStore) CreateCommandForLibrary(ctx context.Context, ownerID, libraryID uuid.UUID, name, slug, description string, tags json.RawMessage) (*model.Command, error) {
	return m.CreateCommandForLibraryFn(ctx, ownerID, libraryID, name, slug, description, tags)
}
func (m *mockLibraryStore) UpdateCommandMeta(ctx context.Context, id uuid.UUID, name, description string, tags json.RawMessage) error {
	if m.UpdateCommandMetaFn != nil {
		return m.UpdateCommandMetaFn(ctx, id, name, description, tags)
	}
	return nil
}
func (m *mockLibraryStore) GetLatestHashByCommand(ctx context.Context, commandID uuid.UUID) (string, error) {
	return m.GetLatestHashByCommandFn(ctx, commandID)
}
func (m *mockLibraryStore) GetLatestVersionByCommand(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error) {
	return m.GetLatestVersionByCommandFn(ctx, commandID)
}
func (m *mockLibraryStore) CreateVersion(ctx context.Context, commandID uuid.UUID, version int, specJSON json.RawMessage, specHash, message string, createdBy uuid.UUID) (*model.CommandVersion, error) {
	return m.CreateVersionFn(ctx, commandID, version, specJSON, specHash, message, createdBy)
}
func (m *mockLibraryStore) ListVersionsByCommand(ctx context.Context, commandID uuid.UUID) ([]model.CommandVersion, error) {
	return m.ListVersionsByCommandFn(ctx, commandID)
}
func (m *mockLibraryStore) CreateOrUpdateLibrary(ctx context.Context, ownerID uuid.UUID, slug, name, description string, gitURL *string) (*model.Library, error) {
	return m.CreateOrUpdateLibraryFn(ctx, ownerID, slug, name, description, gitURL)
}
func (m *mockLibraryStore) LibraryReleaseExists(ctx context.Context, libraryID uuid.UUID, version string) (bool, error) {
	return m.LibraryReleaseExistsFn(ctx, libraryID, version)
}
func (m *mockLibraryStore) CreateLibraryRelease(ctx context.Context, libraryID uuid.UUID, version, tag, commitHash string, commandCount int, releasedBy uuid.UUID) (*model.LibraryRelease, error) {
	return m.CreateLibraryReleaseFn(ctx, libraryID, version, tag, commitHash, commandCount, releasedBy)
}
func (m *mockLibraryStore) UpdateLibraryLatestVersion(ctx context.Context, libraryID uuid.UUID, version string) error {
	return m.UpdateLibraryLatestVersionFn(ctx, libraryID, version)
}
func (m *mockLibraryStore) InstallLibrary(ctx context.Context, userID, libraryID uuid.UUID) error {
	return m.InstallLibraryFn(ctx, userID, libraryID)
}
func (m *mockLibraryStore) UninstallLibrary(ctx context.Context, userID, libraryID uuid.UUID) error {
	return m.UninstallLibraryFn(ctx, userID, libraryID)
}
func (m *mockLibraryStore) ListLibraryReleases(ctx context.Context, libraryID uuid.UUID) ([]model.LibraryRelease, error) {
	return m.ListLibraryReleasesFn(ctx, libraryID)
}
func (m *mockLibraryStore) GetLibraryRelease(ctx context.Context, libraryID uuid.UUID, version string) (*model.LibraryRelease, error) {
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
				ownerID := testLibOwner
				ms.SearchPublicLibrariesFn = func(context.Context, string, int, int) ([]model.Library, int, error) {
					return []model.Library{
						{ID: testLib1, Slug: "kubernetes", Name: "Kubernetes", OwnerID: &ownerID},
					}, 1, nil
				}
				ms.GetOwnerNameFn = func(context.Context, uuid.UUID) (string, error) {
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

			req := requestWithUser("GET", "/libraries"+tt.query, nil, uuid.Nil)
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
				ownerID := testLibOwner
				ms.GetLibraryByOwnerUsernameAndSlugFn = func(context.Context, string, string) (*model.Library, error) {
					return &model.Library{ID: testLib1, Slug: "kubernetes", OwnerID: &ownerID}, nil
				}
				ms.ListCommandsByLibraryFn = func(context.Context, uuid.UUID) ([]store.LibraryCommand, error) {
					return []store.LibraryCommand{
						{CommandID: testCmdLib1, Slug: "deploy", UpdatedAt: now},
					}, nil
				}
				ms.GetOwnerNameFn = func(context.Context, uuid.UUID) (string, error) {
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
					return &model.Library{ID: testLib1, Slug: "kubernetes"}, nil
				}
				ms.ListCommandsByLibraryFn = func(context.Context, uuid.UUID) ([]store.LibraryCommand, error) {
					return nil, nil
				}
				ms.GetOwnerNameFn = func(context.Context, uuid.UUID) (string, error) {
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

			req := requestWithUser("GET", "/libraries/"+tt.owner+"/"+tt.slug, nil, testUser1)
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
					return &model.Library{ID: testLib1}, nil
				}
				ms.InstallLibraryFn = func(context.Context, uuid.UUID, uuid.UUID) error { return nil }
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

			req := requestWithUser("POST", "/libraries/"+tt.owner+"/"+tt.slug+"/install", nil, testUser1)
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
			return &model.Library{ID: testLib1}, nil
		},
		UninstallLibraryFn: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
	}
	h := NewLibraryHandler(&config.Config{}, ms)

	r := chi.NewRouter()
	r.Delete("/libraries/{owner}/{slug}/install", h.Uninstall)

	req := requestWithUser("DELETE", "/libraries/alice/kubernetes/install", nil, testUser1)
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
				ms.CreateOrUpdateLibraryFn = func(context.Context, uuid.UUID, string, string, string, *string) (*model.Library, error) {
					return &model.Library{ID: testLib1, Slug: "kubernetes"}, nil
				}
				ms.LibraryReleaseExistsFn = func(context.Context, uuid.UUID, string) (bool, error) {
					return false, nil
				}
				ms.GetCommandByLibraryAndSlugFn = func(context.Context, uuid.UUID, string) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateCommandForLibraryFn = func(_ context.Context, _, libID uuid.UUID, name, slug, desc string, tags json.RawMessage) (*model.Command, error) {
					return &model.Command{ID: testCmd1, Name: name, Slug: slug}, nil
				}
				ms.GetLatestHashByCommandFn = func(context.Context, uuid.UUID) (string, error) {
					return "", store.ErrNotFound
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, uuid.UUID) (*model.CommandVersion, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateVersionFn = func(_ context.Context, cmdID uuid.UUID, ver int, _ json.RawMessage, hash, _ string, _ uuid.UUID) (*model.CommandVersion, error) {
					return &model.CommandVersion{ID: testCV1, CommandID: cmdID, Version: ver, SpecHash: hash}, nil
				}
				ms.CreateLibraryReleaseFn = func(_ context.Context, libID uuid.UUID, version, tag, commit string, count int, by uuid.UUID) (*model.LibraryRelease, error) {
					return &model.LibraryRelease{
						ID: uuid.MustParse("00000000-0000-4000-8000-000000000060"), LibraryID: libID, Version: version, Tag: tag,
						CommitHash: commit, CommandCount: count, ReleasedBy: by,
						ReleasedAt: time.Now(),
					}, nil
				}
				ms.UpdateLibraryLatestVersionFn = func(context.Context, uuid.UUID, string) error {
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
				ms.CreateOrUpdateLibraryFn = func(context.Context, uuid.UUID, string, string, string, *string) (*model.Library, error) {
					return &model.Library{ID: testLib1}, nil
				}
				ms.LibraryReleaseExistsFn = func(context.Context, uuid.UUID, string) (bool, error) {
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

			req := requestWithUser("POST", "/libraries/"+tt.slug+"/releases", tt.body, testUser2)
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
					return &model.Library{ID: testLib1}, nil
				}
				ms.GetCommandByLibraryAndSlugFn = func(context.Context, uuid.UUID, string) (*model.Command, error) {
					return &model.Command{ID: testCmd1, Slug: "deploy"}, nil
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, uuid.UUID) (*model.CommandVersion, error) {
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
					return &model.Library{ID: testLib1}, nil
				}
				ms.GetCommandByLibraryAndSlugFn = func(context.Context, uuid.UUID, string) (*model.Command, error) {
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

			req := requestWithUser("GET", "/libraries/"+tt.owner+"/"+tt.slug+"/commands/"+tt.cmdSlug, nil, uuid.Nil)
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
	testLR1 := uuid.MustParse("00000000-0000-4000-8000-000000000060")
	testLR2 := uuid.MustParse("00000000-0000-4000-8000-000000000061")

	ms := &mockLibraryStore{
		GetLibraryByOwnerUsernameAndSlugFn: func(context.Context, string, string) (*model.Library, error) {
			return &model.Library{ID: testLib1}, nil
		},
		ListLibraryReleasesFn: func(context.Context, uuid.UUID) ([]model.LibraryRelease, error) {
			return []model.LibraryRelease{
				{ID: testLR1, Version: "1.0.0", Tag: "v1.0.0", ReleasedAt: time.Now()},
				{ID: testLR2, Version: "1.1.0", Tag: "v1.1.0", ReleasedAt: time.Now()},
			}, nil
		},
	}
	h := NewLibraryHandler(&config.Config{}, ms)

	r := chi.NewRouter()
	r.Get("/libraries/{owner}/{slug}/releases", h.ListReleases)

	req := requestWithUser("GET", "/libraries/alice/kubernetes/releases", nil, uuid.Nil)
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

	testAdmin := uuid.MustParse("00000000-0000-4000-8000-000000000070")
	testRegular := uuid.MustParse("00000000-0000-4000-8000-000000000071")

	t.Run("admin can release to system namespace", func(t *testing.T) {
		var capturedOwnerID uuid.UUID
		ms := &mockLibraryStore{
			GetUserByIDFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
				return &model.User{ID: id, Email: "admin@example.com"}, nil
			},
			CreateOrUpdateLibraryFn: func(_ context.Context, ownerID uuid.UUID, slug, name, desc string, gitURL *string) (*model.Library, error) {
				capturedOwnerID = ownerID
				return &model.Library{ID: testLib1, Slug: slug}, nil
			},
			LibraryReleaseExistsFn: func(context.Context, uuid.UUID, string) (bool, error) {
				return false, nil
			},
			GetCommandByLibraryAndSlugFn: func(context.Context, uuid.UUID, string) (*model.Command, error) {
				return nil, store.ErrNotFound
			},
			CreateCommandForLibraryFn: func(_ context.Context, _, libID uuid.UUID, name, slug, desc string, tags json.RawMessage) (*model.Command, error) {
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
			CreateLibraryReleaseFn: func(_ context.Context, libID uuid.UUID, version, tag, commit string, count int, by uuid.UUID) (*model.LibraryRelease, error) {
				return &model.LibraryRelease{
					ID: uuid.MustParse("00000000-0000-4000-8000-000000000060"), LibraryID: libID, Version: version, Tag: tag,
					CommitHash: commit, CommandCount: count, ReleasedBy: by,
					ReleasedAt: time.Now(),
				}, nil
			},
			UpdateLibraryLatestVersionFn: func(context.Context, uuid.UUID, string) error {
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
		req := requestWithUser("POST", "/libraries/kubernetes/releases", body, testAdmin)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if capturedOwnerID != systemUserID {
			t.Errorf("ownerID = %q, want %q", capturedOwnerID, systemUserID)
		}
	})

	t.Run("non-admin rejected for system namespace", func(t *testing.T) {
		ms := &mockLibraryStore{
			GetUserByIDFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
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
		req := requestWithUser("POST", "/libraries/kubernetes/releases", body, testRegular)
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

func TestLibraryHandler_CreateRelease_SoftDeletesStaleCommands(t *testing.T) {
	validSpec := json.RawMessage(`{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "deploy", "slug": "deploy"},
		"steps": [{"name": "run", "run": "echo hello"}]
	}`)

	staleCmd := uuid.MustParse("00000000-0000-4000-8000-000000000099")
	var softDeleted []uuid.UUID

	ms := &mockLibraryStore{
		CreateOrUpdateLibraryFn: func(context.Context, uuid.UUID, string, string, string, *string) (*model.Library, error) {
			return &model.Library{ID: testLib1, Slug: "kubernetes"}, nil
		},
		LibraryReleaseExistsFn: func(context.Context, uuid.UUID, string) (bool, error) {
			return false, nil
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
		// Return two commands: "deploy" (still in release) and "something" (stale)
		ListCommandsByLibraryFn: func(context.Context, uuid.UUID) ([]store.LibraryCommand, error) {
			return []store.LibraryCommand{
				{CommandID: testCmd1, Slug: "deploy", Name: "deploy"},
				{CommandID: staleCmd, Slug: "something", Name: "something"},
			}, nil
		},
		SoftDeleteCommandFn: func(_ context.Context, id uuid.UUID) error {
			softDeleted = append(softDeleted, id)
			return nil
		},
		CreateLibraryReleaseFn: func(_ context.Context, libID uuid.UUID, version, tag, commit string, count int, by uuid.UUID) (*model.LibraryRelease, error) {
			return &model.LibraryRelease{
				ID: uuid.MustParse("00000000-0000-4000-8000-000000000060"), LibraryID: libID, Version: version, Tag: tag,
				CommitHash: commit, CommandCount: count, ReleasedBy: by,
				ReleasedAt: time.Now(),
			}, nil
		},
		UpdateLibraryLatestVersionFn: func(context.Context, uuid.UUID, string) error {
			return nil
		},
	}

	h := NewLibraryHandler(&config.Config{}, ms)

	r := chi.NewRouter()
	r.Post("/libraries/{slug}/releases", h.CreateRelease)

	body := map[string]any{
		"tag":         "v2.0.0",
		"commit_hash": "def456",
		"name":        "Kubernetes",
		"commands":    []json.RawMessage{validSpec},
	}
	req := requestWithUser("POST", "/libraries/kubernetes/releases", body, testUser2)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	if len(softDeleted) != 1 {
		t.Fatalf("expected 1 soft-deleted command, got %d", len(softDeleted))
	}
	if softDeleted[0] != staleCmd {
		t.Errorf("soft-deleted command ID = %s, want %s", softDeleted[0], staleCmd)
	}
}
