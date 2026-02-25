package library

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/manifest-v1.schema.json
var manifestSchemaFS embed.FS

var compiledManifestSchema *jsonschema.Schema

func init() {
	data, err := manifestSchemaFS.ReadFile("schema/manifest-v1.schema.json")
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded manifest schema: %v", err))
	}

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal manifest schema JSON: %v", err))
	}

	c := jsonschema.NewCompiler()
	schemaURL := "manifest-v1.schema.json"
	if err := c.AddResource(schemaURL, schemaDoc); err != nil {
		panic(fmt.Sprintf("failed to add manifest schema resource: %v", err))
	}
	compiledManifestSchema, err = c.Compile(schemaURL)
	if err != nil {
		panic(fmt.Sprintf("failed to compile manifest schema: %v", err))
	}
}

// ValidateManifest validates JSON data against the manifest schema
// and performs semantic validation beyond what the schema can express.
func ValidateManifest(data []byte) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := compiledManifestSchema.Validate(v); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("unmarshal for semantic validation: %w", err)
	}
	return validateManifestSemantics(&m)
}

// validateManifestSemantics checks constraints that JSON Schema cannot express,
// such as alias collisions with library keys and duplicate aliases.
func validateManifestSemantics(m *Manifest) error {
	libKeys := make(map[string]bool)
	for key := range m.Libraries {
		libKeys[key] = true
	}

	allAliases := make(map[string]string) // alias -> owning library key
	for key, libDef := range m.Libraries {
		for _, alias := range libDef.Aliases {
			if alias == key {
				return fmt.Errorf("library alias %q cannot be the same as its own key in %q", alias, key)
			}
			if libKeys[alias] {
				return fmt.Errorf("library alias %q in %q conflicts with library key", alias, key)
			}
			if owner, exists := allAliases[alias]; exists {
				return fmt.Errorf("duplicate library alias %q in %q (already used by %q)", alias, key, owner)
			}
			allAliases[alias] = key
		}
	}
	return nil
}
