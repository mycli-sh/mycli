package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSpecJSON = `{
  "schemaVersion": 1,
  "kind": "command",
  "metadata": {"name": "Deploy", "slug": "deploy", "description": "Deploy a service"},
  "args": {
    "positional": [{"name": "service"}]
  },
  "steps": [{"name": "deploy", "run": "echo deploying {{.args.service}}"}]
}`

func TestRepoLocalPath(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantEnd string
	}{
		{
			name:    "https URL with .git",
			url:     "https://github.com/user/my-library.git",
			wantEnd: filepath.Join("github.com", "user", "my-library"),
		},
		{
			name:    "https URL without .git",
			url:     "https://github.com/user/my-library",
			wantEnd: filepath.Join("github.com", "user", "my-library"),
		},
		{
			name:    "SSH URL",
			url:     "git@github.com:user/my-library.git",
			wantEnd: filepath.Join("github.com", "user", "my-library"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RepoLocalPath(tc.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !hasSuffix(got, tc.wantEnd) {
				t.Errorf("expected path to end with %q, got %q", tc.wantEnd, got)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()

	manifest := Manifest{
		Version: 1,
		Name:    "Test Library",
		Libraries: map[string]LibraryDef{
			"ops": {Name: "Operations", Description: "Ops commands", Path: "ops"},
		},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "mycli.json"), data, 0644); err != nil {
		t.Fatalf("write mycli.json: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "Test Library" {
		t.Errorf("expected name 'Test Library', got %q", m.Name)
	}
	if len(m.Libraries) != 1 {
		t.Fatalf("expected 1 library, got %d", len(m.Libraries))
	}
	if _, ok := m.Libraries["ops"]; !ok {
		t.Error("expected 'ops' library")
	}
}

func TestLoadManifestRejectsInvalid(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
	}{
		{
			name:     "invalid version",
			manifest: `{"schemaVersion": 99, "name": "Bad", "libraries": {"ops": {"name": "X", "path": "x"}}}`,
		},
		{
			name:     "invalid library slug",
			manifest: `{"schemaVersion": 1, "name": "Bad", "libraries": {"INVALID": {"name": "X", "path": "x"}}}`,
		},
		{
			name:     "missing name",
			manifest: `{"schemaVersion": 1, "libraries": {"ops": {"name": "Ops", "path": "ops"}}}`,
		},
		{
			name:     "empty libraries",
			manifest: `{"schemaVersion": 1, "name": "Empty", "libraries": {}}`,
		},
		{
			name:     "invalid library alias",
			manifest: `{"schemaVersion": 1, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops", "aliases": ["INVALID"]}}}`,
		},
		{
			name:     "library alias same as key",
			manifest: `{"schemaVersion": 1, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops", "aliases": ["ops"]}}}`,
		},
		{
			name:     "library alias conflicts with other key",
			manifest: `{"schemaVersion": 1, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops", "aliases": ["tools"]}, "tools": {"name": "Tools", "path": "tools"}}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "mycli.json"), []byte(tc.manifest), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadManifest(dir)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

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
steps:
  - name: deploy
    run: echo deploying {{.args.service}}
`

func TestDiscoverSpecs(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		content   string
		mkSubdir  bool
		wantCount int
		wantSlug  string
		wantLib   string
		wantName  string
	}{
		{
			name:      "valid JSON spec",
			filename:  "deploy.json",
			content:   validSpecJSON,
			wantCount: 1,
			wantSlug:  "deploy",
			wantLib:   "ops",
			wantName:  "Deploy",
		},
		{
			name:      "skips mismatched slug",
			filename:  "wrong.json",
			content:   validSpecJSON,
			wantCount: 0,
		},
		{
			name:      "skips invalid spec",
			filename:  "bad.json",
			content:   `{"not": "a spec"}`,
			wantCount: 0,
		},
		{
			name:      "skips directories",
			filename:  "deploy.json",
			content:   validSpecJSON,
			mkSubdir:  true,
			wantCount: 1,
			wantSlug:  "deploy",
		},
		{
			name:      "YAML extension",
			filename:  "deploy.yaml",
			content:   validSpecYAML,
			wantCount: 1,
			wantSlug:  "deploy",
			wantLib:   "ops",
		},
		{
			name:      "YML extension",
			filename:  "deploy.yml",
			content:   validSpecYAML,
			wantCount: 1,
			wantSlug:  "deploy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			opsDir := filepath.Join(dir, "ops")
			if err := os.MkdirAll(opsDir, 0755); err != nil {
				t.Fatal(err)
			}
			if tc.mkSubdir {
				if err := os.MkdirAll(filepath.Join(opsDir, "subdir"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(opsDir, tc.filename), []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			libDef := LibraryDef{Name: "Operations", Path: "ops"}
			items, err := DiscoverSpecs(dir, "ops", libDef)
			if err != nil {
				t.Fatalf("DiscoverSpecs: %v", err)
			}
			if len(items) != tc.wantCount {
				t.Fatalf("expected %d items, got %d", tc.wantCount, len(items))
			}
			if tc.wantCount == 0 {
				return
			}
			if tc.wantSlug != "" && items[0].Slug != tc.wantSlug {
				t.Errorf("expected slug %q, got %q", tc.wantSlug, items[0].Slug)
			}
			if tc.wantLib != "" && items[0].Library != tc.wantLib {
				t.Errorf("expected library %q, got %q", tc.wantLib, items[0].Library)
			}
			if tc.wantName != "" && items[0].Name != tc.wantName {
				t.Errorf("expected name %q, got %q", tc.wantName, items[0].Name)
			}
		})
	}
}

func TestLoadManifestYAML(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `schemaVersion: 1
name: YAML Library
description: A library defined in YAML
libraries:
  ops:
    name: Operations
    description: Ops commands
    path: ops
`
	if err := os.WriteFile(filepath.Join(dir, "mycli.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write mycli.yaml: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "YAML Library" {
		t.Errorf("expected name 'YAML Library', got %q", m.Name)
	}
	if len(m.Libraries) != 1 {
		t.Fatalf("expected 1 library, got %d", len(m.Libraries))
	}
	if _, ok := m.Libraries["ops"]; !ok {
		t.Error("expected 'ops' library")
	}
}

func TestLoadManifestNoFile(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error when no manifest file exists")
	}
	if !strings.Contains(err.Error(), "no library manifest found") {
		t.Errorf("expected 'no library manifest found' in error, got: %v", err)
	}
}

// --- Library alias tests ---

func TestLoadManifestWithLibraryAliases(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `schemaVersion: 1
name: Alias Library
libraries:
  kubernetes:
    name: Kubernetes
    path: k8s
    aliases:
      - k
      - kube
`
	if err := os.WriteFile(filepath.Join(dir, "mycli.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	lib := m.Libraries["kubernetes"]
	if len(lib.Aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(lib.Aliases))
	}
	if lib.Aliases[0] != "k" || lib.Aliases[1] != "kube" {
		t.Errorf("unexpected aliases: %v", lib.Aliases)
	}
}

func TestDiscoverSpecsPopulatesAliases(t *testing.T) {
	dir := t.TempDir()

	opsDir := filepath.Join(dir, "ops")
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		t.Fatal(err)
	}

	specWithAliases := `{
		"schemaVersion": 1,
		"kind": "command",
		"metadata": {"name": "Deploy", "slug": "deploy", "aliases": ["dep", "d"]},
		"steps": [{"name": "deploy", "run": "echo deploy"}]
	}`
	if err := os.WriteFile(filepath.Join(opsDir, "deploy.json"), []byte(specWithAliases), 0644); err != nil {
		t.Fatal(err)
	}

	libDef := LibraryDef{Name: "Operations", Path: "ops"}
	items, err := DiscoverSpecs(dir, "ops", libDef)
	if err != nil {
		t.Fatalf("DiscoverSpecs: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(items[0].Aliases))
	}
	if items[0].Aliases[0] != "dep" || items[0].Aliases[1] != "d" {
		t.Errorf("unexpected aliases: %v", items[0].Aliases)
	}
}

func TestValidateManifestRejectsUnknownFields(t *testing.T) {
	manifest := `{"schemaVersion": 1, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops"}}, "unknown": true}`
	err := ValidateManifest([]byte(manifest))
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

// helper
func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
