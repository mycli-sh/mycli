package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"mycli.sh/api/internal/model"
)

// ListCommandsByOwner performs a dynamic query with optional cursor pagination
// and ILIKE search. This cannot be expressed as a static sqlc query.
func (s *Store) ListCommandsByOwner(ctx context.Context, ownerID uuid.UUID, cursor string, limit int, query string) ([]model.Command, string, error) {
	if limit <= 0 {
		limit = 20
	}

	args := []any{ownerID, limit + 1}
	whereClause := "owner_user_id = $1 AND deleted_at IS NULL"

	argIdx := 3
	if cursor != "" {
		cursorUUID, err := uuid.Parse(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		whereClause += fmt.Sprintf(" AND id > $%d", argIdx)
		args = append(args, cursorUUID)
		argIdx++
	}
	if query != "" {
		whereClause += fmt.Sprintf(" AND (name ILIKE $%d OR slug ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, "%"+query+"%")
	}

	rows, err := s.db.Query(ctx,
		fmt.Sprintf(
			`SELECT id, owner_user_id, name, slug, description, tags, library_id, created_at, updated_at, deleted_at
			 FROM commands
			 WHERE %s
			 ORDER BY id ASC
			 LIMIT $2`, whereClause),
		args...,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list commands by owner: %w", err)
	}
	defer rows.Close()

	var commands []model.Command
	for rows.Next() {
		var c model.Command
		if err := rows.Scan(
			&c.ID, &c.OwnerUserID, &c.Name, &c.Slug,
			&c.Description, &c.Tags, &c.LibraryID, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan command: %w", err)
		}
		commands = append(commands, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate commands: %w", err)
	}

	var nextCursor string
	if len(commands) > limit {
		commands = commands[:limit]
		nextCursor = commands[limit-1].ID.String()
	}

	return commands, nextCursor, nil
}

// SearchPublicLibraries performs a dynamic ILIKE search across public
// libraries with offset pagination. This cannot be expressed as a static sqlc
// query because the search condition is optional.
func (s *Store) SearchPublicLibraries(ctx context.Context, query string, limit, offset int) ([]model.Library, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	countRow := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM libraries l
		 LEFT JOIN users u ON u.id = l.owner_id
		 WHERE l.is_public = true
		   AND ($1 = '' OR l.name ILIKE '%' || $1 || '%'
		     OR l.slug ILIKE '%' || $1 || '%'
		     OR l.description ILIKE '%' || $1 || '%'
		     OR u.username ILIKE '%' || $1 || '%')`,
		query,
	)
	var total int
	if err := countRow.Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count public libraries: %w", err)
	}

	rows, err := s.db.Query(ctx,
		`SELECT l.id, l.owner_id, l.slug, l.name, l.description, l.git_url, l.is_public, l.install_count, l.latest_version, l.created_at, l.updated_at, l.aliases
		 FROM libraries l
		 LEFT JOIN users u ON u.id = l.owner_id
		 WHERE l.is_public = true
		   AND ($1 = '' OR l.name ILIKE '%' || $1 || '%'
		     OR l.slug ILIKE '%' || $1 || '%'
		     OR l.description ILIKE '%' || $1 || '%'
		     OR u.username ILIKE '%' || $1 || '%')
		 ORDER BY l.install_count DESC, l.name ASC
		 LIMIT $2 OFFSET $3`,
		query, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("search public libraries: %w", err)
	}
	defer rows.Close()

	var libs []model.Library
	for rows.Next() {
		var lib model.Library
		if err := rows.Scan(
			&lib.ID, &lib.OwnerID, &lib.Slug, &lib.Name, &lib.Description,
			&lib.GitURL, &lib.IsPublic, &lib.InstallCount, &lib.LatestVersion, &lib.CreatedAt, &lib.UpdatedAt, &lib.Aliases,
		); err != nil {
			return nil, 0, fmt.Errorf("scan library: %w", err)
		}
		libs = append(libs, lib)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return libs, total, nil
}
