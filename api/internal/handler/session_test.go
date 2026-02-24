package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

type mockSessionStore struct {
	ListSessionsByUserFn      func(ctx context.Context, userID uuid.UUID) ([]model.Session, error)
	RevokeSessionFn           func(ctx context.Context, id uuid.UUID) error
	RevokeAllSessionsExceptFn func(ctx context.Context, userID, exceptID uuid.UUID) (int64, error)
}

func (m *mockSessionStore) ListSessionsByUser(ctx context.Context, userID uuid.UUID) ([]model.Session, error) {
	return m.ListSessionsByUserFn(ctx, userID)
}
func (m *mockSessionStore) RevokeSession(ctx context.Context, id uuid.UUID) error {
	return m.RevokeSessionFn(ctx, id)
}
func (m *mockSessionStore) RevokeAllSessionsExcept(ctx context.Context, userID, exceptID uuid.UUID) (int64, error) {
	return m.RevokeAllSessionsExceptFn(ctx, userID, exceptID)
}

func TestSessionHandler_List(t *testing.T) {
	now := time.Now()
	ms := &mockSessionStore{
		ListSessionsByUserFn: func(_ context.Context, _ uuid.UUID) ([]model.Session, error) {
			return []model.Session{
				{ID: testSes1, UserAgent: "curl/8.0", IPAddress: "127.0.0.1", LastUsedAt: now, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now},
				{ID: testSes2, UserAgent: "my/1.0", IPAddress: "10.0.0.1", LastUsedAt: now, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now},
			}, nil
		},
	}
	h := NewSessionHandler(ms)

	r := chi.NewRouter()
	r.Get("/sessions", h.List)

	req := requestWithUser("GET", "/sessions", nil, testUser1)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	sessions := resp["sessions"].([]any)
	if len(sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(sessions))
	}
}

func TestSessionHandler_Revoke(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		sessionID   string
		setupStore  func(*mockSessionStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:      "success",
			sessionID: testSes1.String(),
			setupStore: func(ms *mockSessionStore) {
				ms.ListSessionsByUserFn = func(context.Context, uuid.UUID) ([]model.Session, error) {
					return []model.Session{
						{ID: testSes1, LastUsedAt: now, ExpiresAt: now, CreatedAt: now},
					}, nil
				}
				ms.RevokeSessionFn = func(context.Context, uuid.UUID) error { return nil }
			},
			wantCode: http.StatusOK,
		},
		{
			name:      "session not owned",
			sessionID: testSes2.String(),
			setupStore: func(ms *mockSessionStore) {
				ms.ListSessionsByUserFn = func(context.Context, uuid.UUID) ([]model.Session, error) {
					return []model.Session{
						{ID: testSes1, LastUsedAt: now, ExpiresAt: now, CreatedAt: now},
					}, nil
				}
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
		{
			name:      "revoke returns not found",
			sessionID: testSes1.String(),
			setupStore: func(ms *mockSessionStore) {
				ms.ListSessionsByUserFn = func(context.Context, uuid.UUID) ([]model.Session, error) {
					return []model.Session{
						{ID: testSes1, LastUsedAt: now, ExpiresAt: now, CreatedAt: now},
					}, nil
				}
				ms.RevokeSessionFn = func(context.Context, uuid.UUID) error { return store.ErrNotFound }
			},
			wantCode:    http.StatusNotFound,
			wantErrCode: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockSessionStore{}
			tt.setupStore(ms)
			h := NewSessionHandler(ms)

			r := chi.NewRouter()
			r.Delete("/sessions/{id}", h.Revoke)

			req := requestWithUser("DELETE", "/sessions/"+tt.sessionID, nil, testUser1)
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

func TestSessionHandler_RevokeAll(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		setupStore  func(*mockSessionStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name:  "success",
			query: "?current_session_id=" + testSesCurrent.String(),
			setupStore: func(ms *mockSessionStore) {
				ms.RevokeAllSessionsExceptFn = func(context.Context, uuid.UUID, uuid.UUID) (int64, error) {
					return 3, nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:        "missing param",
			query:       "",
			setupStore:  func(ms *mockSessionStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "MISSING_PARAM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockSessionStore{}
			tt.setupStore(ms)
			h := NewSessionHandler(ms)

			r := chi.NewRouter()
			r.Delete("/sessions", h.RevokeAll)

			req := requestWithUser("DELETE", "/sessions"+tt.query, nil, testUser1)
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
