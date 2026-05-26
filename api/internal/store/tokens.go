package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mycli.sh/api/internal/model"
)

func (s *Store) CreateAPIToken(ctx context.Context, userID uuid.UUID, name, tokenHash, tokenPrefix string, profileID *uuid.UUID, expiresAt *time.Time) (*model.APIToken, error) {
	var t model.APIToken
	err := s.db.QueryRow(ctx, `
		INSERT INTO api_tokens (user_id, name, token_hash, token_prefix, profile_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, name, token_prefix, profile_id, last_used_at, expires_at, created_at`,
		userID, name, tokenHash, tokenPrefix, profileID, expiresAt,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenPrefix, &t.ProfileID, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}
	return &t, nil
}

func (s *Store) ListAPITokens(ctx context.Context, userID uuid.UUID) ([]model.APIToken, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, name, token_prefix, profile_id, last_used_at, expires_at, created_at
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []model.APIToken
	for rows.Next() {
		var t model.APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenPrefix, &t.ProfileID, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func (s *Store) RevokeAPIToken(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM api_tokens WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete api token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetAPITokenByHash(ctx context.Context, tokenHash string) (*model.APIToken, error) {
	var t model.APIToken
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, name, token_prefix, profile_id, last_used_at, expires_at, created_at
		FROM api_tokens
		WHERE token_hash = $1 AND (expires_at IS NULL OR expires_at > now())`,
		tokenHash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenPrefix, &t.ProfileID, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get api token by hash: %w", err)
	}
	return &t, nil
}

func (s *Store) UpdateAPITokenLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE api_tokens SET last_used_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("update api token last used: %w", err)
	}
	return nil
}

// CountAPITokens returns how many API tokens the user currently owns.
func (s *Store) CountAPITokens(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM api_tokens WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count api tokens: %w", err)
	}
	return count, nil
}

func (s *Store) CountTokensByProfile(ctx context.Context, profileID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM api_tokens WHERE profile_id = $1`, profileID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count tokens by profile: %w", err)
	}
	return count, nil
}
