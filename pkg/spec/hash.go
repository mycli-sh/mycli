package spec

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func Hash(s *CommandSpec) (string, error) {
	// Deterministic JSON: struct fields are marshaled in declaration order, ensuring consistent hashes.
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal for hash: %w", err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}

// CanonicalSpecBytes converts a spec (YAML or JSON) to a canonical JSON byte
// sequence: keys sorted lexicographically, no whitespace, numeric values kept
// as-is. The CLI hashes these exact bytes and sends them as the release
// payload; the server hashes what it received without re-marshaling. This
// eliminates any drift between how each side serializes the same content.
func CanonicalSpecBytes(raw []byte) ([]byte, error) {
	jsonData, err := ToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("format conversion: %w", err)
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(jsonData))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := writeCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, item := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		buf.Write(b)
	}
	return nil
}

// LibraryReleaseHashInput describes one library at one version. LibraryReleaseHash
// digests this into a stable per-library content hash. The bundle release payload
// carries one hash per library — this is what the server compares to detect
// idempotent retries (same hash → 200) vs. real conflicts (different hash → 409).
type LibraryReleaseHashInput struct {
	Slug        string
	Name        string
	Description string
	Aliases     []string
	Specs       []SpecHashEntry
}

// SpecHashEntry pairs a spec slug with its canonical JSON bytes.
type SpecHashEntry struct {
	Slug  string
	Bytes []byte
}

// LibraryReleaseHash returns a deterministic sha256:hex digest scoped to a
// single library at a single version. Format is versioned so future formula
// changes can bump the "v1" prefix without silently colliding.
func LibraryReleaseHash(input LibraryReleaseHashInput) string {
	var b bytes.Buffer
	b.WriteString("v1\n")
	b.WriteString(input.Slug)
	b.WriteByte('\t')
	b.WriteString(input.Name)
	b.WriteByte('\t')
	b.WriteString(input.Description)
	b.WriteByte('\t')

	aliases := append([]string(nil), input.Aliases...)
	sort.Strings(aliases)
	b.WriteString(strings.Join(aliases, ","))
	b.WriteByte('\n')

	entries := append([]SpecHashEntry(nil), input.Specs...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Slug < entries[j].Slug })
	for _, s := range entries {
		b.WriteString(s.Slug)
		b.WriteByte('\t')
		h := sha256.Sum256(s.Bytes)
		fmt.Fprintf(&b, "%x", h)
		b.WriteByte('\n')
	}
	h := sha256.Sum256(b.Bytes())
	return fmt.Sprintf("sha256:%x", h)
}
