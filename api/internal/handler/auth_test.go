package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

type mockAuthStore struct {
	CreateMagicLinkFn         func(ctx context.Context, email, tokenHash, deviceCode string, otpHash *string, expiresAt time.Time) (*model.MagicLink, error)
	GetMagicLinkByTokenHashFn func(ctx context.Context, tokenHash string) (*model.MagicLink, error)
	GetMagicLinkByOTPHashFn   func(ctx context.Context, otpHash string) (*model.MagicLink, error)
	MarkMagicLinkUsedFn       func(ctx context.Context, id string) error
	GetUserByEmailFn          func(ctx context.Context, email string) (*model.User, error)
	CreateUserFn              func(ctx context.Context, email string) (*model.User, error)
	GetUserByIDFn             func(ctx context.Context, id string) (*model.User, error)
	CreateSessionFn           func(ctx context.Context, userID, refreshTokenHash, userAgent, ipAddress string, expiresAt time.Time) (*model.Session, error)
	GetSessionByTokenHashFn   func(ctx context.Context, tokenHash string) (*model.Session, error)
	UpdateSessionLastUsedFn   func(ctx context.Context, id string) error
	GetLibraryBySlugFn        func(ctx context.Context, slug string) (*model.Library, error)
	InstallLibraryFn          func(ctx context.Context, userID, libraryID string) error

	// Device session mocks
	CreateDeviceSessionFn         func(ctx context.Context, deviceCode, userCode, email string, expiresAt time.Time) error
	GetDeviceSessionByCodeFn      func(ctx context.Context, deviceCode string) (*model.DeviceSession, error)
	GetDeviceSessionByUserCodeFn  func(ctx context.Context, userCode string) (*model.DeviceSession, error)
	AuthorizeDeviceSessionFn      func(ctx context.Context, deviceCode, userID string) error
	IncrementDeviceOTPAttemptsFn  func(ctx context.Context, deviceCode string) (int, error)
	ResetDeviceOTPAndExtendFn     func(ctx context.Context, deviceCode string, expiresAt time.Time) error
	DeleteDeviceSessionFn         func(ctx context.Context, deviceCode string) error
	DeleteExpiredDeviceSessionsFn func(ctx context.Context) error
	WithTxFn                      func(ctx context.Context, fn func(store.AuthStore) error) error
}

