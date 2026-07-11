package auth

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func init() {
	keyring.MockInit()
}

// saveBG stores tokens for a background-refresh eligibility test.
func saveBG(t *testing.T, tk *Tokens) {
	t.Helper()
	if err := SaveTokens(tk); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
}

func TestMaybeRefreshInBackground_Eligibility(t *testing.T) {
	cases := []struct {
		name      string
		tokens    *Tokens
		wantFires bool
	}{
		{
			name:      "fires_when_due",
			tokens:    &Tokens{AccessToken: "a", RefreshToken: "r"},
			wantFires: true,
		},
		{
			name:      "skips_without_refresh_token",
			tokens:    &Tokens{AccessToken: "a"},
			wantFires: false,
		},
		{
			name:      "skips_api_token",
			tokens:    &Tokens{AccessToken: "myc_abc", RefreshToken: "r"},
			wantFires: false,
		},
		{
			name:      "skips_within_post_login_grace",
			tokens:    &Tokens{AccessToken: "a", RefreshToken: "r", LoggedInAt: time.Now()},
			wantFires: false,
		},
		{
			name:      "skips_within_refresh_interval",
			tokens:    &Tokens{AccessToken: "a", RefreshToken: "r", LastRefreshedAt: time.Now()},
			wantFires: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			_ = ClearTokens()
			saveBG(t, tc.tokens)

			var fired atomic.Bool
			MaybeRefreshInBackground(func() bool {
				fired.Store(true)
				return true
			})
			WaitForBackgroundRefresh(time.Second)

			if got := fired.Load(); got != tc.wantFires {
				t.Errorf("fired = %v, want %v", got, tc.wantFires)
			}
		})
	}
}

func TestWaitForBackgroundRefresh_WaitsForInFlight(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = ClearTokens()
	saveBG(t, &Tokens{AccessToken: "a", RefreshToken: "r"})

	release := make(chan struct{})
	MaybeRefreshInBackground(func() bool {
		<-release
		return true
	})

	done := make(chan struct{})
	go func() {
		WaitForBackgroundRefresh(2 * time.Second)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("WaitForBackgroundRefresh returned before the refresh finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WaitForBackgroundRefresh did not return after the refresh finished")
	}
}

func TestWaitForBackgroundRefresh_TimesOut(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = ClearTokens()
	saveBG(t, &Tokens{AccessToken: "a", RefreshToken: "r"})

	release := make(chan struct{})
	// Ensure the blocked goroutine is drained before the test ends so it can't
	// leak into other tests sharing the package-global wait group.
	t.Cleanup(func() {
		close(release)
		WaitForBackgroundRefresh(time.Second)
	})
	MaybeRefreshInBackground(func() bool {
		<-release
		return true
	})

	start := time.Now()
	WaitForBackgroundRefresh(50 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("WaitForBackgroundRefresh blocked %v; expected ~timeout", elapsed)
	}
}

func TestWaitForBackgroundRefresh_NothingInFlight(t *testing.T) {
	start := time.Now()
	WaitForBackgroundRefresh(time.Second)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("WaitForBackgroundRefresh blocked %v with nothing in flight", elapsed)
	}
}
