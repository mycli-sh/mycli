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

func assertValidSpec(t *testing.T, s *CommandSpec) {
	t.Helper()
	if s.SchemaVersion != 1 {
		t.Errorf("expected schemaVersion 1, got %d", s.SchemaVersion)
	}
	if s.Kind != "command" {
		t.Errorf("expected kind 'command', got %q", s.Kind)
	}
	if s.Metadata.Name != "Deploy" {
		t.Errorf("expected metadata.name 'Deploy', got %q", s.Metadata.Name)
	}
	if s.Metadata.Slug != "deploy" {
		t.Errorf("expected metadata.slug 'deploy', got %q", s.Metadata.Slug)
	}
	if s.Metadata.Description != "Deploy a service" {
		t.Errorf("expected metadata.description 'Deploy a service', got %q", s.Metadata.Description)
	}
	if len(s.Args.Positional) != 1 {
		t.Fatalf("expected 1 positional arg, got %d", len(s.Args.Positional))
	}
	if s.Args.Positional[0].Name != "service" {
		t.Errorf("expected positional arg name 'service', got %q", s.Args.Positional[0].Name)
	}
	if len(s.Args.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(s.Args.Flags))
	}
	if s.Args.Flags[0].Name != "env" {
		t.Errorf("expected flag name 'env', got %q", s.Args.Flags[0].Name)
	}
	if len(s.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(s.Steps))
	}
	if s.Steps[0].Name != "deploy" {
		t.Errorf("expected step name 'deploy', got %q", s.Steps[0].Name)
	}
}

func TestParseValidSpec(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"JSON", validSpecJSON},
		{"YAML", validSpecYAML},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Parse([]byte(tc.input))
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			assertValidSpec(t, s)
		})
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

func TestSchemaShellValidation(t *testing.T) {
	tests := []struct {
		shell   string
		wantErr bool
	}{
		{"/usr/local/evil", true},
		{"/bin/sh", false},
		{"/bin/bash", false},
		{"/bin/zsh", false},
		{"/usr/bin/env", false},
	}
	for _, tc := range tests {
		t.Run(tc.shell, func(t *testing.T) {
			j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"shell":"` + tc.shell + `"},"steps":[{"name":"s","run":"echo"}]}`
			err := Validate([]byte(j))
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for shell %q", tc.shell)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for shell %q: %v", tc.shell, err)
			}
		})
	}
}

func TestSchemaTimeoutValidation(t *testing.T) {
	tests := []struct {
		timeout string
		wantErr bool
	}{
		{"30", true},
		{"30s", false},
		{"5m", false},
		{"1h", false},
		{"500ms", false},
		{"0s", false},
	}
	for _, tc := range tests {
		t.Run(tc.timeout, func(t *testing.T) {
			j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"timeout":"` + tc.timeout + `"},"steps":[{"name":"s","run":"echo"}]}`
			err := Validate([]byte(j))
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for timeout %q", tc.timeout)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for timeout %q: %v", tc.timeout, err)
			}
		})
	}
}

