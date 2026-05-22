//go:build integration

package harness

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// SeedUser creates a user with a default profile and the given username.
// The two inserts run in a transaction to match the production
// CreateUserWithDefaultProfile path.
func (h *Harness) SeedUser(t *testing.T, email, username string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (email, username) VALUES ($1, $2) RETURNING id`,
		email, username,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO profiles (owner_user_id, slug, name, description, is_default)
		 VALUES ($1, 'default', 'Default', 'Default profile', true)`,
		userID,
	); err != nil {
		t.Fatalf("insert default profile: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return userID
}

// IssueAPIToken inserts a fresh API token for the given user and returns the
// raw token value (the same myc_<hex> shape the CLI sends as a Bearer token).
func (h *Harness) IssueAPIToken(t *testing.T, userID uuid.UUID, name string) string {
	t.Helper()
	ctx := context.Background()

	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}
	token := "myc_" + hex.EncodeToString(raw)
	prefix := token[:12] + "..."
	sum := sha256.Sum256([]byte(token))
	hash := fmt.Sprintf("%x", sum)

	if _, err := h.DB.Exec(ctx,
		`INSERT INTO api_tokens (user_id, name, token_hash, token_prefix)
		 VALUES ($1, $2, $3, $4)`,
		userID, name, hash, prefix,
	); err != nil {
		t.Fatalf("insert api token: %v", err)
	}
	return token
}
