package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mycli.sh/cli/internal/auth"
)

// refreshTokenFromBody extracts the refresh_token field from a /v1/auth/refresh
// request body.
func refreshTokenFromBody(r *http.Request) string {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.RefreshToken
}

// TestClient_ConcurrentRefresh_TwoClients_SingleRotation is the core regression
// for the reported bug: the background keepalive (one *Client) and a command
// (another *Client) refresh at the same time. Process-global coordination must
// dedupe them to exactly ONE rotation of the single-use refresh token, leave the
// user authenticated, and never wipe credentials.
func TestClient_ConcurrentRefresh_TwoClients_SingleRotation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	var rotations atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/refresh":
			// Only the original token rotates; a second POST of the same
			// single-use token would be rejected. With coordination there is
			// never a second POST.
			if refreshTokenFromBody(r) != "R0" {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]string{"code": "INVALID_TOKEN", "message": "session not found"},
				})
				return
			}
			rotations.Add(1)
			writeJSON(w, http.StatusOK, auth.TokenResponse{AccessToken: "A1", RefreshToken: "R1", ExpiresIn: 3600})
		case "/v1/me":
			if r.Header.Get("Authorization") != "Bearer A1" {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]string{"code": "UNAUTHORIZED", "message": "invalid token"},
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": "1", "email": "a@b.c"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Access token already expired; refresh token still valid; last refresh old
	// so the dedup window does not pre-empt the first refresher.
	if err := auth.SaveTokens(&auth.Tokens{
		AccessToken:     "A0",
		RefreshToken:    "R0",
		ExpiresAt:       time.Now().Add(-time.Minute),
		LastRefreshedAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	cA := New(srv.URL) // background keepalive
	defer cA.Close()
	cB := New(srv.URL) // command
	defer cB.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	var getMeErr error
	go func() { defer wg.Done(); cA.RefreshNow() }()
	go func() { defer wg.Done(); _, getMeErr = cB.GetMe() }()
	wg.Wait()

	if got := rotations.Load(); got != 1 {
		t.Errorf("expected exactly 1 rotation, got %d", got)
	}
	if getMeErr != nil {
		t.Errorf("GetMe failed: %v", getMeErr)
	}
	tokens, err := auth.LoadTokens()
	if err != nil {
		t.Fatalf("LoadTokens: %v", err)
	}
	if tokens.AccessToken != "A1" || tokens.RefreshToken != "R1" {
		t.Errorf("tokens not rotated/persisted: access=%q refresh=%q", tokens.AccessToken, tokens.RefreshToken)
	}
}

// TestClient_RefreshNow_DedupGuard verifies RefreshNow skips the network call
// when another refresher rotated within the dedup window.
func TestClient_RefreshNow_DedupGuard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearTestTokens()

	var rotations atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/refresh" {
			rotations.Add(1)
			writeJSON(w, http.StatusOK, auth.TokenResponse{AccessToken: "A1", RefreshToken: "R1", ExpiresIn: 3600})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// LastRefreshedAt is fresh, so RefreshNow should dedupe.
	if err := auth.SaveTokens(&auth.Tokens{
		AccessToken:     "A0",
		RefreshToken:    "R0",
		ExpiresAt:       time.Now().Add(time.Hour),
		LastRefreshedAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	c := New(srv.URL)
	defer c.Close()
	if !c.RefreshNow() {
		t.Error("RefreshNow returned false; expected dedup success")
	}
	if got := rotations.Load(); got != 0 {
		t.Errorf("expected no network rotation, got %d", got)
	}
}

// TestClient_Throttle_MinRefreshInterval verifies refreshes are held to the
// minRefreshInterval floor: a rotation within the window is skipped (token still
// valid), one past the window proceeds.
func TestClient_Throttle_MinRefreshInterval(t *testing.T) {
	cases := []struct {
		name         string
		lastRefresh  time.Duration // how long ago the last refresh was
		wantRotation int32
	}{
		{"within_interval_skips", 5 * time.Minute, 0},
		{"past_interval_refreshes", 20 * time.Minute, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			clearTestTokens()

			var rotations atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/auth/refresh" {
					rotations.Add(1)
					writeJSON(w, http.StatusOK, auth.TokenResponse{AccessToken: "A1", RefreshToken: "R1", ExpiresIn: 900})
					return
				}
				http.NotFound(w, r)
			}))
			defer srv.Close()

			if err := auth.SaveTokens(&auth.Tokens{
				AccessToken:     "A0",
				RefreshToken:    "R0",
				ExpiresAt:       time.Now().Add(-time.Minute),
				LastRefreshedAt: time.Now().Add(-tc.lastRefresh),
			}); err != nil {
				t.Fatalf("SaveTokens: %v", err)
			}

			c := New(srv.URL)
			defer c.Close()
			if !c.RefreshNow() {
				t.Error("RefreshNow returned false")
			}
			if got := rotations.Load(); got != tc.wantRotation {
				t.Errorf("rotations = %d, want %d", got, tc.wantRotation)
			}
		})
	}
}