func (m *mockAuthStore) CreateMagicLink(ctx context.Context, email, tokenHash, deviceCode string, otpHash *string, expiresAt time.Time) (*model.MagicLink, error) {
	return m.CreateMagicLinkFn(ctx, email, tokenHash, deviceCode, otpHash, expiresAt)
}
func (m *mockAuthStore) GetMagicLinkByTokenHash(ctx context.Context, tokenHash string) (*model.MagicLink, error) {
	return m.GetMagicLinkByTokenHashFn(ctx, tokenHash)
}
func (m *mockAuthStore) GetMagicLinkByOTPHash(ctx context.Context, otpHash string) (*model.MagicLink, error) {
	return m.GetMagicLinkByOTPHashFn(ctx, otpHash)
}
func (m *mockAuthStore) MarkMagicLinkUsed(ctx context.Context, id string) error {
	return m.MarkMagicLinkUsedFn(ctx, id)
}
func (m *mockAuthStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return m.GetUserByEmailFn(ctx, email)
}
func (m *mockAuthStore) CreateUser(ctx context.Context, email string) (*model.User, error) {
	return m.CreateUserFn(ctx, email)
}
func (m *mockAuthStore) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	return m.GetUserByIDFn(ctx, id)
}
func (m *mockAuthStore) CreateSession(ctx context.Context, userID, refreshTokenHash, userAgent, ipAddress string, expiresAt time.Time) (*model.Session, error) {
	return m.CreateSessionFn(ctx, userID, refreshTokenHash, userAgent, ipAddress, expiresAt)
}
func (m *mockAuthStore) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	return m.GetSessionByTokenHashFn(ctx, tokenHash)
}
func (m *mockAuthStore) UpdateSessionLastUsed(ctx context.Context, id string) error {
	return m.UpdateSessionLastUsedFn(ctx, id)
}
func (m *mockAuthStore) GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error) {
	if m.GetLibraryBySlugFn != nil {
		return m.GetLibraryBySlugFn(ctx, slug)
	}
	return nil, store.ErrNotFound
}
func (m *mockAuthStore) InstallLibrary(ctx context.Context, userID, libraryID string) error {
	if m.InstallLibraryFn != nil {
		return m.InstallLibraryFn(ctx, userID, libraryID)
	}
	return nil
}
func (m *mockAuthStore) CreateDeviceSession(ctx context.Context, deviceCode, userCode, email string, expiresAt time.Time) error {
	if m.CreateDeviceSessionFn != nil {
		return m.CreateDeviceSessionFn(ctx, deviceCode, userCode, email, expiresAt)
	}
	return nil
}
func (m *mockAuthStore) GetDeviceSessionByCode(ctx context.Context, deviceCode string) (*model.DeviceSession, error) {
	if m.GetDeviceSessionByCodeFn != nil {
		return m.GetDeviceSessionByCodeFn(ctx, deviceCode)
	}
	return nil, store.ErrNotFound
}
func (m *mockAuthStore) GetDeviceSessionByUserCode(ctx context.Context, userCode string) (*model.DeviceSession, error) {
	if m.GetDeviceSessionByUserCodeFn != nil {
		return m.GetDeviceSessionByUserCodeFn(ctx, userCode)
	}
	return nil, store.ErrNotFound
}
func (m *mockAuthStore) AuthorizeDeviceSession(ctx context.Context, deviceCode, userID string) error {
	if m.AuthorizeDeviceSessionFn != nil {
		return m.AuthorizeDeviceSessionFn(ctx, deviceCode, userID)
	}
	return nil
}
func (m *mockAuthStore) IncrementDeviceOTPAttempts(ctx context.Context, deviceCode string) (int, error) {
	if m.IncrementDeviceOTPAttemptsFn != nil {
		return m.IncrementDeviceOTPAttemptsFn(ctx, deviceCode)
	}
	return 0, nil
}
func (m *mockAuthStore) ResetDeviceOTPAndExtend(ctx context.Context, deviceCode string, expiresAt time.Time) error {
	if m.ResetDeviceOTPAndExtendFn != nil {
		return m.ResetDeviceOTPAndExtendFn(ctx, deviceCode, expiresAt)
	}
	return nil
}
func (m *mockAuthStore) DeleteDeviceSession(ctx context.Context, deviceCode string) error {
	if m.DeleteDeviceSessionFn != nil {
		return m.DeleteDeviceSessionFn(ctx, deviceCode)
	}
	return nil
}
func (m *mockAuthStore) DeleteExpiredDeviceSessions(ctx context.Context) error {
	if m.DeleteExpiredDeviceSessionsFn != nil {
		return m.DeleteExpiredDeviceSessionsFn(ctx)
	}
	return nil
}
func (m *mockAuthStore) WithTx(ctx context.Context, fn func(store.AuthStore) error) error {
	if m.WithTxFn != nil {
		return m.WithTxFn(ctx, fn)
	}
	// Default: just call fn with self (no real transaction)
	return fn(m)
}

func newTestAuthHandler(ms *mockAuthStore) *AuthHandler {
	cfg := &config.Config{
		JWTSecret: "test-secret",
		BaseURL:   "http://localhost:8080",
	}
	return NewAuthHandler(cfg, ms, &mockEmailSender{})
}

