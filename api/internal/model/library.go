package model

import "time"

type Library struct {
	ID            string    `json:"id"`
	OwnerID       *string   `json:"owner_id,omitempty"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	GitURL        *string   `json:"git_url,omitempty"`
	IsPublic      bool      `json:"is_public"`
	InstallCount  int       `json:"install_count"`
	LatestVersion *string   `json:"latest_version,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type LibraryInstallation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	LibraryID string    `json:"library_id"`
	CreatedAt time.Time `json:"created_at"`
}
