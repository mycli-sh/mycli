package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Username  *string   `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