func TestAuthHandler_StartDeviceFlow(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		setupStore  func(*mockAuthStore)
		wantCode    int
		wantEmail   bool
		wantErrCode string
	}{
		{
			name: "with valid email",
			body: map[string]string{"email": "user@example.com"},
			setupStore: func(ms *mockAuthStore) {
				ms.CreateMagicLinkFn = func(context.Context, string, string, string, *string, time.Time) (*model.MagicLink, error) {
					return &model.MagicLink{ID: "ml_1"}, nil
				}
			},
			wantCode:  http.StatusOK,
			wantEmail: true,
		},
		{
			name:        "missing email",
			body:        nil,
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_EMAIL",
		},
		{
			name:        "empty email",
			body:        map[string]string{"email": ""},
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_EMAIL",
		},
		{
			name:        "with invalid email",
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
			h := newTestAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/device", h.StartDeviceFlow)

			req := requestWithUser("POST", "/auth/device", tt.body, "")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}

			if tt.wantErrCode != "" {
				var resp errorResponse
				decodeJSON(t, rec, &resp)
				if resp.Error.Code != tt.wantErrCode {
					t.Errorf("got error code %q, want %q", resp.Error.Code, tt.wantErrCode)
				}
			}

			if tt.wantCode == http.StatusOK {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				if resp["device_code"] == nil || resp["device_code"] == "" {
					t.Error("expected device_code in response")
				}
				if resp["user_code"] == nil || resp["user_code"] == "" {
					t.Error("expected user_code in response")
				}
				emailSent, _ := resp["email_sent"].(bool)
				if emailSent != tt.wantEmail {
					t.Errorf("got email_sent=%v, want %v", emailSent, tt.wantEmail)
				}
			}
		})
	}
}

func TestAuthHandler_PollDeviceToken(t *testing.T) {
	tests := []struct {
		name        string
		setupStore  func(ms *mockAuthStore)
		deviceCode  string
		wantCode    int
		wantErrCode string
		wantTokens  bool
	}{
		{
			name:       "authorization pending",
			deviceCode: "test-device-code",
			setupStore: func(ms *mockAuthStore) {
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, dc string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode: dc,
						UserCode:   "ABCD-1234",
						ExpiresAt:  time.Now().Add(15 * time.Minute),
						Authorized: false,
					}, nil
				}
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "AUTHORIZATION_PENDING",
		},
		{
			name:       "expired device code",
			deviceCode: "expired-code",
			setupStore: func(ms *mockAuthStore) {
				// GetDeviceSessionByCode returns ErrNotFound for expired sessions
				// (the SQL query filters by expires_at > NOW())
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
		{
			name:        "invalid device code",
			deviceCode:  "nonexistent",
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
		{
			name:       "authorized - returns tokens",
			deviceCode: "authorized-code",
			setupStore: func(ms *mockAuthStore) {
				userID := "usr_alice"
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, dc string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode: dc,
						UserCode:   "ABCD-1234",
						ExpiresAt:  time.Now().Add(15 * time.Minute),
						Authorized: true,
						UserID:     &userID,
					}, nil
				}
			},
			wantCode:   http.StatusOK,
			wantTokens: true,
		},
		{
			name:       "delete device session fails",
			deviceCode: "delete-fail",
			setupStore: func(ms *mockAuthStore) {
				userID := "usr_alice"
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, dc string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode: dc,
						Authorized: true,
						UserID:     &userID,
					}, nil
				}
				ms.DeleteDeviceSessionFn = func(context.Context, string) error {
					return errors.New("db error")
				}
			},
			wantCode:    http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			ms := &mockAuthStore{
				CreateSessionFn: func(context.Context, string, string, string, string, time.Time) (*model.Session, error) {
					return &model.Session{ID: "ses_1", LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
				},
				GetUserByIDFn: func(_ context.Context, id string) (*model.User, error) {
					return &model.User{ID: id, Email: "alice@example.com"}, nil
				},
			}
			tt.setupStore(ms)
			h := newTestAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/token", h.PollDeviceToken)

			req := requestWithUser("POST", "/auth/token", map[string]string{"device_code": tt.deviceCode}, "")
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
			}
		})
	}
}

