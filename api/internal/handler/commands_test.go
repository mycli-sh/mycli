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
	"github.com/jackc/pgx/v5/pgconn"

	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

// mockCommandStore implements CommandStore for testing.
type mockCommandStore struct {
	CreateCommandFn             func(ctx context.Context, ownerID uuid.UUID, name, slug, description string, tags json.RawMessage) (*model.Command, error)
	GetCommandByIDFn            func(ctx context.Context, id uuid.UUID) (*model.Command, error)
	GetCommandByOwnerAndSlugFn  func(ctx context.Context, ownerID uuid.UUID, slug string) (*model.Command, error)
	ListCommandsByOwnerFn       func(ctx context.Context, ownerID uuid.UUID, cursor string, limit int, query string) ([]model.Command, string, error)
	SoftDeleteCommandFn         func(ctx context.Context, id uuid.UUID) error
	GetLatestVersionByCommandFn func(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error)
	GetLatestHashByCommandFn    func(ctx context.Context, commandID uuid.UUID) (string, error)
	CreateVersionFn             func(ctx context.Context, commandID uuid.UUID, version int, specJSON json.RawMessage, specHash, message string, createdBy uuid.UUID) (*model.CommandVersion, error)
	GetVersionByCommandAndVerFn func(ctx context.Context, commandID uuid.UUID, version int) (*model.CommandVersion, error)
	IsLibraryInstalledFn        func(ctx context.Context, userID, libraryID uuid.UUID) bool
}

func (m *mockCommandStore) CreateCommand(ctx context.Context, ownerID uuid.UUID, name, slug, description string, tags json.RawMessage) (*model.Command, error) {
	return m.CreateCommandFn(ctx, ownerID, name, slug, description, tags)
}
func (m *mockCommandStore) GetCommandByID(ctx context.Context, id uuid.UUID) (*model.Command, error) {
	return m.GetCommandByIDFn(ctx, id)
}
func (m *mockCommandStore) GetCommandByOwnerAndSlug(ctx context.Context, ownerID uuid.UUID, slug string) (*model.Command, error) {
	return m.GetCommandByOwnerAndSlugFn(ctx, ownerID, slug)
}
func (m *mockCommandStore) ListCommandsByOwner(ctx context.Context, ownerID uuid.UUID, cursor string, limit int, query string) ([]model.Command, string, error) {
	return m.ListCommandsByOwnerFn(ctx, ownerID, cursor, limit, query)
}
func (m *mockCommandStore) SoftDeleteCommand(ctx context.Context, id uuid.UUID) error {
	return m.SoftDeleteCommandFn(ctx, id)
}
func (m *mockCommandStore) GetLatestVersionByCommand(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error) {
	return m.GetLatestVersionByCommandFn(ctx, commandID)
}
func (m *mockCommandStore) GetLatestHashByCommand(ctx context.Context, commandID uuid.UUID) (string, error) {
	return m.GetLatestHashByCommandFn(ctx, commandID)
}
func (m *mockCommandStore) CreateVersion(ctx context.Context, commandID uuid.UUID, version int, specJSON json.RawMessage, specHash, message string, createdBy uuid.UUID) (*model.CommandVersion, error) {
	return m.CreateVersionFn(ctx, commandID, version, specJSON, specHash, message, createdBy)
}
func (m *mockCommandStore) GetVersionByCommandAndVersion(ctx context.Context, commandID uuid.UUID, version int) (*model.CommandVersion, error) {
	return m.GetVersionByCommandAndVerFn(ctx, commandID, version)
}
func (m *mockCommandStore) IsLibraryInstalled(ctx context.Context, userID, libraryID uuid.UUID) bool {
	if m.IsLibraryInstalledFn != nil {
		return m.IsLibraryInstalledFn(ctx, userID, libraryID)
	}
	return false
}

