//go:build integration

package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"mycli.sh/pkg/spec"
)

// bundledLibrary mirrors the server's per-library release payload. The harness
// can't import cli/internal/client (Go's internal rule scopes it to cli/...),
// so the wire shape is restated here.
type bundledLibrary struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Aliases       []string          `json:"aliases,omitempty"`
	ContentSHA256 string            `json:"content_sha256"`
	Commands      []json.RawMessage `json:"commands"`
}

type bundledRelease struct {
	Tag        string           `json:"tag"`
	CommitHash string           `json:"commit_hash"`
	Libraries  []bundledLibrary `json:"libraries"`
}

// PublishLibraryRelease POSTs a single library release with the given command
// specs to /v1/releases, the atomic bundled publish endpoint. The caller's token
// must be a valid API token (use IssueAPIToken). Namespace is left empty so the
// server attributes the release to the caller's own username.
//
// commandSpecs are raw JSON spec documents matching pkg/spec/schema.
func (h *Harness) PublishLibraryRelease(
	t *testing.T,
	token, slug, name, description, tag string,
	commandSpecs ...json.RawMessage,
) {
	t.Helper()

	// Canonicalize before hashing: json.Marshal compacts json.RawMessage, and the
	// server hashes the bytes it receives verbatim. Canonical bytes are already
	// whitespace-free, so what we hash is exactly what lands on the wire.
	commands := make([]json.RawMessage, 0, len(commandSpecs))
	hashEntries := make([]spec.SpecHashEntry, 0, len(commandSpecs))
	for i, raw := range commandSpecs {
		canon, err := spec.CanonicalSpecBytes(raw)
		if err != nil {
			t.Fatalf("canonicalize spec[%d]: %v", i, err)
		}
		parsed, err := spec.Parse(canon)
		if err != nil {
			t.Fatalf("parse spec[%d]: %v", i, err)
		}
		commands = append(commands, canon)
		hashEntries = append(hashEntries, spec.SpecHashEntry{Slug: parsed.Metadata.Slug, Bytes: canon})
	}

	contentHash := spec.LibraryReleaseHash(spec.LibraryReleaseHashInput{
		Slug:        slug,
		Name:        name,
		Description: description,
		Specs:       hashEntries,
	})

	body := bundledRelease{
		Tag:        tag,
		CommitHash: "0000000000000000000000000000000000000000",
		Libraries: []bundledLibrary{{
			Slug:          slug,
			Name:          name,
			Description:   description,
			ContentSHA256: contentHash,
			Commands:      commands,
		}},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal release body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, h.APIURL+"/v1/releases", bytes.NewReader(buf))
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
