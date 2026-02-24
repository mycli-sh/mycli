package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Command struct {
	ID          uuid.UUID       `json:"id"`
	OwnerUserID uuid.UUID       `json:"owner_user_id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Tags        json.RawMessage `json:"tags"`
	LibraryID   *uuid.UUID      `json:"library_id,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
}