func TestSchemaEnvVarValidation(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr bool
	}{
		{"invalid key with space", `{"invalid key":"v"}`, true},
		{"valid keys", `{"MY_VAR":"v","_foo":"v","PATH":"v"}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"defaults":{"env":` + tc.env + `},"steps":[{"name":"s","run":"echo"}]}`
			err := Validate([]byte(j))
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
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

func TestParseSpecAliases(t *testing.T) {
	t.Run("with aliases", func(t *testing.T) {
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
	})

	t.Run("without aliases", func(t *testing.T) {
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
	})
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

func TestParseSpecDependencies(t *testing.T) {
	t.Run("with dependencies", func(t *testing.T) {
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
	})

	t.Run("without dependencies", func(t *testing.T) {
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
	})
}

func TestSchemaRejectsDuplicateDependencies(t *testing.T) {
	j := `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"dependencies":["kubectl","kubectl"],"steps":[{"name":"s","run":"echo"}]}`
	if err := Validate([]byte(j)); err == nil {
		t.Fatal("expected error for duplicate dependencies (uniqueItems)")
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

// --- Argument validation constraint tests ---

func ptrInt(i int) *int    { return &i }
func ptrBool(b bool) *bool { return &b }

func argSpecJSON(args string) string {
	return `{"schemaVersion":1,"kind":"command","metadata":{"name":"x","slug":"x"},"args":` +
		args + `,"steps":[{"name":"s","run":"echo"}]}`
}

func TestSchemaAcceptsArgConstraints(t *testing.T) {
	args := `{"positional":[{"name":"mode","enum":["a","b"]}],` +
		`"flags":[{"name":"env","type":"string","enum":["staging","prod"],"default":"prod","minLength":1,"maxLength":10},` +
		`{"name":"port","type":"int","min":1,"max":65535,"default":8080}]}`
	if _, err := Parse([]byte(argSpecJSON(args))); err != nil {
		t.Fatalf("expected valid spec to parse, got: %v", err)
	}
}

func TestValidateArgConstraintsRejects(t *testing.T) {
	tests := []struct {
		name string
		args string
	}{
		{"bad flag pattern", `{"flags":[{"name":"f","pattern":"["}]}`},
		{"bad positional pattern", `{"positional":[{"name":"p","pattern":"["}]}`},
		{"min on string flag", `{"flags":[{"name":"f","type":"string","min":1}]}`},
		{"pattern on int flag", `{"flags":[{"name":"f","type":"int","pattern":"^x$"}]}`},
		{"enum on bool flag", `{"flags":[{"name":"f","type":"bool","enum":["a"]}]}`},
		{"int min>max", `{"flags":[{"name":"f","type":"int","min":5,"max":1}]}`},
		{"minLength>maxLength", `{"flags":[{"name":"f","minLength":5,"maxLength":1}]}`},
		{"flag default violates enum", `{"flags":[{"name":"f","enum":["a","b"],"default":"c"}]}`},
		{"flag default out of range", `{"flags":[{"name":"f","type":"int","min":1,"max":10,"default":50}]}`},
		{"positional default violates enum", `{"positional":[{"name":"p","enum":["a"],"default":"z"}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate([]byte(argSpecJSON(tc.args))); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestValidateArgValues(t *testing.T) {
	enumFlag := &CommandSpec{Args: Args{Flags: []FlagArg{{Name: "env", Enum: []string{"staging", "prod"}}}}}
	reqEnumFlag := &CommandSpec{Args: Args{Flags: []FlagArg{{Name: "env", Required: ptrBool(true), Enum: []string{"staging", "prod"}}}}}
	patternFlag := &CommandSpec{Args: Args{Flags: []FlagArg{{Name: "name", Pattern: "^[a-z]+$"}}}}
	lenFlag := &CommandSpec{Args: Args{Flags: []FlagArg{{Name: "name", MinLength: ptrInt(3), MaxLength: ptrInt(5)}}}}
	intFlag := &CommandSpec{Args: Args{Flags: []FlagArg{{Name: "port", Type: "int", Min: ptrInt(1), Max: ptrInt(65535)}}}}
	enumPos := &CommandSpec{Args: Args{Positional: []PositionalArg{{Name: "mode", Enum: []string{"fast", "slow"}}}}}
	optEnumPos := &CommandSpec{Args: Args{Positional: []PositionalArg{{Name: "mode", Required: ptrBool(false), Enum: []string{"fast"}}}}}

	tests := []struct {
		name    string
		spec    *CommandSpec
		values  map[string]any
		wantErr bool
	}{
		{"enum ok", enumFlag, map[string]any{"env": "prod"}, false},
		{"enum miss", enumFlag, map[string]any{"env": "dev"}, true},
		{"enum optional unset skipped", enumFlag, map[string]any{"env": ""}, false},
		{"enum required empty fails", reqEnumFlag, map[string]any{"env": ""}, true},
		{"pattern match", patternFlag, map[string]any{"name": "abc"}, false},
		{"pattern no match", patternFlag, map[string]any{"name": "Abc1"}, true},
		{"length ok", lenFlag, map[string]any{"name": "abcd"}, false},
		{"length too short", lenFlag, map[string]any{"name": "ab"}, true},
		{"length too long", lenFlag, map[string]any{"name": "abcdef"}, true},
		{"int ok", intFlag, map[string]any{"port": "8080"}, false},
		{"int not integer", intFlag, map[string]any{"port": "abc"}, true},
		{"int below min", intFlag, map[string]any{"port": "0"}, true},
		{"int above max", intFlag, map[string]any{"port": "70000"}, true},
		{"positional enum ok", enumPos, map[string]any{"mode": "fast"}, false},
		{"positional enum miss", enumPos, map[string]any{"mode": "x"}, true},
		{"positional optional unset skipped", optEnumPos, map[string]any{"mode": ""}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateArgValues(tc.spec, tc.values)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

func TestDefaultToString(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{float64(8080), "8080"},
		{float64(1000000), "1000000"},
		{float64(3.5), "3.5"},
		{"prod", "prod"},
		{true, "true"},
		{nil, ""},
	}
	for _, tc := range tests {
		if got := DefaultToString(tc.in); got != tc.want {
			t.Errorf("DefaultToString(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
