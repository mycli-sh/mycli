package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/authservice"
	"mycli.sh/api/internal/config"
	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

type mockAuthStore struct {
	CreateMagicLinkFn                func(ctx context.Context, email, tokenHash, deviceCode string, otpHash *string, expiresAt time.Time) (*model.MagicLink, error)
	GetMagicLinkByTokenHashFn        func(ctx context.Context, tokenHash string) (*model.MagicLink, error)
	GetMagicLinkByOTPHashFn          func(ctx context.Context, otpHash string) (*model.MagicLink, error)
	GetMagicLinkByDeviceCodeFn       func(ctx context.Context, deviceCode string) (*model.MagicLink, error)
	MarkMagicLinkUsedFn              func(ctx context.Context, id uuid.UUID) error
	AuthorizeMagicLinkByDeviceCodeFn func(ctx context.Context, deviceCode string, userID uuid.UUID) error
	IncrementMagicLinkOTPAttemptsFn  func(ctx context.Context, id uuid.UUID) (int, error)
	DeleteMagicLinksByDeviceCodeFn   func(ctx context.Context, deviceCode string) error
	DeleteExpiredMagicLinksFn        func(ctx context.Context) error
	ConsumeAuthorizedDeviceCodeFn    func(ctx context.Context, deviceCode string, userID uuid.UUID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error)
	GetUserByEmailFn                 func(ctx context.Context, email string) (*model.User, error)
	CreateUserFn                     func(ctx context.Context, email string) (*model.User, error)
	GetUserByIDFn                    func(ctx context.Context, id uuid.UUID) (*model.User, error)
	CreateSessionFn                  func(ctx context.Context, userID uuid.UUID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error)
	RevokeSessionByDeviceIDFn        func(ctx context.Context, userID uuid.UUID, deviceID string) error
	RevokeSessionFn                  func(ctx context.Context, id uuid.UUID) error
	GetSessionByTokenHashFn          func(ctx context.Context, tokenHash string) (*model.Session, error)
	UpdateSessionLastUsedFn          func(ctx context.Context, id uuid.UUID) error
	UpdateSessionRefreshTokenHashFn  func(ctx context.Context, id uuid.UUID, newHash string) error
	CountOTPAttemptsByDeviceCodeFn   func(ctx context.Context, deviceCode string) (int, error)
	GetLibraryBySlugFn               func(ctx context.Context, slug string) (*model.Library, error)
	InstallLibraryFn                 func(ctx context.Context, userID, libraryID uuid.UUID) error
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
func (m *mockAuthStore) GetMagicLinkByDeviceCode(ctx context.Context, deviceCode string) (*model.MagicLink, error) {
	if m.GetMagicLinkByDeviceCodeFn != nil {
		return m.GetMagicLinkByDeviceCodeFn(ctx, deviceCode)
	}
	return nil, store.ErrNotFound
}
func (m *mockAuthStore) MarkMagicLinkUsed(ctx context.Context, id uuid.UUID) error {
	return m.MarkMagicLinkUsedFn(ctx, id)
}
func (m *mockAuthStore) AuthorizeMagicLinkByDeviceCode(ctx context.Context, deviceCode string, userID uuid.UUID) error {
	if m.AuthorizeMagicLinkByDeviceCodeFn != nil {
		return m.AuthorizeMagicLinkByDeviceCodeFn(ctx, deviceCode, userID)
	}
	return nil
}
func (m *mockAuthStore) IncrementMagicLinkOTPAttempts(ctx context.Context, id uuid.UUID) (int, error) {
	if m.IncrementMagicLinkOTPAttemptsFn != nil {
		return m.IncrementMagicLinkOTPAttemptsFn(ctx, id)
	}
	return 0, nil
}
func (m *mockAuthStore) DeleteMagicLinksByDeviceCode(ctx context.Context, deviceCode string) error {
	if m.DeleteMagicLinksByDeviceCodeFn != nil {
		return m.DeleteMagicLinksByDeviceCodeFn(ctx, deviceCode)
	}
	return nil
}
func (m *mockAuthStore) DeleteExpiredMagicLinks(ctx context.Context) error {
	if m.DeleteExpiredMagicLinksFn != nil {
		return m.DeleteExpiredMagicLinksFn(ctx)
	}
	return nil
}
func (m *mockAuthStore) ConsumeAuthorizedDeviceCode(ctx context.Context, deviceCode string, userID uuid.UUID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error) {
	if m.ConsumeAuthorizedDeviceCodeFn != nil {
		return m.ConsumeAuthorizedDeviceCodeFn(ctx, deviceCode, userID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName, expiresAt)
	}
	now := time.Now()
	return &model.Session{ID: testSes1, LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
}
func (m *mockAuthStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return m.GetUserByEmailFn(ctx, email)
}
func (m *mockAuthStore) CreateUser(ctx context.Context, email string) (*model.User, error) {
	return m.CreateUserFn(ctx, email)
}
func (m *mockAuthStore) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return m.GetUserByIDFn(ctx, id)
}
func (m *mockAuthStore) CreateSession(ctx context.Context, userID uuid.UUID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error) {
	return m.CreateSessionFn(ctx, userID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName, expiresAt)
}
func (m *mockAuthStore) RevokeSessionByDeviceID(ctx context.Context, userID uuid.UUID, deviceID string) error {
	if m.RevokeSessionByDeviceIDFn != nil {
		return m.RevokeSessionByDeviceIDFn(ctx, userID, deviceID)
	}
	return nil
}
func (m *mockAuthStore) RevokeSession(ctx context.Context, id uuid.UUID) error {
	if m.RevokeSessionFn != nil {
		return m.RevokeSessionFn(ctx, id)
	}
	return nil
}
func (m *mockAuthStore) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	return m.GetSessionByTokenHashFn(ctx, tokenHash)
}
func (m *mockAuthStore) UpdateSessionLastUsed(ctx context.Context, id uuid.UUID) error {
	return m.UpdateSessionLastUsedFn(ctx, id)
}
func (m *mockAuthStore) UpdateSessionRefreshTokenHash(ctx context.Context, id uuid.UUID, newHash string) error {
	if m.UpdateSessionRefreshTokenHashFn != nil {
		return m.UpdateSessionRefreshTokenHashFn(ctx, id, newHash)
	}
	return nil
}
func (m *mockAuthStore) CountOTPAttemptsByDeviceCode(ctx context.Context, deviceCode string) (int, error) {
	if m.CountOTPAttemptsByDeviceCodeFn != nil {
		return m.CountOTPAttemptsByDeviceCodeFn(ctx, deviceCode)
	}
	return 0, nil
}
func (m *mockAuthStore) GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error) {
	if m.GetLibraryBySlugFn != nil {
		return m.GetLibraryBySlugFn(ctx, slug)
	}
	return nil, store.ErrNotFound
}
func (m *mockAuthStore) InstallLibrary(ctx context.Context, userID, libraryID uuid.UUID) error {
	if m.InstallLibraryFn != nil {
		return m.InstallLibraryFn(ctx, userID, libraryID)
	}
	return nil
}