// TestClient_RetryOn401_TransientFailure_KeepsTokens verifies that a
// non-definitive refresh failure (network error or 5xx) does NOT wipe
// credentials — the next invocation can retry.
func TestClient_RetryOn401_TransientFailure_KeepsTokens(t *testing.T) {
	cases := []struct {
		name    string
		refresh func(w http.ResponseWriter, r *http.Request)
	}{
		{"server_5xx", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"code": "TOKEN_ERROR", "message": "boom"},
			})
		}},
		{"malformed_body", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("not json"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			clearTestTokens()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/auth/refresh":
					tc.refresh(w, r)
				case "/v1/me":
					writeJSON(w, http.StatusUnauthorized, map[string]any{
						"error": map[string]string{"code": "UNAUTHORIZED", "message": "invalid token"},
					})
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			saveTestTokens(t, "stale-access", "stale-refresh", time.Now().Add(time.Hour))

			c := New(srv.URL)
			defer c.Close()
			if _, err := c.GetMe(); err == nil {
				t.Fatal("expected error, got nil")
			}

			tokens, err := auth.LoadTokens()
			if err != nil {
				t.Fatalf("LoadTokens: %v", err)
			}
			if tokens.RefreshToken != "stale-refresh" {
				t.Errorf("tokens were wiped on a transient failure: refresh=%q", tokens.RefreshToken)
			}
		})
	}
}

// TestClient_RetryOn401_DefinitiveFailure_ClearsAndReportsExpired verifies that a
// definitive server rejection clears credentials and surfaces ErrSessionExpired
// for each session-dead error code, across both do() and GetCatalog().
func TestClient_RetryOn401_DefinitiveFailure_ClearsAndReportsExpired(t *testing.T) {
	for _, code := range []string{"INVALID_TOKEN", "SESSION_REVOKED", "SESSION_EXPIRED"} {
		t.Run(code, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			clearTestTokens()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/auth/refresh":
					writeJSON(w, http.StatusUnauthorized, map[string]any{
						"error": map[string]string{"code": code, "message": "dead"},
					})
				default:
					writeJSON(w, http.StatusUnauthorized, map[string]any{
						"error": map[string]string{"code": "UNAUTHORIZED", "message": "invalid token"},
					})
				}
			}))
			defer srv.Close()

			saveTestTokens(t, "stale-access", "stale-refresh", time.Now().Add(time.Hour))

			c := New(srv.URL)
			defer c.Close()

			_, err := c.GetCatalog("")
			if !errors.Is(err, ErrSessionExpired) {
				t.Errorf("GetCatalog error = %v; want ErrSessionExpired", err)
			}

			tokens, loadErr := auth.LoadTokens()
			if loadErr == nil && tokens != nil && tokens.RefreshToken != "" {
				t.Errorf("expected tokens cleared, got refresh=%q", tokens.RefreshToken)
			}
		})
	}
}

// TestClient_APIToken_NeverRefreshesOrClears verifies an API token (myc_ prefix)
// hitting a 401 does not trigger a refresh and never clears credentials.
func TestClient_APIToken_NeverRefreshesOrClears(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MY_API_TOKEN", "myc_deadbeef")

	var refreshCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/refresh" {
			refreshCalls.Add(1)
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]string{"code": "UNAUTHORIZED", "message": "invalid token"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	defer c.Close()
	if _, err := c.GetMe(); err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := refreshCalls.Load(); got != 0 {
		t.Errorf("API token triggered %d refresh calls; want 0", got)
	}
	// The env token is always present; ensure ErrSessionExpired is NOT returned.
	if _, err := c.GetMe(); errors.Is(err, ErrSessionExpired) {
		t.Error("API token path returned ErrSessionExpired")
	}
}
