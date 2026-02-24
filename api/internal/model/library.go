package model

import (
	"time"

	"github.com/google/uuid"
)

type Library struct {
	ID            uuid.UUID  `json:"id"`
	OwnerID       *uuid.UUID `json:"owner_id,omitempty"`
	Slug          string     `json:"slug"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	GitURL        *string    `json:"git_url,omitempty"`
	IsPublic      bool       `json:"is_public"`
	InstallCount  int        `json:"install_count"`
	LatestVersion *string    `json:"latest_version,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type LibraryInstallation struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	LibraryID uuid.UUID `json:"library_id"`
	CreatedAt time.Time `json:"created_at"`
}