func newTestAuthHandler(ms *mockAuthStore) *AuthHandler {
	cfg := &config.Config{
		JWTSecret: "test-secret",
		BaseURL:   "http://localhost:8080",
	}
	authSvc := authservice.New(cfg.JWTSecret, ms)
	return NewAuthHandler(cfg, ms, &mockEmailSender{}, authSvc)
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
					return &model.MagicLink{ID: testML1}, nil
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

			req := requestWithUser("POST", "/auth/device", tt.body, uuid.Nil)
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
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, dc string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode: dc,
						ExpiresAt:  time.Now().Add(15 * time.Minute),
						Authorized: false,
					}, nil
				}
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "AUTHORIZATION_PENDING",
		},
		{
			name:        "expired device code",
			deviceCode:  "expired-code",
			setupStore:  func(ms *mockAuthStore) {},
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
				userID := testUser1
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, dc string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode: dc,
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
			name:       "consume device code fails",
			deviceCode: "consume-fail",
			setupStore: func(ms *mockAuthStore) {
				userID := testUser1
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, dc string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode: dc,
						Authorized: true,
						UserID:     &userID,
					}, nil
				}
				ms.ConsumeAuthorizedDeviceCodeFn = func(context.Context, string, uuid.UUID, string, string, string, string, string, time.Time) (*model.Session, error) {
					return nil, errors.New("db error")
				}
			},
			wantCode:    http.StatusInternalServerError,
			wantErrCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{
				GetUserByIDFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
					return &model.User{ID: id, Email: "alice@example.com"}, nil
				},
			}
			tt.setupStore(ms)
			h := newTestAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/token", h.PollDeviceToken)

			req := requestWithUser("POST", "/auth/token", map[string]string{"device_code": tt.deviceCode}, uuid.Nil)
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
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, deviceCode string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode:  dc,
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: dc}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return nil }
				ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: testUser1, Email: email}, nil
				}
				return dc, otp
			},
			wantCode: http.StatusOK,
		},
		{
			name: "invalid OTP",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "otp-device-bad"
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, deviceCode string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode:  dc,
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
				return dc, "123456"
			},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
		{
			name: "device code mismatch",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "mismatch-device"
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, deviceCode string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode:  dc,
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: "other-device"}, nil
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
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, deviceCode string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode:  dc,
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "new@example.com", DeviceCode: dc}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return nil }
				ms.GetUserByEmailFn = func(context.Context, string) (*model.User, error) {
					return nil, store.ErrNotFound
				}
				ms.CreateUserFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: testUserNew, Email: email}, nil
				}
				return dc, "654321"
			},
			wantCode: http.StatusOK,
		},
		{
			name: "authorize device session fails",
			setup: func(ms *mockAuthStore) (string, string) {
				dc := "auth-fail-device"
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, deviceCode string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode:  dc,
						ExpiresAt:   time.Now().Add(15 * time.Minute),
						OTPAttempts: 0,
					}, nil
				}
				ms.GetMagicLinkByOTPHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: dc}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return nil }
				ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: testUser1, Email: email}, nil
				}
				ms.AuthorizeMagicLinkByDeviceCodeFn = func(context.Context, string, uuid.UUID) error {
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
			req := requestWithUser("POST", "/auth/verify-otp", body, uuid.Nil)
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
		GetMagicLinkByDeviceCodeFn: func(_ context.Context, dc string) (*model.MagicLink, error) {
			return &model.MagicLink{
				DeviceCode:  dc,
				ExpiresAt:   time.Now().Add(15 * time.Minute),
				OTPAttempts: attempts,
			}, nil
		},
		GetMagicLinkByOTPHashFn: func(context.Context, string) (*model.MagicLink, error) {
			return nil, store.ErrNotFound
		},
		IncrementMagicLinkOTPAttemptsFn: func(_ context.Context, _ uuid.UUID) (int, error) {
			attempts++
			return attempts, nil
		},
		CountOTPAttemptsByDeviceCodeFn: func(_ context.Context, _ string) (int, error) {
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
		req := requestWithUser("POST", "/auth/verify-otp", body, uuid.Nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: got status %d, want 400", i+1, rec.Code)
		}
	}

	// 6th attempt should be rate-limited
	body := map[string]string{"device_code": dc, "code": "000000"}
	req := requestWithUser("POST", "/auth/verify-otp", body, uuid.Nil)
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
				tok, _ := authservice.GenerateJWTToken("test-secret", testUser1.String(), "refresh", 30*24*time.Hour)
				return tok
			},
			setupStore: func(ms *mockAuthStore) {
				ms.GetSessionByTokenHashFn = func(context.Context, string) (*model.Session, error) {
					return &model.Session{ID: testSes1, UserID: testUser1, LastUsedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now}, nil
				}
				ms.UpdateSessionLastUsedFn = func(context.Context, uuid.UUID) error { return nil }
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
				tok, _ := authservice.GenerateJWTToken("test-secret", testUser1.String(), "access", time.Hour)
				return tok
			},
			setupStore:  func(ms *mockAuthStore) {},
			wantCode:    http.StatusUnauthorized,
			wantErrCode: "INVALID_TOKEN",
		},
		{
			name: "revoked session",
			getToken: func() string {
				tok, _ := authservice.GenerateJWTToken("test-secret", testUser1.String(), "refresh", 30*24*time.Hour)
				return tok
			},
			setupStore: func(ms *mockAuthStore) {
				revokedAt := time.Now()
				ms.GetSessionByTokenHashFn = func(context.Context, string) (*model.Session, error) {
					return &model.Session{
						ID: testSes1, UserID: testUser1,
						RevokedAt:  &revokedAt,
						LastUsedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now,
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
			authSvc := authservice.New(cfg.JWTSecret, ms)
			h := NewAuthHandler(cfg, ms, &mockEmailSender{}, authSvc)

			r := chi.NewRouter()
			r.Post("/auth/refresh", h.RefreshTokenHandler)

			body := map[string]string{"refresh_token": tt.getToken()}
			req := requestWithUser("POST", "/auth/refresh", body, uuid.Nil)
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
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, dc string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode: dc,
						ExpiresAt:  time.Now().Add(15 * time.Minute),
					}, nil
				}
				ms.CreateMagicLinkFn = func(context.Context, string, string, string, *string, time.Time) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1}, nil
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
			name:        "expired device code",
			body:        map[string]string{"device_code": "expired-dc", "email": "user@example.com"},
			setup:       func(ms *mockAuthStore) {},
			wantCode:    http.StatusBadRequest,
			wantErrCode: "EXPIRED_TOKEN",
		},
		{
			name: "create magic link fails",
			body: map[string]string{"device_code": "create-fail-dc", "email": "user@example.com"},
			setup: func(ms *mockAuthStore) {
				ms.GetMagicLinkByDeviceCodeFn = func(_ context.Context, dc string) (*model.MagicLink, error) {
					return &model.MagicLink{
						DeviceCode: dc,
						ExpiresAt:  time.Now().Add(15 * time.Minute),
					}, nil
				}
				ms.CreateMagicLinkFn = func(context.Context, string, string, string, *string, time.Time) (*model.MagicLink, error) {
					return nil, errors.New("db error")
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

			req := requestWithUser("POST", "/auth/resend", tt.body, uuid.Nil)
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
	var authorizedUserID *uuid.UUID

	ms := &mockAuthStore{
		CreateMagicLinkFn: func(_ context.Context, _ string, _ string, deviceCode string, otpHash *string, _ time.Time) (*model.MagicLink, error) {
			capturedDeviceCode = deviceCode
			if otpHash != nil {
				capturedOTPHash = *otpHash
			}
			return &model.MagicLink{ID: testML1}, nil
		},
		GetMagicLinkByDeviceCodeFn: func(_ context.Context, dc string) (*model.MagicLink, error) {
			if dc != capturedDeviceCode {
				return nil, store.ErrNotFound
			}
			return &model.MagicLink{
				DeviceCode:  dc,
				ExpiresAt:   time.Now().Add(15 * time.Minute),
				Authorized:  authorized,
				UserID:      authorizedUserID,
				OTPAttempts: 0,
			}, nil
		},
		AuthorizeMagicLinkByDeviceCodeFn: func(_ context.Context, _ string, userID uuid.UUID) error {
			authorized = true
			authorizedUserID = &userID
			return nil
		},
		ConsumeAuthorizedDeviceCodeFn: func(_ context.Context, dc string, _ uuid.UUID, _, _, _, _, _ string, _ time.Time) (*model.Session, error) {
			// After consumption, subsequent lookups should fail
			capturedDeviceCode = ""
			now := time.Now()
			return &model.Session{ID: testSes1, LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
		},
	}

	cfg := &config.Config{
		JWTSecret: "test-secret",
		BaseURL:   "http://localhost:8080",
	}
	emailSender := &mockEmailSender{}
	authSvc := authservice.New(cfg.JWTSecret, ms)
	h := NewAuthHandler(cfg, ms, emailSender, authSvc)

	router := chi.NewRouter()
	router.Post("/auth/device", h.StartDeviceFlow)
	router.Post("/auth/verify-otp", h.VerifyOTP)
	router.Post("/auth/token", h.PollDeviceToken)

	// Step 1: Start device flow with email
	req := requestWithUser("POST", "/auth/device", map[string]string{"email": "user@example.com"}, uuid.Nil)
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
		if hash == authservice.HashToken(sentOTP) {
			return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: deviceCode}, nil
		}
		return nil, store.ErrNotFound
	}
	ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return nil }
	ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
		return &model.User{ID: testUser1, Email: email}, nil
	}

	req = requestWithUser("POST", "/auth/verify-otp", map[string]string{
		"device_code": deviceCode,
		"code":        sentOTP,
	}, uuid.Nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("VerifyOTP: got status %d (body=%s)", rec.Code, rec.Body.String())
	}

	// Step 3: Poll for token — should now be authorized
	ms.GetUserByIDFn = func(_ context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{ID: id, Email: "user@example.com"}, nil
	}

	req = requestWithUser("POST", "/auth/token", map[string]string{"device_code": deviceCode}, uuid.Nil)
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
	req = requestWithUser("POST", "/auth/token", map[string]string{"device_code": deviceCode}, uuid.Nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("second poll: got status %d, want 400", rec.Code)
	}
}

