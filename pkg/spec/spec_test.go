package spec

import (
	"encoding/json"
	"testing"
)

const validSpecJSON = `{
  "schemaVersion": 1,
  "kind": "command",
  "metadata": {"name": "Deploy", "slug": "deploy", "description": "Deploy a service"},
  "args": {
    "positional": [{"name": "service"}],
    "flags": [{"name": "env", "short": "e", "type": "string", "default": "staging"}]
  },
  "steps": [{"name": "deploy", "run": "echo deploying {{.args.service}} to {{.args.env}}"}]
}`

func TestParseValidSpec(t *testing.T) {
	spec, err := Parse([]byte(validSpecJSON))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if spec.SchemaVersion != 1 {
		t.Errorf("expected schemaVersion 1, got %d", spec.SchemaVersion)
	}
	if spec.Kind != "command" {
		t.Errorf("expected kind 'command', got %q", spec.Kind)
	}
	if spec.Metadata.Name != "Deploy" {
		t.Errorf("expected metadata.name 'Deploy', got %q", spec.Metadata.Name)
	}
	if spec.Metadata.Slug != "deploy" {
		t.Errorf("expected metadata.slug 'deploy', got %q", spec.Metadata.Slug)
	}
	if spec.Metadata.Description != "Deploy a service" {
		t.Errorf("expected metadata.description 'Deploy a service', got %q", spec.Metadata.Description)
	}
	if len(spec.Args.Positional) != 1 {
		t.Fatalf("expected 1 positional arg, got %d", len(spec.Args.Positional))
	}
	if spec.Args.Positional[0].Name != "service" {
		t.Errorf("expected positional arg name 'service', got %q", spec.Args.Positional[0].Name)
	}
	if len(spec.Args.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(spec.Args.Flags))
	}
	if spec.Args.Flags[0].Name != "env" {
		t.Errorf("expected flag name 'env', got %q", spec.Args.Flags[0].Name)
	}
	if len(spec.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(spec.Steps))
	}
	if spec.Steps[0].Name != "deploy" {
		t.Errorf("expected step name 'deploy', got %q", spec.Steps[0].Name)
	}
}

func TestParseMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "missing schemaVersion",
			json: `{"kind":"command","metadata":{"name":"x","slug":"x"},"steps":[{"name":"s","run":"echo"}]}`,
		},
		{
			name: "missing kind",
			json: `{"schemaVersion":1,"metadata":{"name":"x","slug":"x"},"steps":[{"name":"s","run":"echo"}]}`,
		},
		{
			name: "missing metadata",
			json: `{"schemaVersion":1,"kind":"command","steps":[{"name":"s","run":"echo"}]}`,
		},
		{
			name: "missing steps",
			json: `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"}}`,
		},
		{
			name: "missing metadata.name",
			json: `{"schemaVersion":1,"kind":"command","metadata":{"slug":"x"},"steps":[{"name":"s","run":"echo"}]}`,
		},
		{
			name: "missing metadata.slug",
			json: `{"schemaVersion":1,"kind":"command","metadata":{"name":"x"},"steps":[{"name":"s","run":"echo"}]}`,
		},
		{
			name: "empty steps array",
			json: `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"steps":[]}`,
		},
		{
			name: "missing step.run",
			json: `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"steps":[{"name":"s"}]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.json))
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestValidateInvalidJSON(t *testing.T) {
	err := Validate([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestHashConsistent(t *testing.T) {
	spec, err := Parse([]byte(validSpecJSON))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	h1, err := Hash(spec)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	h2, err := Hash(spec)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	if h1 != h2 {
		t.Errorf("expected consistent hash, got %q and %q", h1, h2)
	}
	if len(h1) == 0 {
		t.Error("expected non-empty hash")
	}
	if h1[:7] != "sha256:" {
		t.Errorf("expected hash to start with 'sha256:', got %q", h1[:7])
	}
}

func TestHashDifferentSpecs(t *testing.T) {
	spec1, err := Parse([]byte(validSpecJSON))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	otherJSON := `{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Build", "slug": "build"},
		"steps": [{"name": "build", "run": "make build"}]
	}`
	spec2, err := Parse([]byte(otherJSON))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	h1, err := Hash(spec1)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	h2, err := Hash(spec2)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	if h1 == h2 {
		t.Error("expected different hashes for different specs")
	}
}

func TestPositionalArgIsRequired(t *testing.T) {
	// Default (nil Required) should return true
	p := PositionalArg{Name: "test"}
	if !p.IsRequired() {
		t.Error("expected PositionalArg.IsRequired() to default to true")
	}

	// Explicit false
	f := false
	p2 := PositionalArg{Name: "test", Required: &f}
	if p2.IsRequired() {
		t.Error("expected PositionalArg.IsRequired() to return false when set to false")
	}

	// Explicit true
	tr := true
	p3 := PositionalArg{Name: "test", Required: &tr}
	if !p3.IsRequired() {
		t.Error("expected PositionalArg.IsRequired() to return true when set to true")
	}
}

func TestFlagArgGetType(t *testing.T) {
	// Default (empty Type) should return "string"
	f := FlagArg{Name: "test"}
	if f.GetType() != "string" {
		t.Errorf("expected FlagArg.GetType() to default to 'string', got %q", f.GetType())
	}

	// Explicit type
	f2 := FlagArg{Name: "test", Type: "bool"}
	if f2.GetType() != "bool" {
		t.Errorf("expected FlagArg.GetType() to return 'bool', got %q", f2.GetType())
	}

	f3 := FlagArg{Name: "test", Type: "int"}
	if f3.GetType() != "int" {
		t.Errorf("expected FlagArg.GetType() to return 'int', got %q", f3.GetType())
	}
}

func TestFlagArgIsRequired(t *testing.T) {
	// Default (nil Required) should return false
	f := FlagArg{Name: "test"}
	if f.IsRequired() {
		t.Error("expected FlagArg.IsRequired() to default to false")
	}

	// Explicit true
	tr := true
	f2 := FlagArg{Name: "test", Required: &tr}
	if !f2.IsRequired() {
		t.Error("expected FlagArg.IsRequired() to return true when set to true")
	}
}

// --- Schema constraint tests (A1-A3, A6) ---

func TestSchemaRejectsEmptyName(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"","slug":"x"},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestSchemaRejectsMultiCharShort(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"flags":[{"name":"f","short":"ab"}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for multi-char short flag")
	}
}

func TestSchemaRejectsEmptyShort(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"flags":[{"name":"f","short":""}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for empty short flag")
	}
}

func TestSchemaRejectsInvalidShell(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"shell":"/usr/local/evil"},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for invalid shell")
	}
}

func TestSchemaAcceptsValidShells(t *testing.T) {
	for _, shell := range []string{"/bin/sh", "/bin/bash", "/bin/zsh", "/usr/bin/env"} {
		j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"shell":"` + shell + `"},"steps":[{"name":"s","run":"echo"}]}`
		if err := Validate([]byte(j)); err != nil {
			t.Errorf("expected no error for shell %q, got: %v", shell, err)
		}
	}
}

func TestSchemaRejectsInvalidTimeout(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"timeout":"30"},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for timeout without unit")
	}
}

