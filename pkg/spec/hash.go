package spec

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func Hash(s *CommandSpec) (string, error) {
	// Canonical JSON: marshal with sorted keys (Go's encoding/json does this by default for structs)
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal for hash: %w", err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}
