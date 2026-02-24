package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"mycli.sh/api/internal/authservice"
	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

func newTestWebAuthHandler(ms *mockAuthStore) *AuthHandler {
	cfg := &config.Config{
		JWTSecret:  "test-secret",
		BaseURL:    "http://localhost:8080",
		WebBaseURL: "http://localhost:5173",
	}
	authSvc := authservice.New(cfg.JWTSecret, ms)
	return NewAuthHandler(cfg, ms, &mockEmailSender{}, authSvc)
}

func TestWebAuth_Login(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		setupStore  func(*mockAuthStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name: "success",
			body: map[string]string{"email": "user@example.com"},
			setupStore: func(ms *mockAuthStore) {
				ms.CreateMagicLinkFn = func(context.Context, string, string, string, *string, time.Time) (*model.MagicLink, error) {
					return &model.MagicLink{ID: "ml_1"}, nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:        "empty email",
			body:        map[string]string{"email": ""},
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_EMAIL",
		},
		{
			name:        "invalid email",
			body:        map[string]string{"email": "not-an-email"},
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_EMAIL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{}
			tt.setupStore(ms)
			h := newTestWebAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/web/login", h.WebLogin)

			req := requestWithUser("POST", "/auth/web/login", tt.body, "")
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

func TestWebAuth_Verify(t *testing.T) {
	now := time.Now()
	magicToken := authservice.GenerateCode(32)
	tokenHash := authservice.HashToken(magicToken)

	tests := []struct {
		name        string
		body        any
		setupStore  func(*mockAuthStore)
		wantCode    int
		wantErrCode string
		wantTokens  bool
	}{
		{
			name: "success - existing user",
			body: map[string]string{"token": magicToken},
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					if hash == tokenHash {
						return &model.MagicLink{ID: "ml_1", Email: "user@example.com", ExpiresAt: now.Add(15 * time.Minute)}, nil
					}
					return nil, store.ErrNotFound
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, string) error { return nil }
				ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: "usr_1", Email: email}, nil
				}
				ms.CreateSessionFn = func(_ context.Context, userID, _, _, _, _, _ string, _ time.Time) (*model.Session, error) {
					return &model.Session{ID: "ses_1", UserID: userID, LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
				}
				ms.GetUserByIDFn = func(_ context.Context, id string) (*model.User, error) {
					return &model.User{ID: id, Email: "user@example.com"}, nil
				}
			},
			wantCode:   http.StatusOK,
			wantTokens: true,
		},
		{
			name: "success - new user",
			body: map[string]string{"token": magicToken},
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					if hash == tokenHash {
						return &model.MagicLink{ID: "ml_1", Email: "new@example.com", ExpiresAt: now.Add(15 * time.Minute)}, nil
					}
					return nil, store.ErrNotFound
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, string) error { return nil }
				ms.GetUserByEmailFn = func(context.Context, string) (*model.User, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateUserFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: "usr_new", Email: email}, nil
				}
				ms.CreateSessionFn = func(_ context.Context, userID, _, _, _, _, _ string, _ time.Time) (*model.Session, error) {
					return &model.Session{ID: "ses_1", UserID: userID, LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
				}
				ms.GetUserByIDFn = func(_ context.Context, id string) (*model.User, error) {
					return &model.User{ID: id, Email: "new@example.com"}, nil
				}
			},
			wantCode:   http.StatusOK,
			wantTokens: true,
		},
		{
			name:        "missing token",
			body:        map[string]string{"token": ""},
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "MISSING_TOKEN",
		},
		{
			name: "invalid token",
			body: map[string]string{"token": "invalid-token"},
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_TOKEN",
		},
		{
			name: "expired token",
			body: map[string]string{"token": magicToken},
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					if hash == tokenHash {
						return &model.MagicLink{
							ID: "ml_1", Email: "user@example.com",
							ExpiresAt: now.Add(-1 * time.Minute), // expired
						}, nil
					}
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
		{
			name: "already used token",
			body: map[string]string{"token": magicToken},
			setupStore: func(ms *mockAuthStore) {
				usedAt := now
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					if hash == tokenHash {
						return &model.MagicLink{
							ID: "ml_1", Email: "user@example.com",
							ExpiresAt: now.Add(15 * time.Minute),
							UsedAt:    &usedAt,
						}, nil
					}
					return nil, store.ErrNotFound
				}
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{}
			tt.setupStore(ms)
			h := newTestWebAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/web/verify", h.WebVerify)

			req := requestWithUser("POST", "/auth/web/verify", tt.body, "")
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
			if tt.wantTokens {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				if resp["access_token"] == nil || resp["access_token"] == "" {
					t.Error("expected access_token")
				}
				if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
					t.Error("expected refresh_token")
				}
				if resp["session_id"] == nil || resp["session_id"] == "" {
					t.Error("expected session_id")
				}
			}
		})
	}
}
