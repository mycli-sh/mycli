package auth

import (
	"sync"
	"time"
)

const (
	// backgroundRefreshInterval is the minimum time between background refresh
	// attempts. Set well below the server's RefreshTokenDuration (60 days) so
	// an active user effectively never sees their session expire.
	backgroundRefreshInterval = 7 * 24 * time.Hour

	// postLoginGrace skips background refresh for a short window after login,
	// while the user is likely running rapid setup commands and the normal
	// proactive-refresh path already keeps the access token fresh.
	postLoginGrace = 30 * time.Minute
)

// bgRefreshWG tracks in-flight background refresh goroutines so the process can
// wait for a rotation to be persisted before exiting (see WaitForBackgroundRefresh).
var bgRefreshWG sync.WaitGroup

// MaybeRefreshInBackground fires a fire-and-forget goroutine that refreshes
// the JWT session if eligible. It is safe to call on every CLI invocation —
// the cost is one keyring/file read on the hot path and (rarely) one HTTP
// request in a goroutine. Failures are silent; the next 401 will trigger the
// reactive refresh in the HTTP client anyway. The refresh callback is globally
// coordinated (see the client package's refresh lock), so it can't race a
// command's own refresh over the same single-use token.
//
// Skips when:
//   - the user is logged out or using an API token (no refresh token to use),
//   - the session was just created (within postLoginGrace),
//   - the last refresh happened within backgroundRefreshInterval.
func MaybeRefreshInBackground(refresh func() bool) {
	tokens, err := LoadTokens()
	if err != nil || tokens.RefreshToken == "" {
		return
	}
	if IsAPIToken(tokens.AccessToken) {
		return
	}
	if !tokens.LoggedInAt.IsZero() && time.Since(tokens.LoggedInAt) < postLoginGrace {
		return
	}
	if !tokens.LastRefreshedAt.IsZero() && time.Since(tokens.LastRefreshedAt) < backgroundRefreshInterval {
		return
	}
	bgRefreshWG.Add(1)
	go func() {
		defer bgRefreshWG.Done()
		_ = refresh()
	}()
}

// WaitForBackgroundRefresh blocks until an in-flight background refresh finishes
// or the timeout elapses. Called just before process exit so a mid-flight
// rotation is persisted rather than dropped (which would brick the single-use
// refresh token). Matters mainly for local-only commands, which otherwise never
// wait on the client's refresh lock. Returns immediately when nothing is in flight.
func WaitForBackgroundRefresh(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		bgRefreshWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}
