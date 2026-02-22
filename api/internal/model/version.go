package model

import (
	"encoding/json"
	"time"
)

type CommandVersion struct {
	ID        string          `json:"id"`
	CommandID string          `json:"command_id"`
	Version   int             `json:"version"`
	SpecJSON  json.RawMessage `json:"spec_json"`
	SpecHash  string          `json:"spec_hash"`
	Message   string          `json:"message"`
	CreatedBy string          `json:"created_by"`
	CreatedAt time.Time       `json:"created_at"`
}
