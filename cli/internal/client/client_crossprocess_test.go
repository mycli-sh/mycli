package client

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mycli.sh/cli/internal/auth"
)

const (
	crossProcHelperEnv = "MYCLI_REFRESH_HELPER"
	crossProcURLEnv    = "MYCLI_FAKE_URL"
)

// TestCrossProcessRefreshHelper is re-invoked as a subprocess by
// TestClient_CrossProcessRefresh_SingleRotation. In an ordinary run it is a
// no-op; under the helper env it performs one real refresh and exits non-zero
// if it fails.
func TestCrossProcessRefreshHelper(t *testing.T) {
	if os.Getenv(crossProcHelperEnv) != "1" {
		return
	}
	c := New(os.Getenv(crossProcURLEnv))
	defer c.Close()
	if !c.RefreshNow() {
		os.Exit(3)
	}
	os.Exit(0)
}

// TestClient_CrossProcessRefresh_SingleRotation spawns several real OS processes
// that refresh concurrently over a shared credential file. The cross-process
// file lock + throttle must collapse them to a single /v1/auth/refresh call, so
// nobody double-POSTs the single-use token and nobody gets logged out. Uses
// MY_NO_KEYRING so the processes share ~/.my/credentials.json on every platform
// (no OS keyring, no macOS keychain prompt).
func TestClient_CrossProcessRefresh_SingleRotation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MY_NO_KEYRING", "1")
	clearTestTokens()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/refresh" {
			http.NotFound(w, r)
			return
		}
		attempts.Add(1)
		// Single-use: only the original token rotates. A second POST of the same
		// token would be rejected — which the file lock must make impossible.
		if refreshTokenFromBody(r) != "R0" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"code": "INVALID_TOKEN", "message": "session not found"},
			})
			return
		}
		writeJSON(w, http.StatusOK, auth.TokenResponse{AccessToken: "A1", RefreshToken: "R1", ExpiresIn: 900})
	}))
	defer srv.Close()

	// Shared seed: access expired, refresh valid, last refresh old enough that
	// the throttle doesn't pre-empt the first refresher.
	if err := auth.SaveTokens(&auth.Tokens{
		AccessToken:     "A0",
		RefreshToken:    "R0",
		ExpiresAt:       time.Now().Add(-time.Minute),
		LastRefreshedAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("seed tokens: %v", err)
	}

	const procs = 5
	var wg sync.WaitGroup
	errs := make([]error, procs)
	for i := 0; i < procs; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrossProcessRefreshHelper$")
			cmd.Env = append(os.Environ(),
				crossProcHelperEnv+"=1",
				crossProcURLEnv+"="+srv.URL,
				"HOME="+home,
				"MY_NO_KEYRING=1",
			)
			errs[i] = cmd.Run()
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("refresher process %d failed (double-POST / logout?): %v", i, e)
		}
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("/v1/auth/refresh hit %d times across %d processes; want exactly 1", got, procs)
	}
	if tk, err := auth.LoadTokens(); err != nil || tk == nil || tk.RefreshToken == "" {
		t.Errorf("credentials were wiped: tokens=%+v err=%v", tk, err)
	}
}