func TestSchemaAcceptsValidTimeout(t *testing.T) {
	for _, timeout := range []string{"30s", "5m", "1h", "500ms", "0s"} {
		j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"timeout":"` + timeout + `"},"steps":[{"name":"s","run":"echo"}]}`
		if err := Validate([]byte(j)); err != nil {
			t.Errorf("expected no error for timeout %q, got: %v", timeout, err)
		}
	}
}

func TestSchemaRejectsInvalidEnvVarKey(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"env":{"invalid key":"v"}},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for env var key with space")
	}
}

func TestSchemaAcceptsValidEnvVarKeys(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"env":{"MY_VAR":"v","_foo":"v","PATH":"v"}},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err != nil {
		t.Fatalf("expected no error for valid env var keys, got: %v", err)
	}
}

// --- Semantic validation tests (A4-A5) ---

func TestSemanticRejectsDuplicatePositionalNames(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"positional":[{"name":"a"},{"name":"a"}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for duplicate positional arg names")
	}
}

func TestSemanticRejectsDuplicateFlagNames(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"flags":[{"name":"f"},{"name":"f"}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for duplicate flag names")
	}
}

func TestSemanticRejectsFlagNameCollidingWithPositional(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"positional":[{"name":"name"}],"flags":[{"name":"name"}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for flag name colliding with positional arg")
	}
}

func TestSemanticRejectsDuplicateShortFlags(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"flags":[{"name":"foo","short":"f"},{"name":"bar","short":"f"}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for duplicate short flag names")
	}
}

func TestSemanticRejectsRequiredAfterOptional(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"positional":[{"name":"a","required":false},{"name":"b","required":true}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for required positional after optional")
	}
}

func TestSemanticAcceptsOptionalAfterRequired(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"positional":[{"name":"a","required":true},{"name":"b","required":false}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err != nil {
		t.Fatalf("expected no error for optional after required, got: %v", err)
	}
}

func TestSemanticAcceptsUniqueArgs(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":{"positional":[{"name":"a"},{"name":"b"}],"flags":[{"name":"c","short":"c"},{"name":"d","short":"d"}]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err != nil {
		t.Fatalf("expected no error for unique args, got: %v", err)
	}
}

// --- Alias tests ---

func TestParseSpecWithAliases(t *testing.T) {
	j := `{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Deploy", "slug": "deploy", "aliases": ["dep", "d"]},
		"steps": [{"name": "deploy", "run": "echo deploy"}]
	}`
	s, err := Parse([]byte(j))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(s.Metadata.Aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(s.Metadata.Aliases))
	}
	if s.Metadata.Aliases[0] != "dep" || s.Metadata.Aliases[1] != "d" {
		t.Errorf("unexpected aliases: %v", s.Metadata.Aliases)
	}
}

func TestParseSpecWithoutAliases(t *testing.T) {
	j := `{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Deploy", "slug": "deploy"},
		"steps": [{"name": "deploy", "run": "echo deploy"}]
	}`
	s, err := Parse([]byte(j))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(s.Metadata.Aliases) != 0 {
		t.Errorf("expected no aliases, got %v", s.Metadata.Aliases)
	}
}

func TestSchemaRejectsInvalidAliasPattern(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x","aliases":["INVALID"]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for alias with uppercase letters")
	}
}

func TestSchemaRejectsDuplicateAliases(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x","aliases":["a","a"]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for duplicate aliases (uniqueItems)")
	}
}

func TestSemanticRejectsAliasEqualToSlug(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"deploy","aliases":["dep","deploy"]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for alias equal to slug")
	}
}

func TestSemanticAcceptsValidAliases(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"deploy","aliases":["dep","d"]},"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err != nil {
		t.Fatalf("expected no error for valid aliases, got: %v", err)
	}
}

// --- Dependencies tests ---

func TestParseSpecWithDependencies(t *testing.T) {
	j := `{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Deploy", "slug": "deploy"},
		"dependencies": ["kubectl", "jq", "fzf"],
		"steps": [{"name": "deploy", "run": "echo deploy"}]
	}`
	s, err := Parse([]byte(j))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(s.Dependencies) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(s.Dependencies))
	}
	if s.Dependencies[0] != "kubectl" || s.Dependencies[1] != "jq" || s.Dependencies[2] != "fzf" {
		t.Errorf("unexpected dependencies: %v", s.Dependencies)
	}
}

func TestParseSpecWithoutDependencies(t *testing.T) {
	j := `{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Deploy", "slug": "deploy"},
		"steps": [{"name": "deploy", "run": "echo deploy"}]
	}`
	s, err := Parse([]byte(j))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(s.Dependencies) != 0 {
		t.Errorf("expected no dependencies, got %v", s.Dependencies)
	}
}

func TestSchemaRejectsDuplicateDependencies(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"dependencies":["kubectl","kubectl"],"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for duplicate dependencies (uniqueItems)")
	}
}

func TestSemanticRejectsDuplicateDependencies(t *testing.T) {
	// This tests the semantic layer (belt-and-suspenders with uniqueItems)
	s := &CommandSpec{
		SchemaVersion: 1,
		Kind:          "command",
		Metadata:      Metadata{Name: "x", Slug: "x"},
		Dependencies:  []string{"kubectl", "kubectl"},
		Steps:         []Step{{Name: "s", Run: "echo"}},
	}
	if err := validateSemantics(s); err == nil {
		t.Fatal("expected error for duplicate dependency names")
	}
}

func TestSchemaRejectsEmptyDependencyString(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"dependencies":[""],"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for empty dependency string")
	}
}

// --- YAML support tests ---

const validSpecYAML = `
schemaVersion: 1
kind: command
metadata:
  name: Deploy
  slug: deploy
  description: Deploy a service