func TestCommandHandler_Create(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		setupStore  func(*mockCommandStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name: "success",
			body: map[string]any{"name": "Deploy", "slug": "deploy", "description": "Deploy app"},
			setupStore: func(ms *mockCommandStore) {
				ms.CreateCommandFn = func(_ context.Context, ownerID uuid.UUID, name, slug, desc string, tags json.RawMessage) (*model.Command, error) {
					return &model.Command{
						ID:          testCmd123,
						OwnerUserID: ownerID,
						Name:        name,
						Slug:        slug,
						Description: desc,
						Tags:        tags,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					}, nil
				}
			},
			wantCode: http.StatusCreated,
		},
		{
			name:        "missing name",
			body:        map[string]any{"slug": "deploy"},
			setupStore:  func(ms *mockCommandStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:        "missing slug",
			body:        map[string]any{"name": "Deploy"},
			setupStore:  func(ms *mockCommandStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name:        "invalid slug",
			body:        map[string]any{"name": "Deploy", "slug": "UPPER"},
			setupStore:  func(ms *mockCommandStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_SLUG",
		},
		{
			name: "duplicate slug",
			body: map[string]any{"name": "Deploy", "slug": "deploy"},
			setupStore: func(ms *mockCommandStore) {
				ms.CreateCommandFn = func(context.Context, uuid.UUID, string, string, string, json.RawMessage) (*model.Command, error) {
					return nil, &pgconn.PgError{Code: "23505"}
				}
			},
			wantCode:    http.StatusConflict,
			wantErrCode: "SLUG_EXISTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockCommandStore{}
			tt.setupStore(ms)
			h := NewCommandHandler(ms)

			r := chi.NewRouter()
			r.Post("/commands", h.Create)

			req := requestWithUser("POST", "/commands", tt.body, testUser2)
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

func TestCommandHandler_List(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		setupStore func(*mockCommandStore)
		wantCode   int
		wantCount  int
	}{
		{
			name:  "list all",
			query: "",
			setupStore: func(ms *mockCommandStore) {
				ms.ListCommandsByOwnerFn = func(context.Context, uuid.UUID, string, int, string) ([]model.Command, string, error) {
					return []model.Command{
						{ID: testCmd1, Slug: "deploy"},
						{ID: testCmd2, Slug: "build"},
					}, "", nil
				}
			},
			wantCode:  http.StatusOK,
			wantCount: 2,
		},
		{
			name:  "slug lookup found",
			query: "?slug=deploy",
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByOwnerAndSlugFn = func(context.Context, uuid.UUID, string) (*model.Command, error) {
					return &model.Command{ID: testCmd1, Slug: "deploy"}, nil
				}
			},
			wantCode:  http.StatusOK,
			wantCount: 1,
		},
		{
			name:  "slug lookup not found",
			query: "?slug=nonexistent",
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByOwnerAndSlugFn = func(context.Context, uuid.UUID, string) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:  http.StatusOK,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockCommandStore{}
			tt.setupStore(ms)
			h := NewCommandHandler(ms)

			r := chi.NewRouter()
			r.Get("/commands", h.List)

			req := requestWithUser("GET", "/commands"+tt.query, nil, testUser2)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantCode)
			}

			var resp map[string]any
			decodeJSON(t, rec, &resp)
			commands := resp["commands"].([]any)
			if len(commands) != tt.wantCount {
				t.Errorf("got %d commands, want %d", len(commands), tt.wantCount)
			}
		})
	}
}

