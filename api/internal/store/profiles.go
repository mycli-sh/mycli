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

func scanProfile(row pgx.Row) (*model.Profile, error) {
	var p model.Profile
	err := row.Scan(&p.ID, &p.OwnerUserID, &p.Slug, &p.Name, &p.Description, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreateProfile(ctx context.Context, ownerID uuid.UUID, slug, name, description string) (*model.Profile, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO profiles (owner_user_id, slug, name, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, owner_user_id, slug, name, description, is_default, created_at, updated_at`,
		ownerID, slug, name, description)
	p, err := scanProfile(row)
	if err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}
	return p, nil
}

func (s *Store) GetProfileByOwnerAndSlug(ctx context.Context, ownerID uuid.UUID, slug string) (*model.Profile, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, owner_user_id, slug, name, description, is_default, created_at, updated_at
		FROM profiles WHERE owner_user_id = $1 AND slug = $2`, ownerID, slug)
	p, err := scanProfile(row)
	if err != nil {
		return nil, fmt.Errorf("get profile by owner and slug: %w", err)
	}
	return p, nil
}

func (s *Store) ListProfilesByOwner(ctx context.Context, ownerID uuid.UUID) ([]model.Profile, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, owner_user_id, slug, name, description, is_default, created_at, updated_at
		FROM profiles WHERE owner_user_id = $1
		ORDER BY is_default DESC, created_at ASC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	var profiles []model.Profile
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.OwnerUserID, &p.Slug, &p.Name, &p.Description, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func (s *Store) UpdateProfile(ctx context.Context, id uuid.UUID, name, description string) (*model.Profile, error) {
	row := s.db.QueryRow(ctx, `
		UPDATE profiles SET name = $2, description = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, owner_user_id, slug, name, description, is_default, created_at, updated_at`,
		id, name, description)
	p, err := scanProfile(row)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return p, nil
}

func (s *Store) DeleteProfile(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM profiles WHERE id = $1 AND is_default = false`, id)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetDefaultProfile(ctx context.Context, ownerID, profileID uuid.UUID) error {
	return s.withTx(ctx, func(tx *Store) error {
		// Unset current default
		if _, err := tx.db.Exec(ctx, `
			UPDATE profiles SET is_default = false WHERE owner_user_id = $1 AND is_default = true`, ownerID); err != nil {
			return fmt.Errorf("unset default: %w", err)
		}
		// Set new default
		tag, err := tx.db.Exec(ctx, `
			UPDATE profiles SET is_default = true, updated_at = now()
			WHERE id = $1 AND owner_user_id = $2`, profileID, ownerID)
		if err != nil {
			return fmt.Errorf("set default: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) GetDefaultProfile(ctx context.Context, ownerID uuid.UUID) (*model.Profile, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, owner_user_id, slug, name, description, is_default, created_at, updated_at
		FROM profiles WHERE owner_user_id = $1 AND is_default = true`, ownerID)
	p, err := scanProfile(row)
	if err != nil {
		return nil, fmt.Errorf("get default profile: %w", err)
	}
	return p, nil
}

// GetProfileByOwner fetches a profile by ID, scoped to the given owner.
// Returns ErrNotFound if the profile does not exist or belongs to a different user.
func (s *Store) GetProfileByOwner(ctx context.Context, ownerID, profileID uuid.UUID) (*model.Profile, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, owner_user_id, slug, name, description, is_default, created_at, updated_at
		FROM profiles WHERE id = $1 AND owner_user_id = $2`, profileID, ownerID)
	p, err := scanProfile(row)
	if err != nil {
		return nil, fmt.Errorf("get profile by owner: %w", err)
	}
	return p, nil
}

// AddLibraryToProfile adds a library to a profile and bumps the library's
// install count on the first add (the unique constraint catches duplicates).
func (s *Store) AddLibraryToProfile(ctx context.Context, profileID, libraryID uuid.UUID) error {
	return s.withTx(ctx, func(tx *Store) error {
		tag, err := tx.db.Exec(ctx, `
			INSERT INTO profile_libraries (profile_id, library_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, profileID, libraryID)
		if err != nil {
			return fmt.Errorf("add library to profile: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if err := tx.q.IncrementInstallCount(ctx, libraryID); err != nil {
			return fmt.Errorf("increment install count: %w", err)
		}
		return nil
	})
}

// RemoveLibraryFromProfile removes a library from a profile and decrements
// the install count when a row was actually deleted.
func (s *Store) RemoveLibraryFromProfile(ctx context.Context, profileID, libraryID uuid.UUID) error {
	return s.withTx(ctx, func(tx *Store) error {
		tag, err := tx.db.Exec(ctx, `
			DELETE FROM profile_libraries WHERE profile_id = $1 AND library_id = $2`, profileID, libraryID)
		if err != nil {
			return fmt.Errorf("remove library from profile: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if err := tx.q.DecrementInstallCount(ctx, libraryID); err != nil {
			return fmt.Errorf("decrement install count: %w", err)
		}
		return nil
	})
}

func (s *Store) ListProfileLibraries(ctx context.Context, profileID uuid.UUID) ([]model.Library, error) {
	rows, err := s.db.Query(ctx, `
		SELECT l.id, l.owner_id, l.slug, l.name, l.description, l.git_url, l.aliases,
		       l.is_public, l.install_count, l.latest_version, l.created_at, l.updated_at
		FROM libraries l
		JOIN profile_libraries pl ON pl.library_id = l.id
		WHERE pl.profile_id = $1
		ORDER BY l.name`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list profile libraries: %w", err)
	}
	defer rows.Close()

	var libs []model.Library
	for rows.Next() {
		var l model.Library
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&l.ID, &l.OwnerID, &l.Slug, &l.Name, &l.Description, &l.GitURL, &l.Aliases,
			&l.IsPublic, &l.InstallCount, &l.LatestVersion, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan profile library: %w", err)
		}
		l.CreatedAt = createdAt
		l.UpdatedAt = updatedAt
		libs = append(libs, l)
	}
	return libs, nil
}
