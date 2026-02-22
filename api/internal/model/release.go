package model

import "time"

type LibraryRelease struct {
	ID           string    `json:"id"`
	LibraryID    string    `json:"library_id"`
	Version      string    `json:"version"`
	Tag          string    `json:"tag"`
	CommitHash   string    `json:"commit_hash"`
	CommandCount int       `json:"command_count"`
	ReleasedBy   string    `json:"released_by"`
	ReleasedAt   time.Time `json:"released_at"`
}
