package model

import "time"

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  *string   `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
