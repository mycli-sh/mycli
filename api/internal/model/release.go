package model

import (
	"time"

	"github.com/google/uuid"
)

type LibraryRelease struct {
	ID           uuid.UUID `json:"id"`
	LibraryID    uuid.UUID `json:"library_id"`
	Version      string    `json:"version"`
	Tag          string    `json:"tag"`
	CommitHash   string    `json:"commit_hash"`
	CommandCount int       `json:"command_count"`
	ReleasedBy   uuid.UUID `json:"released_by"`
	ReleasedAt   time.Time `json:"released_at"`
}
