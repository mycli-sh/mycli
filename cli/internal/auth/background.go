package auth

import (
	"time"
)

const (
	// backgroundRefreshInterval is the minimum time between background refresh
	// attempts. Set well below the server's RefreshTokenDuration (30 days) so
	// an active user effectively never sees their session expire.
	backgroundRefreshInterval = 7 * 24 * time.Hour

	// postLoginGrace skips background refresh for a short window after login,
	// while the user is likely running rapid setup commands and the normal
	// proactive-refresh path already keeps the access token fresh.
	postLoginGrace = 30 * time.Minute
)

// MaybeRefreshInBackground fires a fire-and-forget goroutine that refreshes
// the JWT session if eligible. It is safe to call on every CLI invocation —
// the cost is one keyring/file read on the hot path and (rarely) one HTTP
// request in a goroutine. Failures are silent; the next 401 will trigger the
// reactive refresh in the HTTP client anyway.
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
	go func() {
		_ = refresh()
	}()
}
