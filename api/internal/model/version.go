package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type CommandVersion struct {
	ID        uuid.UUID       `json:"id"`
	CommandID uuid.UUID       `json:"command_id"`
	Version   int             `json:"version"`
	SpecJSON  json.RawMessage `json:"spec_json"`
	SpecHash  string          `json:"spec_hash"`
	Message   string          `json:"message"`
	CreatedBy uuid.UUID       `json:"created_by"`
	CreatedAt time.Time       `json:"created_at"`
}
