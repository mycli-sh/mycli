package shelf

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
			url:     "https://github.com/user/my-shelf.git",
			wantEnd: filepath.Join("github.com", "user", "my-shelf"),
		},
		{
			name:    "https URL without .git",
			url:     "https://github.com/user/my-shelf",
			wantEnd: filepath.Join("github.com", "user", "my-shelf"),
		},
		{
			name:    "SSH URL",
			url:     "git@github.com:user/my-shelf.git",
			wantEnd: filepath.Join("github.com", "user", "my-shelf"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RepoLocalPath(tc.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !contains(got, tc.wantEnd) {
				t.Errorf("expected path to end with %q, got %q", tc.wantEnd, got)
			}
		})
	}
}

func TestLoadSaveRegistry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Load should return empty registry when file doesn't exist
	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg.Shelves) != 0 {
		t.Errorf("expected empty shelves, got %d", len(reg.Shelves))
	}

	// Save and reload
	reg.Shelves = append(reg.Shelves, ShelfEntry{
		Name: "test-shelf",
		URL:  "https://github.com/user/test-shelf.git",
		Ref:  "main",
	})
	if err := SaveRegistry(reg); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	reg2, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(reg2.Shelves) != 1 {
		t.Fatalf("expected 1 shelf, got %d", len(reg2.Shelves))
	}
	if reg2.Shelves[0].Name != "test-shelf" {
		t.Errorf("expected shelf name 'test-shelf', got %q", reg2.Shelves[0].Name)
	}
}

func TestFindByName(t *testing.T) {
	reg := &ShelfRegistry{
		Shelves: []ShelfEntry{
			{Name: "alpha"},
			{Name: "beta"},
		},
	}

	if found := FindByName(reg, "alpha"); found == nil {
		t.Error("expected to find 'alpha'")
	}
	if found := FindByName(reg, "gamma"); found != nil {
		t.Error("expected nil for 'gamma'")
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()

	manifest := ShelfManifest{
		ShelfVersion: 1,
		Name:         "Test Shelf",
		Libraries: map[string]LibraryDef{
			"ops": {Name: "Operations", Description: "Ops commands", Path: "ops"},
		},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), data, 0644); err != nil {
		t.Fatalf("write shelf.json: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "Test Shelf" {
		t.Errorf("expected name 'Test Shelf', got %q", m.Name)
	}
	if len(m.Libraries) != 1 {
		t.Fatalf("expected 1 library, got %d", len(m.Libraries))
	}
	if _, ok := m.Libraries["ops"]; !ok {
		t.Error("expected 'ops' library")
	}
}

func TestLoadManifestRejectsInvalidVersion(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"shelfVersion": 99, "name": "Bad", "libraries": {"x": {"name": "X", "path": "x"}}}`
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid shelf version")
	}
}

func TestLoadManifestRejectsInvalidLibrarySlug(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"shelfVersion": 1, "name": "Bad", "libraries": {"INVALID": {"name": "X", "path": "x"}}}`
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid library slug")
	}
}

func TestLoadManifestRejectsMissingName(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"shelfVersion": 1, "libraries": {"ops": {"name": "Ops", "path": "ops"}}}`
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadManifestRejectsEmptyLibraries(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"shelfVersion": 1, "name": "Empty", "libraries": {}}`
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for empty libraries")
	}
}

func TestDiscoverSpecs(t *testing.T) {
	dir := t.TempDir()

	// Create library directory with a valid spec
	opsDir := filepath.Join(dir, "ops")
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opsDir, "deploy.json"), []byte(validSpecJSON), 0644); err != nil {
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
	if items[0].Slug != "deploy" {
		t.Errorf("expected slug 'deploy', got %q", items[0].Slug)
	}
	if items[0].Library != "ops" {
		t.Errorf("expected library 'ops', got %q", items[0].Library)
	}
	if items[0].Name != "Deploy" {
		t.Errorf("expected name 'Deploy', got %q", items[0].Name)
	}
}

func TestDiscoverSpecsSkipsMismatchedSlug(t *testing.T) {
	dir := t.TempDir()

	opsDir := filepath.Join(dir, "ops")
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// File named "wrong.json" but spec slug is "deploy"
	if err := os.WriteFile(filepath.Join(opsDir, "wrong.json"), []byte(validSpecJSON), 0644); err != nil {
		t.Fatal(err)
	}

	libDef := LibraryDef{Name: "Operations", Path: "ops"}
	items, err := DiscoverSpecs(dir, "ops", libDef)
	if err != nil {
		t.Fatalf("DiscoverSpecs: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items (mismatched slug), got %d", len(items))
	}
}

