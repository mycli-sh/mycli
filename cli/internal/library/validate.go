package library

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/manifest-v1.schema.json
var manifestSchemaFS embed.FS

var (
	compiledManifestSchema    *jsonschema.Schema
	compiledManifestSchemaErr error
	compileManifestSchemaOnce sync.Once
)

func getCompiledManifestSchema() (*jsonschema.Schema, error) {
	compileManifestSchemaOnce.Do(func() {
		data, err := manifestSchemaFS.ReadFile("schema/manifest-v1.schema.json")
		if err != nil {
			compiledManifestSchemaErr = fmt.Errorf("failed to read embedded manifest schema: %w", err)
			return
		}

		schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			compiledManifestSchemaErr = fmt.Errorf("failed to unmarshal manifest schema JSON: %w", err)
			return
		}

		c := jsonschema.NewCompiler()
		schemaURL := "manifest-v1.schema.json"
		if err := c.AddResource(schemaURL, schemaDoc); err != nil {
			compiledManifestSchemaErr = fmt.Errorf("failed to add manifest schema resource: %w", err)
			return
		}
		compiledManifestSchema, err = c.Compile(schemaURL)
		if err != nil {
			compiledManifestSchemaErr = fmt.Errorf("failed to compile manifest schema: %w", err)
		}
	})
	return compiledManifestSchema, compiledManifestSchemaErr
}

// ValidateManifest validates JSON data against the manifest schema
// and performs semantic validation beyond what the schema can express.
func ValidateManifest(data []byte) error {
	schema, err := getCompiledManifestSchema()
	if err != nil {
		return fmt.Errorf("schema initialization failed: %w", err)
	}

	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := schema.Validate(v); err != nil {
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
