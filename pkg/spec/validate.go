package spec

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/command-v1.schema.json
var schemaFS embed.FS

var (
	compiledSchema    *jsonschema.Schema
	compiledSchemaErr error
	compileSchemaOnce sync.Once
)

func getCompiledSchema() (*jsonschema.Schema, error) {
	compileSchemaOnce.Do(func() {
		data, err := schemaFS.ReadFile("schema/command-v1.schema.json")
		if err != nil {
			compiledSchemaErr = fmt.Errorf("failed to read embedded schema: %w", err)
			return
		}

		schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			compiledSchemaErr = fmt.Errorf("failed to unmarshal schema JSON: %w", err)
			return
		}

		c := jsonschema.NewCompiler()
		schemaURL := "command-v1.schema.json"
		if err := c.AddResource(schemaURL, schemaDoc); err != nil {
			compiledSchemaErr = fmt.Errorf("failed to add schema resource: %w", err)
			return
		}
		compiledSchema, err = c.Compile(schemaURL)
		if err != nil {
			compiledSchemaErr = fmt.Errorf("failed to compile schema: %w", err)
		}
	})
	return compiledSchema, compiledSchemaErr
}

func Validate(data []byte) error {
	schema, err := getCompiledSchema()
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

	// Check that arg validation constraints are well-formed and applicable.
	if err := validateArgConstraints(s); err != nil {
		return err
	}

	return nil
}

// validateArgConstraints verifies that validation constraints declared on args
// are well-formed (regex compiles, bounds ordered), applicable to the arg's
// type, and that any default value satisfies its own constraints. These are
// authoring errors, caught when a spec is parsed/pushed.
func validateArgConstraints(s *CommandSpec) error {
	for _, p := range s.Args.Positional {
		if p.Pattern != "" {
			if _, err := regexp.Compile(p.Pattern); err != nil {
				return fmt.Errorf("argument %q has an invalid pattern: %w", p.Name, err)
			}
		}
		if p.MinLength != nil && p.MaxLength != nil && *p.MinLength > *p.MaxLength {
			return fmt.Errorf("argument %q: minLength (%d) cannot exceed maxLength (%d)", p.Name, *p.MinLength, *p.MaxLength)
		}
		if p.Default != "" {
			if err := validatePositionalValue(p, p.Default); err != nil {
				return fmt.Errorf("default for %w", err)
			}
		}
	}

	for _, f := range s.Args.Flags {
		hasStringConstraints := len(f.Enum) > 0 || f.Pattern != "" || f.MinLength != nil || f.MaxLength != nil
		hasIntConstraints := f.Min != nil || f.Max != nil

		switch f.GetType() {
		case "bool":
			if hasStringConstraints || hasIntConstraints {
				return fmt.Errorf("flag %q is bool and cannot have validation constraints", f.Name)
			}
		case "int":
			if hasStringConstraints {
				return fmt.Errorf("flag %q is int: use min/max, not enum/pattern/minLength/maxLength", f.Name)
			}
			if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
				return fmt.Errorf("flag %q: min (%d) cannot exceed max (%d)", f.Name, *f.Min, *f.Max)
			}
		default: // string
			if hasIntConstraints {
				return fmt.Errorf("flag %q is string: use minLength/maxLength, not min/max", f.Name)
			}
			if f.Pattern != "" {
				if _, err := regexp.Compile(f.Pattern); err != nil {
					return fmt.Errorf("flag %q has an invalid pattern: %w", f.Name, err)
				}
			}
			if f.MinLength != nil && f.MaxLength != nil && *f.MinLength > *f.MaxLength {
				return fmt.Errorf("flag %q: minLength (%d) cannot exceed maxLength (%d)", f.Name, *f.MinLength, *f.MaxLength)
			}
		}

		if f.GetType() != "bool" && f.Default != nil {
			if def := DefaultToString(f.Default); def != "" {
				if err := validateFlagValue(f, def); err != nil {
					return fmt.Errorf("default for %w", err)
				}
			}
		}
	}

	return nil
}
