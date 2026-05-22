//go:build integration

package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// PublishLibraryRelease POSTs a single library release with the given command
// specs to /v1/libraries/{slug}/releases. The caller's token must be a valid
// API token (use IssueAPIToken).
//
// commandSpecs are raw JSON spec documents matching pkg/spec/schema.
func (h *Harness) PublishLibraryRelease(
	t *testing.T,
	token, slug, name, description, tag string,
	commandSpecs ...json.RawMessage,
) {
	t.Helper()

	body := map[string]any{
		"tag":         tag,
		"commit_hash": "0000000000000000000000000000000000000000",
		"name":        name,
		"description": description,
		"commands":    commandSpecs,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal release body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, h.APIURL+"/v1/libraries/"+slug+"/releases", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("publish %s: HTTP %d: %s", slug, resp.StatusCode, string(raw))
	}
}

// EchoSpec returns a JSON command spec that runs `echo <marker>` so tests can
// assert deterministic stdout.
func EchoSpec(slug, marker string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
  "schemaVersion": 1,
  "kind": "command",
  "metadata": {
    "name": %q,
    "slug": %q,
    "description": "Integration test echo"
  },
  "steps": [
    {"name": "emit", "run": %q}
  ]
}`, slug, slug, "echo "+marker))
}