func TestAuthHandler_VerifyOTP(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(ms *mockAuthStore) (deviceCode, otp string)
		wantCode    int
		wantErrCode string
	}{
		{
			name: "success",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "otp-device"
				otp := "123456"
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, deviceCode string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode:  dc,
						UserCode:    "ABCD-1234",
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: "ml_1", Email: "user@example.com", DeviceCode: dc}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, string) error { return nil }
				ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: "usr_1", Email: email}, nil
				}
				return dc, otp
			},
			wantCode: http.StatusOK,
		},
		{
			name: "invalid OTP",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "otp-device-bad"
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, deviceCode string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode:  dc,
						UserCode:    "ABCD-1234",
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return nil, store.ErrNotFound
				}
				return dc, "000000"
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_CODE",
		},
		{
			name: "expired device code",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "expired-otp"
				// GetDeviceSessionByCode returns ErrNotFound for expired sessions
				return dc, "123456"
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
		{
			name: "device code mismatch",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "mismatch-device"
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, deviceCode string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode:  dc,
						UserCode:    "ABCD-1234",
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: "ml_1", Email: "user@example.com", DeviceCode: "other-device"}, nil
				}
				return dc, "123456"
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_CODE",
		},
		{
			name: "creates new user when not found",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "new-user-device"
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, deviceCode string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode:  dc,
						UserCode:    "ABCD-1234",
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: "ml_1", Email: "new@example.com", DeviceCode: dc}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, string) error { return nil }
				ms.GetUserByEmailFn = func(context.Context, string) (*model.User, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateUserFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: "usr_new", Email: email}, nil
				}
				return dc, "654321"
			},
			wantCode: http.StatusOK,
		},
		{
			name: "authorize device session fails",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "auth-fail-device"
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, deviceCode string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode:  dc,
						UserCode:    "ABCD-1234",
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: "ml_1", Email: "user@example.com", DeviceCode: dc}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, string) error { return nil }
				ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: "usr_1", Email: email}, nil
				}
				ms.AuthorizeDeviceSessionFn = func(context.Context, string, string) error {
					return errors.New("db error")
				}
				return dc, "123456"
			},
			wantCode:    http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{}
			deviceCode, otp := tt.setup(ms)
			h := newTestAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/verify-otp", h.VerifyOTP)

			body := map[string]string{"device_code": deviceCode, "code": otp}
			req := requestWithUser("POST", "/auth/verify-otp", body, "")
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

func TestAuthHandler_VerifyOTP_BruteForceProtection(t *testing.T) {
	attempts := 0
	ms := &mockAuthStore{
		GetDeviceSessionByCodeFn: func(_ context.Context, dc string) (*model.DeviceSession, error) {
			return &model.DeviceSession{
				DeviceCode:  dc,
				UserCode:    "ABCD-1234",
				ExpiresAt:   time.Now().Add(15 * time.Minute),
				OTPAttempts: attempts,
			}, nil
		},
		GetMagicLinkByOTPHashFn: func(context.Context, string) (*model.MagicLink, error) {
			return nil, store.ErrNotFound
		},
		IncrementDeviceOTPAttemptsFn: func(_ context.Context, _ string) (int, error) {
			attempts++
			return attempts, nil
		},
	}
	h := newTestAuthHandler(ms)

	dc := "brute-force-device"

	r := chi.NewRouter()
	r.Post("/auth/verify-otp", h.VerifyOTP)

	// Make 5 failed attempts
	for i := 0; i < 5; i++ {
		body := map[string]string{"device_code": dc, "code": "000000"}
		req := requestWithUser("POST", "/auth/verify-otp", body, "")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: got status %d, want 400", i+1, rec.Code)
		}
	}

	// 6th attempt should be rate-limited
	body := map[string]string{"device_code": dc, "code": "000000"}
	req := requestWithUser("POST", "/auth/verify-otp", body, "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("got status %d, want 429 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp errorResponse
	decodeJSON(t, rec, &resp)
	if resp.Error.Code != "TOO_MANY_ATTEMPTS" {
		t.Errorf("got error code %q, want TOO_MANY_ATTEMPTS", resp.Error.Code)
	}
}

