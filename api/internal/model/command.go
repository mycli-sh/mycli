package model

import (
	"encoding/json"
	"time"
)

type Command struct {
	ID          string          `json:"id"`
	OwnerUserID string          `json:"owner_user_id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Tags        json.RawMessage `json:"tags"`
	LibraryID   *string         `json:"library_id,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
}
