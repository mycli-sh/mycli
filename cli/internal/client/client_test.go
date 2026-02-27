package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"mycli.sh/cli/internal/auth"
)

func init() {
	keyring.MockInit()
}

// writeJSON writes a JSON response with the correct Content-Type header.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// saveTestTokens saves tokens to the mock keyring with the given access token,
// refresh token, and expiry time.
func saveTestTokens(t *testing.T, access, refresh string, expiresAt time.Time) {
	t.Helper()
	if err := auth.SaveTokens(&auth.Tokens{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expiresAt,
	}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
}

func clearTestTokens() {
	_ = auth.ClearTokens()
}

func TestClient_ProactiveRefresh_ExpiredToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	var refreshCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			refreshCalls.Add(1)
			writeJSON(w, http.StatusOK, auth.TokenResponse{
				AccessToken:  "new-access",
				RefreshToken: "new-refresh",
				ExpiresIn:    3600,
			})
		case "/v1/me":
			if r.Header.Get("Authorization") != "Bearer new-access" {
				t.Errorf("expected new-access token, got %q", r.Header.Get("Authorization"))
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": "123", "email": "test@example.com"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Token expired 1 minute ago
	saveTestTokens(t, "old-access", "old-refresh", time.Now().Add(-1*time.Minute))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	if got := refreshCalls.Load(); got != 1 {
		t.Errorf("refresh called %d times, want 1", got)
	}
}

func TestClient_ProactiveRefresh_NearExpiry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	var refreshCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			refreshCalls.Add(1)
			writeJSON(w, http.StatusOK, auth.TokenResponse{
				AccessToken:  "refreshed-access",
				RefreshToken: "refreshed-refresh",
				ExpiresIn:    3600,
			})
		case "/v1/me":
			writeJSON(w, http.StatusOK, map[string]string{"id": "123", "email": "test@example.com"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Token expires in 15s — within the 30s buffer
	saveTestTokens(t, "near-expiry-access", "my-refresh", time.Now().Add(15*time.Second))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	if got := refreshCalls.Load(); got != 1 {
		t.Errorf("refresh called %d times, want 1", got)
	}
}

func TestClient_NoRefresh_ValidToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	var refreshCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			refreshCalls.Add(1)
			w.WriteHeader(http.StatusOK)
		case "/v1/me":
			if r.Header.Get("Authorization") != "Bearer valid-access" {
				t.Errorf("expected valid-access token, got %q", r.Header.Get("Authorization"))
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": "123", "email": "test@example.com"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Token expires in 1 hour — well outside the 30s buffer
	saveTestTokens(t, "valid-access", "my-refresh", time.Now().Add(1*time.Hour))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	if got := refreshCalls.Load(); got != 0 {
		t.Errorf("refresh called %d times, want 0", got)
	}
}

func TestClient_ProactiveRefresh_Fails_UsesOldToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"code": "INVALID_TOKEN", "message": "bad refresh"},
			})
		case "/v1/me":
			// The old token should still be used since refresh failed
			if r.Header.Get("Authorization") != "Bearer old-access" {
				t.Errorf("expected old-access token, got %q", r.Header.Get("Authorization"))
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": "123", "email": "test@example.com"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	saveTestTokens(t, "old-access", "my-refresh", time.Now().Add(-1*time.Minute))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
}

func TestClient_ProactiveRefresh_NoRefreshToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	var refreshCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			refreshCalls.Add(1)
			w.WriteHeader(http.StatusOK)
		case "/v1/me":
			writeJSON(w, http.StatusOK, map[string]string{"id": "123", "email": "test@example.com"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Expired token, but no refresh token stored
	saveTestTokens(t, "expired-access", "", time.Now().Add(-1*time.Minute))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	if got := refreshCalls.Load(); got != 0 {
		t.Errorf("refresh called %d times, want 0 (no refresh token)", got)
	}
}

func TestClient_ProactiveRefresh_UpdatesRefreshToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			writeJSON(w, http.StatusOK, auth.TokenResponse{
				AccessToken:  "rotated-access",
				RefreshToken: "rotated-refresh",
				ExpiresIn:    7200,
			})
		case "/v1/me":
			writeJSON(w, http.StatusOK, map[string]string{"id": "123", "email": "test@example.com"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	saveTestTokens(t, "old-access", "old-refresh", time.Now().Add(-1*time.Minute))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	tokens, err := auth.LoadTokens()
	if err != nil {
		t.Fatalf("LoadTokens: %v", err)
	}
	if tokens.AccessToken != "rotated-access" {
		t.Errorf("access token = %q, want %q", tokens.AccessToken, "rotated-access")
	}
	if tokens.RefreshToken != "rotated-refresh" {
		t.Errorf("refresh token = %q, want %q", tokens.RefreshToken, "rotated-refresh")
	}
}

func TestClient_RetryOn401_RefreshSucceeds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	var meCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			writeJSON(w, http.StatusOK, auth.TokenResponse{
				AccessToken:  "fresh-access",
				RefreshToken: "fresh-refresh",
				ExpiresIn:    3600,
			})
		case "/v1/me":
			call := meCalls.Add(1)
			if call == 1 {
				// First call: return 401 to trigger retry
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]string{"code": "UNAUTHORIZED", "message": "expired"},
				})
				return
			}
			// Retry: should succeed with new token
			if r.Header.Get("Authorization") != "Bearer fresh-access" {
				t.Errorf("retry: expected fresh-access, got %q", r.Header.Get("Authorization"))
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": "123", "email": "test@example.com"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Token looks valid (not expired), so no proactive refresh
	saveTestTokens(t, "stale-access", "my-refresh", time.Now().Add(1*time.Hour))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	if got := meCalls.Load(); got != 2 {
		t.Errorf("/v1/me called %d times, want 2", got)
	}
}

func TestClient_RetryOn401_RefreshFails_ClearsTokens(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"code": "INVALID_TOKEN", "message": "bad"},
			})
		case "/v1/me":
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"code": "UNAUTHORIZED", "message": "expired"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	saveTestTokens(t, "bad-access", "bad-refresh", time.Now().Add(1*time.Hour))

	c := New(srv.URL)
	defer c.Close()

	_, err := c.GetMe()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Tokens should be cleared after failed refresh on 401
	tokens, loadErr := auth.LoadTokens()
	if loadErr == nil && tokens != nil && tokens.AccessToken != "" {
		t.Errorf("expected tokens to be cleared, but access token = %q", tokens.AccessToken)
	}
}
