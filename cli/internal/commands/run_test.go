package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFromFile(t *testing.T) {
	dir := t.TempDir()
	specContent := `schemaVersion: 1
kind: command
metadata:
  name: test
  slug: test
  description: A test command
args:
  positional: []
  flags: []
steps:
  - name: run
    run: echo hello
`
	specPath := filepath.Join(dir, "command.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newRunCmd()
	cmd.SetArgs([]string{"-f", specPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNoArgsNoFile(t *testing.T) {
	cmd := newRunCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args and no --file")
	}
	// In non-TTY (test env), should suggest providing a slug argument
	if !strings.Contains(err.Error(), "slug") {
		t.Fatalf("expected error about slug argument, got: %v", err)
	}
}

func TestRunFromFileDot(t *testing.T) {
	dir := t.TempDir()
	specContent := `schemaVersion: 1
kind: command
metadata:
  name: dottest
  slug: dottest
  description: A dot test command
args:
  positional: []
  flags: []
steps:
  - name: run
    run: echo dot-test
`
	specPath := filepath.Join(dir, "command.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0600); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cmd := newRunCmd()
	cmd.SetArgs([]string{"-f", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDetectLocalSpecFile(t *testing.T) {
	// Test with no spec file
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if got := detectLocalSpecFile(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	// Test with command.yaml
	if err := os.WriteFile(filepath.Join(dir, "command.yaml"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := detectLocalSpecFile(); got != "command.yaml" {
		t.Fatalf("expected command.yaml, got %q", got)
	}

	// Test with command.yml (remove yaml first)
	_ = os.Remove(filepath.Join(dir, "command.yaml"))
	if err := os.WriteFile(filepath.Join(dir, "command.yml"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := detectLocalSpecFile(); got != "command.yml" {
		t.Fatalf("expected command.yml, got %q", got)
	}
}
