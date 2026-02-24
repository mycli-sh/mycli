package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

type mockMeStore struct {
	GetUserByIDFn           func(ctx context.Context, id uuid.UUID) (*model.User, error)
	IsUsernameTakenFn       func(ctx context.Context, username string) (bool, error)
	SetUsernameFn           func(ctx context.Context, userID uuid.UUID, username string) error
	CountCommandsByOwnerFn  func(ctx context.Context, ownerID uuid.UUID) (int, error)
	GetInstalledLibrariesFn func(ctx context.Context, userID uuid.UUID) ([]model.Library, error)
	ListCommandsByLibraryFn func(ctx context.Context, libraryID uuid.UUID) ([]store.LibraryCommand, error)
	GetOwnerNameFn          func(ctx context.Context, ownerID uuid.UUID) (string, error)
}

func (m *mockMeStore) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return m.GetUserByIDFn(ctx, id)
}
func (m *mockMeStore) IsUsernameTaken(ctx context.Context, username string) (bool, error) {
	return m.IsUsernameTakenFn(ctx, username)
}
func (m *mockMeStore) SetUsername(ctx context.Context, userID uuid.UUID, username string) error {
	return m.SetUsernameFn(ctx, userID, username)
}
func (m *mockMeStore) CountCommandsByOwner(ctx context.Context, ownerID uuid.UUID) (int, error) {
	return m.CountCommandsByOwnerFn(ctx, ownerID)
}
func (m *mockMeStore) GetInstalledLibraries(ctx context.Context, userID uuid.UUID) ([]model.Library, error) {
	return m.GetInstalledLibrariesFn(ctx, userID)
}
func (m *mockMeStore) ListCommandsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]store.LibraryCommand, error) {
	return m.ListCommandsByLibraryFn(ctx, libraryID)
}
func (m *mockMeStore) GetOwnerName(ctx context.Context, ownerID uuid.UUID) (string, error) {
	return m.GetOwnerNameFn(ctx, ownerID)
}

