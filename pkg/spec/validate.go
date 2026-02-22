package spec

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/command-v1.schema.json
var schemaFS embed.FS

var compiledSchema *jsonschema.Schema

func init() {
	data, err := schemaFS.ReadFile("schema/command-v1.schema.json")
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded schema: %v", err))
	}

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal schema JSON: %v", err))
	}

	c := jsonschema.NewCompiler()
	schemaURL := "command-v1.schema.json"
	if err := c.AddResource(schemaURL, schemaDoc); err != nil {
		panic(fmt.Sprintf("failed to add schema resource: %v", err))
	}
	compiledSchema, err = c.Compile(schemaURL)
	if err != nil {
		panic(fmt.Sprintf("failed to compile schema: %v", err))
	}
}

func Validate(data []byte) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	err := compiledSchema.Validate(v)
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	// Semantic validation (beyond what JSON Schema can express)
	var s CommandSpec
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("unmarshal for semantic validation: %w", err)
	}
	if err := validateSemantics(&s); err != nil {
		return err
	}

	return nil
}

func validateSemantics(s *CommandSpec) error {
	// Check for duplicate positional arg names
	posNames := make(map[string]bool)
	for _, p := range s.Args.Positional {
		if posNames[p.Name] {
			return fmt.Errorf("duplicate positional arg name: %q", p.Name)
		}
		posNames[p.Name] = true
	}

	// Check for duplicate flag names
	flagNames := make(map[string]bool)
	for _, f := range s.Args.Flags {
		if flagNames[f.Name] {
			return fmt.Errorf("duplicate flag name: %q", f.Name)
		}
		flagNames[f.Name] = true
	}

	// Check for name collision between positional args and flags
	for _, f := range s.Args.Flags {
		if posNames[f.Name] {
			return fmt.Errorf("flag name %q conflicts with positional arg of the same name", f.Name)
		}
	}

	// Check for duplicate flag short names
	shortNames := make(map[string]bool)
	for _, f := range s.Args.Flags {
		if f.Short != "" {
			if shortNames[f.Short] {
				return fmt.Errorf("duplicate flag short name: %q", f.Short)
			}
			shortNames[f.Short] = true
		}
	}

	// Check that required positional args don't follow optional ones
	seenOptional := false
	for _, p := range s.Args.Positional {
		if !p.IsRequired() {
			seenOptional = true
		} else if seenOptional {
			return fmt.Errorf("required positional arg %q cannot follow an optional positional arg", p.Name)
		}
	}

	// Check for duplicate dependency names
	depNames := make(map[string]bool)
	for _, d := range s.Dependencies {
		if depNames[d] {
			return fmt.Errorf("duplicate dependency: %q", d)
		}
		depNames[d] = true
	}

	// Check that aliases don't equal the command's own slug and have no duplicates
	aliasSet := make(map[string]bool)
	for _, alias := range s.Metadata.Aliases {
		if alias == s.Metadata.Slug {
			return fmt.Errorf("alias %q cannot be the same as the command slug", alias)
		}
		if aliasSet[alias] {
			return fmt.Errorf("duplicate alias: %q", alias)
		}
		aliasSet[alias] = true
	}

	return nil
}