args:
  positional:
    - name: service
  flags:
    - name: env
      short: e
      type: string
      default: staging
steps:
  - name: deploy
    run: echo deploying {{.args.service}} to {{.args.env}}
`

func TestParseValidYAML(t *testing.T) {
	s, err := Parse([]byte(validSpecYAML))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if s.SchemaVersion != 1 {
		t.Errorf("expected schemaVersion 1, got %d", s.SchemaVersion)
	}
	if s.Kind != "command" {
		t.Errorf("expected kind 'command', got %q", s.Kind)
	}
	if s.Metadata.Name != "Deploy" {
		t.Errorf("expected name 'Deploy', got %q", s.Metadata.Name)
	}
	if s.Metadata.Slug != "deploy" {
		t.Errorf("expected slug 'deploy', got %q", s.Metadata.Slug)
	}
	if len(s.Args.Positional) != 1 || s.Args.Positional[0].Name != "service" {
		t.Errorf("unexpected positional args: %+v", s.Args.Positional)
	}
	if len(s.Args.Flags) != 1 || s.Args.Flags[0].Name != "env" {
		t.Errorf("unexpected flags: %+v", s.Args.Flags)
	}
	if len(s.Steps) != 1 || s.Steps[0].Name != "deploy" {
		t.Errorf("unexpected steps: %+v", s.Steps)
	}
}

func TestParseYAMLWithComments(t *testing.T) {
	yaml := `
# This is a comment
schemaVersion: 1
kind: command
metadata:
  name: Test # inline comment
  slug: test
steps:
  - name: run
    run: echo hello
`
	s, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if s.Metadata.Name != "Test" {
		t.Errorf("expected name 'Test', got %q", s.Metadata.Name)
	}
}

func TestParseYAMLMultilineRun(t *testing.T) {
	yaml := `
schemaVersion: 1
kind: command
metadata:
  name: Multi
  slug: multi
steps:
  - name: run
    run: |
      echo line1
      echo line2
`
	s, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(s.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(s.Steps))
	}
	if s.Steps[0].Run != "echo line1\necho line2\n" {
		t.Errorf("unexpected run content: %q", s.Steps[0].Run)
	}
}

func TestToJSONFromYAML(t *testing.T) {
	result, err := ToJSON([]byte(validSpecYAML))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Result should be valid JSON
	var v any
	if err := json.Unmarshal(result, &v); err != nil {
		t.Fatalf("ToJSON result is not valid JSON: %v", err)
	}
}

func TestToJSONFromJSON(t *testing.T) {
	input := []byte(validSpecJSON)
	result, err := ToJSON(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Should pass through unchanged
	if string(result) != string(input) {
		t.Error("expected JSON to pass through unchanged")
	}
}

func TestValidateYAML(t *testing.T) {
	// Convert YAML to JSON first (as Parse does), then validate
	jsonData, err := ToJSON([]byte(validSpecYAML))
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if err := Validate(jsonData); err != nil {
		t.Fatalf("expected YAML-derived JSON to pass validation, got: %v", err)
	}
}

// --- YAML schema constraint tests ---

func TestYAMLSchemaRejectsRunAsArray(t *testing.T) {
	y := `
schemaVersion: 1
kind: command
metadata:
  name: Test
  slug: test
steps:
  - name: run
    run:
      - echo hello
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("expected error when run is an array in YAML")
	}
}

func TestYAMLSchemaRejectsEmptyRun(t *testing.T) {
	y := `
schemaVersion: 1
kind: command
metadata:
  name: Test
  slug: test
steps:
  - name: run
    run: ""
`
	_, err := Parse([]byte(y))
	if err == nil {
		t.Fatal("expected error for empty run string in YAML")
	}
}