func TestDiscoverSpecsSkipsInvalidSpec(t *testing.T) {
	dir := t.TempDir()

	opsDir := filepath.Join(dir, "ops")
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Invalid spec JSON
	if err := os.WriteFile(filepath.Join(opsDir, "bad.json"), []byte(`{"not": "a spec"}`), 0644); err != nil {
		t.Fatal(err)
	}

	libDef := LibraryDef{Name: "Operations", Path: "ops"}
	items, err := DiscoverSpecs(dir, "ops", libDef)
	if err != nil {
		t.Fatalf("DiscoverSpecs: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items (invalid spec), got %d", len(items))
	}
}

func TestDiscoverSpecsSkipsDirectories(t *testing.T) {
	dir := t.TempDir()

	opsDir := filepath.Join(dir, "ops")
	if err := os.MkdirAll(filepath.Join(opsDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opsDir, "deploy.json"), []byte(validSpecJSON), 0644); err != nil {
		t.Fatal(err)
	}

	libDef := LibraryDef{Name: "Operations", Path: "ops"}
	items, err := DiscoverSpecs(dir, "ops", libDef)
	if err != nil {
		t.Fatalf("DiscoverSpecs: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (skipping subdir), got %d", len(items))
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

func TestDiscoverSpecsYAML(t *testing.T) {
	dir := t.TempDir()

	opsDir := filepath.Join(dir, "ops")
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opsDir, "deploy.yaml"), []byte(validSpecYAML), 0644); err != nil {
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
	if items[0].Slug != "deploy" {
		t.Errorf("expected slug 'deploy', got %q", items[0].Slug)
	}
	if items[0].Library != "ops" {
		t.Errorf("expected library 'ops', got %q", items[0].Library)
	}
}

func TestDiscoverSpecsYMLExtension(t *testing.T) {
	dir := t.TempDir()

	opsDir := filepath.Join(dir, "ops")
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opsDir, "deploy.yml"), []byte(validSpecYAML), 0644); err != nil {
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
	if items[0].Slug != "deploy" {
		t.Errorf("expected slug 'deploy', got %q", items[0].Slug)
	}
}

func TestLoadManifestYAML(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `shelfVersion: 1
name: YAML Shelf
description: A shelf defined in YAML
libraries:
  ops:
    name: Operations
    description: Ops commands
    path: ops
`
	if err := os.WriteFile(filepath.Join(dir, "shelf.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write shelf.yaml: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "YAML Shelf" {
		t.Errorf("expected name 'YAML Shelf', got %q", m.Name)
	}
	if len(m.Libraries) != 1 {
		t.Fatalf("expected 1 library, got %d", len(m.Libraries))
	}
	if _, ok := m.Libraries["ops"]; !ok {
		t.Error("expected 'ops' library")
	}
}

func TestLoadManifestPrefersYAMLOverJSON(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `shelfVersion: 1
name: YAML Wins
libraries:
  ops:
    name: Operations
    path: ops
`
	jsonContent := `{"shelfVersion": 1, "name": "JSON Loses", "libraries": {"ops": {"name": "Operations", "path": "ops"}}}`

	if err := os.WriteFile(filepath.Join(dir, "shelf.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "YAML Wins" {
		t.Errorf("expected YAML to take precedence, got name %q", m.Name)
	}
}

func TestLoadManifestNoFile(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error when no manifest file exists")
	}
	if !strings.Contains(err.Error(), "no shelf manifest found") {
		t.Errorf("expected 'no shelf manifest found' in error, got: %v", err)
	}
}

// --- Library alias tests ---

func TestLoadManifestWithLibraryAliases(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `shelfVersion: 1
name: Alias Shelf
libraries:
  kubernetes:
    name: Kubernetes
    path: k8s
    aliases:
      - k
      - kube
`
	if err := os.WriteFile(filepath.Join(dir, "shelf.yaml"), []byte(yamlContent), 0644); err != nil {
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

func TestLoadManifestRejectsInvalidLibraryAlias(t *testing.T) {
	dir := t.TempDir()

	manifest := `{"shelfVersion": 1, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops", "aliases": ["INVALID"]}}}`
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid library alias pattern")
	}
}

func TestLoadManifestRejectsLibraryAliasSameAsKey(t *testing.T) {
	dir := t.TempDir()

	manifest := `{"shelfVersion": 1, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops", "aliases": ["ops"]}}}`
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for library alias same as its own key")
	}
}

func TestLoadManifestRejectsLibraryAliasConflictsWithOtherKey(t *testing.T) {
	dir := t.TempDir()

	manifest := `{"shelfVersion": 1, "name": "Bad", "libraries": {"ops": {"name": "Ops", "path": "ops", "aliases": ["tools"]}, "tools": {"name": "Tools", "path": "tools"}}}`
	if err := os.WriteFile(filepath.Join(dir, "shelf.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("expected error for library alias conflicting with another library key")
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

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