func TestMeHandler_GetMe(t *testing.T) {
	username := "alice"
	tests := []struct {
		name       string
		setupStore func(*mockMeStore)
		wantCode   int
		wantUser   bool
	}{
		{
			name: "success with username",
			setupStore: func(ms *mockMeStore) {
				ms.GetUserByIDFn = func(_ context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{ID: id, Email: "alice@example.com", Username: &username}, nil
				}
			},
			wantCode: http.StatusOK,
			wantUser: true,
		},
		{
			name: "success without username",
			setupStore: func(ms *mockMeStore) {
				ms.GetUserByIDFn = func(_ context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{ID: id, Email: "alice@example.com"}, nil
				}
			},
			wantCode: http.StatusOK,
			wantUser: true,
		},
		{
			name: "not found",
			setupStore: func(ms *mockMeStore) {
				ms.GetUserByIDFn = func(context.Context, uuid.UUID) (*model.User, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockMeStore{}
			tt.setupStore(ms)
			h := NewMeHandler(ms)

			r := chi.NewRouter()
			r.Get("/me", h.GetMe)

			req := requestWithUser("GET", "/me", nil, testUser1)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantUser {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				if resp["email"] != "alice@example.com" {
					t.Errorf("got email %v, want alice@example.com", resp["email"])
				}
			}
		})
	}
}

func TestMeHandler_SetUsername(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		setupStore  func(*mockMeStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name: "success",
			body: map[string]string{"username": "alice"},
			setupStore: func(ms *mockMeStore) {
				ms.IsUsernameTakenFn = func(context.Context, string) (bool, error) { return false, nil }
				ms.SetUsernameFn = func(context.Context, uuid.UUID, string) error { return nil }
			},
			wantCode: http.StatusOK,
		},
		{
			name:        "too short",
			body:        map[string]string{"username": "ab"},
			setupStore:  func(ms *mockMeStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_USERNAME",
		},
		{
			name:        "invalid characters",
			body:        map[string]string{"username": "Alice_Name"},
			setupStore:  func(ms *mockMeStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_USERNAME",
		},
		{
			name:        "reserved name",
			body:        map[string]string{"username": "admin"},
			setupStore:  func(ms *mockMeStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_USERNAME",
		},
		{
			name: "already taken",
			body: map[string]string{"username": "alice"},
			setupStore: func(ms *mockMeStore) {
				ms.IsUsernameTakenFn = func(context.Context, string) (bool, error) { return true, nil }
			},
			wantCode:    http.StatusConflict,
			wantErrCode: "USERNAME_TAKEN",
		},
		{
			name: "already set",
			body: map[string]string{"username": "alice"},
			setupStore: func(ms *mockMeStore) {
				ms.IsUsernameTakenFn = func(context.Context, string) (bool, error) { return false, nil }
				ms.SetUsernameFn = func(context.Context, uuid.UUID, string) error { return store.ErrNotFound }
			},
			wantCode:    http.StatusConflict,
			wantErrCode: "USERNAME_ALREADY_SET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockMeStore{}
			tt.setupStore(ms)
			h := NewMeHandler(ms)

			r := chi.NewRouter()
			r.Put("/me/username", h.SetUsername)

			req := requestWithUser("PUT", "/me/username", tt.body, testUser1)
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

func TestMeHandler_CheckUsernameAvailable(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		setupStore func(*mockMeStore)
		wantAvail  bool
	}{
		{
			name:     "available",
			username: "alice",
			setupStore: func(ms *mockMeStore) {
				ms.IsUsernameTakenFn = func(context.Context, string) (bool, error) { return false, nil }
			},
			wantAvail: true,
		},
		{
			name:     "taken",
			username: "alice",
			setupStore: func(ms *mockMeStore) {
				ms.IsUsernameTakenFn = func(context.Context, string) (bool, error) { return true, nil }
			},
			wantAvail: false,
		},
		{
			name:       "invalid - too short",
			username:   "ab",
			setupStore: func(ms *mockMeStore) {},
			wantAvail:  false,
		},
		{
			name:       "reserved",
			username:   "admin",
			setupStore: func(ms *mockMeStore) {},
			wantAvail:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockMeStore{}
			tt.setupStore(ms)
			h := NewMeHandler(ms)

			r := chi.NewRouter()
			r.Get("/me/username/{username}", h.CheckUsernameAvailable)

			req := requestWithUser("GET", "/me/username/"+tt.username, nil, testUser1)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
			}

			var resp map[string]any
			decodeJSON(t, rec, &resp)
			avail, _ := resp["available"].(bool)
			if avail != tt.wantAvail {
				t.Errorf("got available=%v, want %v", avail, tt.wantAvail)
			}
		})
	}
}

func TestMeHandler_SyncSummary(t *testing.T) {
	ms := &mockMeStore{}
	ms.CountCommandsByOwnerFn = func(context.Context, uuid.UUID) (int, error) { return 5, nil }
	ms.GetInstalledLibrariesFn = func(context.Context, uuid.UUID) ([]model.Library, error) {
		return []model.Library{
			{ID: testLib1, Slug: "kubernetes", Name: "Kubernetes", OwnerID: &testLibOwner},
		}, nil
	}
	ms.ListCommandsByLibraryFn = func(context.Context, uuid.UUID) ([]store.LibraryCommand, error) {
		return []store.LibraryCommand{
			{CommandID: testCmd1, Slug: "deploy"},
			{CommandID: testCmd2, Slug: "rollback"},
		}, nil
	}
	ms.GetOwnerNameFn = func(context.Context, uuid.UUID) (string, error) { return "kube-author", nil }

	h := NewMeHandler(ms)

	r := chi.NewRouter()
	r.Get("/me/sync-summary", h.SyncSummary)

	req := requestWithUser("GET", "/me/sync-summary", nil, testUser1)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)

	userCmds := int(resp["user_commands_count"].(float64))
	if userCmds != 5 {
		t.Errorf("got user_commands_count=%d, want 5", userCmds)
	}
	totalCmds := int(resp["total_commands"].(float64))
	if totalCmds != 7 {
		t.Errorf("got total_commands=%d, want 7", totalCmds)
	}
}