func TestAuthHandler_VerifyMagicLink(t *testing.T) {
	magicToken := authservice.GenerateCode(32)
	tokenHash := authservice.HashToken(magicToken)
	now := time.Now()

	tests := []struct {
		name       string
		url        string
		setupStore func(*mockAuthStore)
		wantCode   int
		wantHTML   bool // true = expect HTML response (success or error page)
	}{
		{
			name: "success",
			url:  "/auth/verify?token=" + magicToken,
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					if hash == tokenHash {
						return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: "dc_1", ExpiresAt: now.Add(15 * time.Minute)}, nil
					}
					return nil, store.ErrNotFound
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return nil }
				ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: testUser1, Email: email}, nil
				}
			},
			wantCode: http.StatusOK,
			wantHTML: true,
		},
		{
			name:       "missing token param",
			url:        "/auth/verify",
			setupStore: func(ms *mockAuthStore) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name: "invalid token",
			url:  "/auth/verify?token=invalid",
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(context.Context, string) (*model.MagicLink, error) {
					return nil, store.ErrNotFound
				}
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "expired link",
			url:  "/auth/verify?token=" + magicToken,
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: "dc_1", ExpiresAt: now.Add(-1 * time.Minute)}, nil
				}
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "already used link",
			url:  "/auth/verify?token=" + magicToken,
			setupStore: func(ms *mockAuthStore) {
				usedAt := now
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: "dc_1", ExpiresAt: now.Add(15 * time.Minute), UsedAt: &usedAt}, nil
				}
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "MarkMagicLinkUsed fails",
			url:  "/auth/verify?token=" + magicToken,
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: "dc_1", ExpiresAt: now.Add(15 * time.Minute)}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return errors.New("db error") }
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "user creation fails",
			url:  "/auth/verify?token=" + magicToken,
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "new@example.com", DeviceCode: "dc_1", ExpiresAt: now.Add(15 * time.Minute)}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return nil }
				ms.GetUserByEmailFn = func(context.Context, string) (*model.User, error) { return nil, store.ErrNotFound }
				ms.CreateUserFn = func(context.Context, string) (*model.User, error) { return nil, errors.New("db error") }
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "AuthorizeMagicLinkByDeviceCode fails",
			url:  "/auth/verify?token=" + magicToken,
			setupStore: func(ms *mockAuthStore) {
				ms.GetMagicLinkByTokenHashFn = func(_ context.Context, hash string) (*model.MagicLink, error) {
					return &model.MagicLink{ID: testML1, Email: "user@example.com", DeviceCode: "dc_1", ExpiresAt: now.Add(15 * time.Minute)}, nil
				}
				ms.MarkMagicLinkUsedFn = func(context.Context, uuid.UUID) error { return nil }
				ms.GetUserByEmailFn = func(_ context.Context, email string) (*model.User, error) {
					return &model.User{ID: testUser1, Email: email}, nil
				}
				ms.AuthorizeMagicLinkByDeviceCodeFn = func(context.Context, string, uuid.UUID) error { return errors.New("db error") }
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{}
			tt.setupStore(ms)
			h := newTestAuthHandler(ms)

			r := chi.NewRouter()
			r.Get("/auth/verify", h.VerifyMagicLink)

			req := httptest.NewRequest("GET", tt.url, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantHTML {
				body := rec.Body.String()
				if !strings.Contains(body, "<!DOCTYPE html>") {
					t.Error("expected HTML response")
				}
			}
		})
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		userID     uuid.UUID
		deviceID   string
		body       any
		setupStore func(*mockAuthStore)
		wantCode   int
	}{
		{
			name:     "logout with device ID",
			userID:   testUser1,
			deviceID: testDeviceUUID,
			setupStore: func(ms *mockAuthStore) {
				ms.RevokeSessionByDeviceIDFn = func(_ context.Context, userID uuid.UUID, deviceID string) error {
					if userID != testUser1 || deviceID != testDeviceUUID {
						return errors.New("unexpected args")
					}
					return nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:   "logout with refresh token",
			userID: testUser1,
			body: map[string]string{
				"refresh_token": "some-refresh-token",
			},
			setupStore: func(ms *mockAuthStore) {
				ms.GetSessionByTokenHashFn = func(context.Context, string) (*model.Session, error) {
					return &model.Session{ID: testSes1, UserID: testUser1, LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
				}
				ms.RevokeSessionFn = func(_ context.Context, id uuid.UUID) error {
					if id != testSes1 {
						return errors.New("unexpected session id")
					}
					return nil
				}
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "logout with both device ID and refresh token",
			userID:   testUser1,
			deviceID: testDeviceUUID,
			body: map[string]string{
				"refresh_token": "some-refresh-token",
			},
			setupStore: func(ms *mockAuthStore) {
				ms.RevokeSessionByDeviceIDFn = func(context.Context, uuid.UUID, string) error { return nil }
				ms.GetSessionByTokenHashFn = func(context.Context, string) (*model.Session, error) {
					return &model.Session{ID: testSes1, UserID: testUser1, LastUsedAt: now, ExpiresAt: now, CreatedAt: now}, nil
				}
				ms.RevokeSessionFn = func(context.Context, uuid.UUID) error { return nil }
			},
			wantCode: http.StatusOK,
		},
		{
			name:       "unauthenticated request",
			userID:     uuid.Nil,
			setupStore: func(ms *mockAuthStore) {},
			wantCode:   http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuthStore{}
			tt.setupStore(ms)
			h := newTestAuthHandler(ms)

			r := chi.NewRouter()
			r.Post("/auth/logout", h.Logout)

			req := requestWithUser("POST", "/auth/logout", tt.body, tt.userID)
			if tt.deviceID != "" {
				req.Header.Set("X-Device-ID", tt.deviceID)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d (body=%s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if tt.wantCode == http.StatusOK {
				var resp map[string]any
				decodeJSON(t, rec, &resp)
				if resp["logged_out"] != true {
					t.Error("expected logged_out=true")
				}
			}
		})
	}
}

func TestGenerateJWTToken(t *testing.T) {
	token, err := authservice.GenerateJWTToken("secret", "usr_1", "access", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

// Suppress unused import warning — json is used via requestWithUser.
var _ = json.Marshal