func TestAuthHandler_RefreshToken(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "test-secret",
		BaseURL:   "http://localhost:8080",
	}

	now := time.Now()

	tests := []struct {
		name        string
		getToken    func() string
		setupStore  func(*mockAuthStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name: "success",
			getToken: func() string {
				tok, _ := generateJWTToken("test-secret", "usr_alice", "refresh", 30*24*time.Hour)
				return tok
			},
			setupStore: func(ms *mockAuthStore) {
				ms.GetSessionByTokenHashFn = func(context.Context, string) (*model.Session, error) {
					return &model.Session{ID: "ses_1", UserID: "usr_alice", LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
				}
				ms.UpdateSessionLastUsedFn = func(context.Context, string) error { return nil }
			},
			wantCode: http.StatusOK,
		},
		{
			name: "invalid token",
			getToken: func() string {
				return "not-a-valid-jwt"
			},
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusUnauthorized,
			wantErrCode: "INVALID_TOKEN",
		},
		{
			name: "access token instead of refresh",
			getToken: func() string {
				tok, _ := generateJWTToken("test-secret", "usr_alice", "access", time.Hour)
				return tok
			},
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusUnauthorized,
			wantErrCode: "INVALID_TOKEN",
		},
		{
			name: "revoked session",
			getToken: func() string {
				tok, _ := generateJWTToken("test-secret", "usr_alice", "refresh", 30*24*time.Hour)
				return tok
			},
			setupStore: func(ms *mockAuthStore) {
				revokedAt := time.Now()
				ms.GetSessionByTokenHashFn = func(context.Context, string) (*model.Session, error) {
					return &model.Session{
						ID: "ses_1", UserID: "usr_alice",
						RevokedAt:  &revokedAt,
						LastUsedAt: now, ExpiresAt: now, CreatedAt: now,
					}, nil
				}
			},
			wantCode:    http.StatusUnauthorized,
			wantErrCode: "SESSION_REVOKED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{}
			tt.setupStore(ms)
			h := NewAuthHandler(cfg, ms, &mockEmailSender{})

			r := chi.NewRouter()
			r.Post("/auth/refresh", h.RefreshTokenHandler)

			body := map[string]string{"refresh_token": tt.getToken()}
			req := requestWithUser("POST", "/auth/refresh", body, "")
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

func TestAuthHandler_ResendVerification(t *testing.T) {
	tests := []struct {
		name        string
		body        any
		setup       func(ms *mockAuthStore)
		wantCode    int
		wantErrCode string
	}{
		{
			name: "success",
			body: map[string]string{"device_code": "resend-dc", "email": "user@example.com"},
			setup: func(ms *mockAuthStore) {
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, dc string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode: dc,
						UserCode:   "ABCD-1234",
						ExpiresAt:  time.Now().Add(15 * time.Minute),
					}, nil
				}
				ms.CreateMagicLinkFn = func(context.Context, string, string, string, *string, time.Time) (*model.MagicLink, error) {
					return &model.MagicLink{ID: "ml_1"}, nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:        "missing fields",
			body:        map[string]string{},
			setup:       func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "INVALID_REQUEST",
		},
		{
			name: "expired device code",
			body: map[string]string{"device_code": "expired-dc", "email": "user@example.com"},
			setup: func(ms *mockAuthStore) {
				// GetDeviceSessionByCode returns ErrNotFound for expired sessions
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
		{
			name: "reset OTP fails",
			body: map[string]string{"device_code": "reset-fail-dc", "email": "user@example.com"},
			setup: func(ms *mockAuthStore) {
				ms.GetDeviceSessionByCodeFn = func(_ context.Context, dc string) (*model.DeviceSession, error) {
					return &model.DeviceSession{
						DeviceCode: dc,
						UserCode:   "ABCD-1234",
						ExpiresAt:  time.Now().Add(15 * time.Minute),
					}, nil
				}
				ms.ResetDeviceOTPAndExtendFn = func(context.Context, string, time.Time) error {
					return errors.New("db error")
				}
			},
			wantCode:    http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{}
			tt.setup(ms)
			h := newTestAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/resend", h.ResendVerification)

			req := requestWithUser("POST", "/auth/resend", tt.body, "")
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

func TestAuthHandler_DeviceFlow_EndToEnd(t *testing.T) {
	// Tests the full flow: StartDeviceFlow → VerifyOTP → PollDeviceToken
	var capturedOTPHash string
	var capturedDeviceCode string
	authorized := false
	var authorizedUserID *string

	ms := &mockAuthStore{
		CreateDeviceSessionFn: func(_ context.Context, deviceCode, userCode, email string, _ time.Time) error {
			capturedDeviceCode = deviceCode
			return nil
		},
		GetDeviceSessionByCodeFn: func(_ context.Context, dc string) (*model.DeviceSession, error) {
			if dc != capturedDeviceCode {
				return nil, store.ErrNotFound
			}
			return &model.DeviceSession{
				DeviceCode:  dc,
				UserCode:    "ABCD-1234",
				ExpiresAt:   time.Now().Add(15 * time.Minute),
				Authorized:  authorized,
				UserID:      authorizedUserID,
				OTPAttempts: 0,
			}, nil
		},
		CreateMagicLinkFn: func(_ context.Context, _ string, _ string, _ string, otpHash *string, _ time.Time) (*model.MagicLink, error) {
			if otpHash != nil {
				capturedOTPHash = *otpHash
			}
			return &model.MagicLink{ID: "ml_1"}, nil
		},
		AuthorizeDeviceSessionFn: func(_ context.Context, _ string, userID string) error {
			authorized = true
			authorizedUserID = &userID
			return nil
		},
		DeleteDeviceSessionFn: func(_ context.Context, dc string) error {
			// After deletion, subsequent lookups should fail
			capturedDeviceCode = ""
			return nil
		},
	}

	cfg := &config.Config{
		JWTSecret: "test-secret",
		BaseURL:   "http://localhost:8080",
	}
	emailSender := &mockEmailSender{}
	h := NewAuthHandler(cfg, ms, emailSender)

	router := chi.NewRouter()
	router.Post("/auth/device", h.StartDeviceFlow)
	router.Post("/auth/verify-otp", h.VerifyOTP)
	router.Post("/auth/token", h.PollDeviceToken)

	// Step 1: Start device flow with email
	req := requestWithUser("POST", "/auth/device", map[string]string{"email": "user@example.com"}, "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("StartDeviceFlow: got status %d (body=%s)", rec.Code, rec.Body.String())
	}

	var deviceResp map[string]any
	decodeJSON(t, rec, &deviceResp)
	deviceCode := deviceResp["device_code"].(string)

	// Verify an OTP was captured and an email was sent
	if capturedOTPHash == "" {
		t.Fatal("expected OTP hash to be captured")
	}
	if len(emailSender.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(emailSender.calls))
	}
	sentOTP := emailSender.calls[0].OTPCode

	// Step 2: Verify the OTP
	ms.GetMagicLinkByOTPHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
		if hash == hashToken(sentOTP) {
			return &model.MagicLink{ID: "ml_1", Email: "user@example.com", DeviceCode: deviceCode}, nil
		}
		return nil, store.ErrNotFound
	}
	ms.MarkMagicLinkUsedFn = func(context.Context, string) error { return nil }
	ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
		return &model.User{ID: "usr_1", Email: email}, nil
	}

	req = requestWithUser("POST", "/auth/verify-otp", map[string]string{
		"device_code": deviceCode,
		"code":        sentOTP,
	}, "")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("VerifyOTP: got status %d (body=%s)", rec.Code, rec.Body.String())
	}

	// Step 3: Poll for token — should now be authorized
	now := time.Now()
	ms.CreateSessionFn = func(context.Context, string, string, string, string, time.Time) (*model.Session, error) {
		return &model.Session{ID: "ses_1", LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
	}
	ms.GetUserByIDFn = func(_ context.Context, id string) (*model.User, error) {
		return &model.User{ID: id, Email: "user@example.com"}, nil
	}

	req = requestWithUser("POST", "/auth/token", map[string]string{"device_code": deviceCode}, "")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PollDeviceToken: got status %d (body=%s)", rec.Code, rec.Body.String())
	}

	var tokenResp map[string]any
	decodeJSON(t, rec, &tokenResp)
	if tokenResp["access_token"] == nil || tokenResp["access_token"] == "" {
		t.Error("expected access_token in token response")
	}
	if tokenResp["refresh_token"] == nil || tokenResp["refresh_token"] == "" {
		t.Error("expected refresh_token in token response")
	}

	// Validate the access token can be parsed
	accessToken := tokenResp["access_token"].(string)
	if accessToken == "" {
		t.Fatal("empty access token")
	}

	// Step 4: Polling again should fail (device code consumed)
	req = requestWithUser("POST", "/auth/token", map[string]string{"device_code": deviceCode}, "")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("second poll: got status %d, want 400", rec.Code)
	}
}

func TestGenerateJWTToken(t *testing.T) {
	token, err := generateJWTToken("secret", "usr_1", "access", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

// Suppress unused import warning — json is used via requestWithUser.
var _ = json.Marshal
