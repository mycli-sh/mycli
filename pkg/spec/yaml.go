package spec

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// isYAML returns true if data appears to be YAML (not JSON).
// It checks whether the first non-whitespace byte is '{'.
func isYAML(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return false
		default:
			return true
		}
	}
	return false
}

// yamlToJSON converts YAML bytes to JSON bytes.
func yamlToJSON(data []byte) ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	v = convertYAMLValue(v)
	return json.Marshal(v)
}

// ToJSON converts spec data to JSON. If the input is already JSON, it is
// returned as-is. If it is YAML, it is converted to JSON.
func ToJSON(data []byte) ([]byte, error) {
	if !isYAML(data) {
		return data, nil
	}
	return yamlToJSON(data)
}

// convertYAMLValue recursively normalizes yaml.v3 output so that
// map keys are strings (JSON requires string keys).
func convertYAMLValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[k] = convertYAMLValue(v)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = convertYAMLValue(v)
		}
		return out
	default:
		return v
	}
}
