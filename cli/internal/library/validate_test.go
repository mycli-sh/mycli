package library

import (
	"strings"
	"testing"
)

func TestValidateManifestValid(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "Test Library",
		"libraries": {
			"ops": {"name": "Operations", "path": "ops"}
		}
	}`)
	if err := ValidateManifest(data); err != nil {
		t.Fatalf("expected valid manifest, got error: %v", err)
	}
}

func TestValidateManifestInvalidJSON(t *testing.T) {
	err := ValidateManifest([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected 'invalid JSON' in error, got: %v", err)
	}
}

func TestValidateManifestSchemaInvalid(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing required name",
			data: `{"schemaVersion": 1, "libraries": {"ops": {"name": "Ops", "path": "ops"}}}`,
		},
		{
			name: "wrong schema version",
			data: `{"schemaVersion": 99, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops"}}}`,
		},
		{
			name: "empty libraries",
			data: `{"schemaVersion": 1, "name": "Bad", "libraries": {}}`,
		},
		{
			name: "invalid library key",
			data: `{"schemaVersion": 1, "name": "Bad", "libraries": {"INVALID": {"name": "Ops", "path": "ops"}}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateManifest([]byte(tc.data))
			if err == nil {
				t.Fatal("expected schema validation error")
			}
			if !strings.Contains(err.Error(), "schema validation failed") {
				t.Errorf("expected 'schema validation failed' in error, got: %v", err)
			}
		})
	}
}

func TestValidateManifestSemanticsAliasSameAsKey(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "Bad",
		"libraries": {
			"ops": {"name": "Ops", "path": "ops", "aliases": ["ops"]}
		}
	}`)
	err := ValidateManifest(data)
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "cannot be the same as its own key") {
		t.Errorf("expected alias-same-as-key error, got: %v", err)
	}
}

func TestValidateManifestSemanticsAliasConflictsWithKey(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "Bad",
		"libraries": {
			"ops": {"name": "Ops", "path": "ops", "aliases": ["tools"]},
			"tools": {"name": "Tools", "path": "tools"}
		}
	}`)
	err := ValidateManifest(data)
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "conflicts with library key") {
		t.Errorf("expected alias-conflicts-with-key error, got: %v", err)
	}
}

func TestValidateManifestSemanticsDuplicateAlias(t *testing.T) {
	data := []byte(`{
		"schemaVersion": 1,
		"name": "Bad",
		"libraries": {
			"ops": {"name": "Ops", "path": "ops", "aliases": ["o"]},
			"tools": {"name": "Tools", "path": "tools", "aliases": ["o"]}
		}
	}`)
	err := ValidateManifest(data)
	if err == nil {
		t.Fatal("expected semantic validation error")
	}
	if !strings.Contains(err.Error(), "duplicate library alias") {
		t.Errorf("expected duplicate-alias error, got: %v", err)
	}
}

func TestValidateManifestSemanticsNoAliases(t *testing.T) {
	m := &Manifest{
		Libraries: map[string]LibraryDef{
			"ops":   {Name: "Ops", Path: "ops"},
			"tools": {Name: "Tools", Path: "tools"},
		},
	}
	if err := validateManifestSemantics(m); err != nil {
		t.Fatalf("expected no error for manifest without aliases, got: %v", err)
	}
}
