package commands

import (
	"os"
	"path/filepath"
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
}
