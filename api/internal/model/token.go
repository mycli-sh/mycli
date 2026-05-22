package model

import (
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	ProfileID   *uuid.UUID `json:"profile_id,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
