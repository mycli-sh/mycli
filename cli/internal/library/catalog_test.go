package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSpecValid(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "deploy.yaml")

	content := `schemaVersion: 1
kind: command
metadata:
  name: Deploy
  slug: deploy
  description: Deploy a service
steps:
  - name: deploy
    run: echo deploying
`
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := GetSpec(specPath)
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if s.Metadata.Slug != "deploy" {
		t.Errorf("expected slug 'deploy', got %q", s.Metadata.Slug)
	}
	if s.Metadata.Name != "Deploy" {
		t.Errorf("expected name 'Deploy', got %q", s.Metadata.Name)
	}
}

func TestGetSpecMissingFile(t *testing.T) {
	_, err := GetSpec("/nonexistent/path/spec.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGetSpecInvalidContent(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(specPath, []byte(`{"not": "a spec"}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := GetSpec(specPath)
	if err == nil {
		t.Fatal("expected error for invalid spec content")
	}
}
