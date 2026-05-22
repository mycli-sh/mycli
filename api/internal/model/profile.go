package model

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	ID          uuid.UUID `json:"id"`
	OwnerUserID uuid.UUID `json:"owner_user_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