func TestCommandHandler_Get(t *testing.T) {
	tests := []struct {
		name        string
		cmdID       string
		setupStore  func(*mockCommandStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:  "success",
			cmdID: testCmd123.String(),
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUser2, Slug: "deploy", Name: "Deploy"}, nil
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, uuid.UUID) (*model.CommandVersion, error) {
					return &model.CommandVersion{Version: 3}, nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:  "not found",
			cmdID: testCmd1.String(),
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(context.Context, uuid.UUID) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:  "not owner",
			cmdID: testCmd123.String(),
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUserOther}, nil
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockCommandStore{}
			tt.setupStore(ms)
			h := NewCommandHandler(ms)

			r := chi.NewRouter()
			r.Get("/commands/{id}", h.Get)

			req := requestWithUser("GET", "/commands/"+tt.cmdID, nil, testUser2)
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

func TestCommandHandler_Delete(t *testing.T) {
	tests := []struct {
		name        string
		cmdID       string
		setupStore  func(*mockCommandStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:  "success",
			cmdID: testCmd123.String(),
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUser2}, nil
				}
				ms.SoftDeleteCommandFn = func(context.Context, uuid.UUID) error {
					return nil
				}
			},
			wantCode: http.StatusNoContent,
		},
		{
			name:  "not found",
			cmdID: testCmd1.String(),
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(context.Context, uuid.UUID) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:  "not owner",
			cmdID: testCmd123.String(),
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUserOther}, nil
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockCommandStore{}
			tt.setupStore(ms)
			h := NewCommandHandler(ms)

			r := chi.NewRouter()
			r.Delete("/commands/{id}", h.Delete)

			req := requestWithUser("DELETE", "/commands/"+tt.cmdID, nil, testUser2)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestCommandHandler_PublishVersion(t *testing.T) {
	validSpec := json.RawMessage(`{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "deploy", "slug": "deploy"},
		"steps": [{"name": "run", "run": "echo hello"}]
	}`)

	tests := []struct {
		name        string
		cmdID       string
		body        any
		setupStore  func(*mockCommandStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:  "success",
			cmdID: testCmd123.String(),
			body:  map[string]any{"spec_json": validSpec, "message": "initial"},
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUser2}, nil
				}
				ms.GetLatestHashByCommandFn = func(context.Context, uuid.UUID) (string, error) {
					return "", store.ErrNotFound
				}
				ms.GetLatestVersionByCommandFn = func(context.Context, uuid.UUID) (*model.CommandVersion, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateVersionFn = func(_ context.Context, cmdID uuid.UUID, ver int, _ json.RawMessage, hash, msg string, by uuid.UUID) (*model.CommandVersion, error) {
					return &model.CommandVersion{ID: testCV1, CommandID: cmdID, Version: ver, SpecHash: hash}, nil
				}
			},
			wantCode: http.StatusCreated,
		},
		{
			name:  "not found",
			cmdID: testCmd1.String(),
			body:  map[string]any{"spec_json": validSpec},
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(context.Context, uuid.UUID) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:  "not owner",
			cmdID: testCmd123.String(),
			body:  map[string]any{"spec_json": validSpec},
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUserOther}, nil
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:  "invalid spec",
			cmdID: testCmd123.String(),
			body:  map[string]any{"spec_json": json.RawMessage(`{"invalid": true}`)},
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUser2}, nil
				}
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_SPEC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockCommandStore{}
			tt.setupStore(ms)
			h := NewCommandHandler(ms)

			r := chi.NewRouter()
			r.Post("/commands/{id}/versions", h.PublishVersion)

			req := requestWithUser("POST", "/commands/"+tt.cmdID+"/versions", tt.body, testUser2)
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

func TestCommandHandler_GetVersion(t *testing.T) {
	tests := []struct {
		name        string
		cmdID       string
		version     string
		setupStore  func(*mockCommandStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:    "success",
			cmdID:   testCmd123.String(),
			version: "1",
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUser2}, nil
				}
				ms.GetVersionByCommandAndVerFn = func(_ context.Context, cmdID uuid.UUID, ver int) (*model.CommandVersion, error) {
					return &model.CommandVersion{ID: testCV1, CommandID: cmdID, Version: ver, SpecHash: "abc123"}, nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:    "command not found",
			cmdID:   testCmd1.String(),
			version: "1",
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(context.Context, uuid.UUID) (*model.Command, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:    "version not found",
			cmdID:   testCmd123.String(),
			version: "99",
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUser2}, nil
				}
				ms.GetVersionByCommandAndVerFn = func(context.Context, uuid.UUID, int) (*model.CommandVersion, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:    "invalid version number",
			cmdID:   testCmd123.String(),
			version: "abc",
			setupStore: func(ms *mockCommandStore) {
				ms.GetCommandByIDFn = func(_ context.Context, id uuid.UUID) (*model.Command, error) {
					return &model.Command{ID: id, OwnerUserID: testUser2}, nil
				}
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockCommandStore{}
			tt.setupStore(ms)
			h := NewCommandHandler(ms)

			r := chi.NewRouter()
			r.Get("/commands/{id}/versions/{version}", h.GetVersion)

			req := requestWithUser("GET", "/commands/"+tt.cmdID+"/versions/"+tt.version, nil, testUser2)
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
