//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"mycli.sh/test/integration/harness"
)

// TestRefreshTokenReuseGrace drives the real POST /v1/auth/refresh endpoint to
// verify the reuse grace: a duplicate/racing submission of a just-rotated
// refresh token is accepted within the grace window, but a previous token whose
// grace has lapsed — and any token on an expired session — is rejected.
func TestRefreshTokenReuseGrace(t *testing.T) {
	h := harness.Start(t)
	ctx := context.Background()
	secret := h.JWTSecret()
	userID := h.SeedUser(t, "grace@test.local", "graceuser")

	hash := func(tok string) string {
		sum := sha256.Sum256([]byte(tok))
		return hex.EncodeToString(sum[:])
	}
	// jti makes every minted token unique — otherwise tokens built in the same
	// second share identical claims, hash to the same value, and collide on the
	// sessions.refresh_token_hash UNIQUE constraint.
	jti := 0
	mkRefreshJWT := func(exp time.Duration) string {
		jti++
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":  userID.String(),
			"type": "refresh",
			"jti":  jti,
			"iat":  time.Now().Unix(),
			"exp":  time.Now().Add(exp).Unix(),
		})
		s, err := tok.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("sign refresh jwt: %v", err)
		}
		return s
	}
	// insertSession creates a session row, optionally with a populated previous
	// token + grace deadline. Returns nothing; refresh_token_hash = hash(cur).
	insertSession := func(t *testing.T, cur string, expiresIn time.Duration, prev string, prevGraceIn time.Duration) {
		t.Helper()
		var prevHash any
		var prevUntil any
		if prev != "" {
			prevHash = hash(prev)
			prevUntil = time.Now().Add(prevGraceIn)
		}
		if _, err := h.DB.Exec(ctx,
			`INSERT INTO sessions (user_id, refresh_token_hash, expires_at, previous_refresh_token_hash, previous_hash_valid_until)
			 VALUES ($1, $2, $3, $4, $5)`,
			userID, hash(cur), time.Now().Add(expiresIn), prevHash, prevUntil,
		); err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}
	postRefresh := func(t *testing.T, token string) int {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"refresh_token": token})
		req, err := http.NewRequestWithContext(ctx, "POST", h.APIURL+"/v1/auth/refresh", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST refresh: %v", err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	t.Run("grace_replay_accepted", func(t *testing.T) {
		r0 := mkRefreshJWT(60 * 24 * time.Hour)
		insertSession(t, r0, 60*24*time.Hour, "", 0)

		if code := postRefresh(t, r0); code != http.StatusOK {
			t.Fatalf("first refresh: got %d, want 200", code)
		}
		// r0 is now the previous token, within its 30s grace: a duplicate/racing
		// submission must still succeed instead of wiping the session.
		if code := postRefresh(t, r0); code != http.StatusOK {
			t.Errorf("grace replay: got %d, want 200", code)
		}
	})

	t.Run("previous_token_after_grace_rejected", func(t *testing.T) {
		cur := mkRefreshJWT(60 * 24 * time.Hour)
		prev := mkRefreshJWT(60 * 24 * time.Hour)
		insertSession(t, cur, 60*24*time.Hour, prev, -time.Minute) // grace already lapsed

		if code := postRefresh(t, prev); code != http.StatusUnauthorized {
			t.Errorf("lapsed previous token: got %d, want 401", code)
		}
		if code := postRefresh(t, cur); code != http.StatusOK {
			t.Errorf("current token: got %d, want 200", code)
		}
	})

	t.Run("unknown_token_rejected", func(t *testing.T) {
		if code := postRefresh(t, mkRefreshJWT(60*24*time.Hour)); code != http.StatusUnauthorized {
			t.Errorf("unknown token: got %d, want 401", code)
		}
	})

	t.Run("expired_session_rejected", func(t *testing.T) {
		cur := mkRefreshJWT(60 * 24 * time.Hour)
		insertSession(t, cur, -time.Hour, "", 0) // session already expired

		if code := postRefresh(t, cur); code != http.StatusUnauthorized {
			t.Errorf("expired session: got %d, want 401", code)
		}
	})
}
