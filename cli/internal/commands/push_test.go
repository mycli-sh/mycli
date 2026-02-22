package commands

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDiscoverSpecFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a tree with spec and non-spec files
	for _, sub := range []string{"deploy", "build", "scripts"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}

	for _, entry := range []struct{ path, content string }{
		{filepath.Join(dir, "command.yaml"), "root"},
		{filepath.Join(dir, "deploy", "command.yml"), "deploy"},
		{filepath.Join(dir, "build", "command.json"), "build"},
		{filepath.Join(dir, "scripts", "run.sh"), "#!/bin/sh"},
		{filepath.Join(dir, "README.md"), "readme"},
	} {
		if err := os.WriteFile(entry.path, []byte(entry.content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	files, err := discoverSpecFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(files)
	expected := []string{
		filepath.Join(dir, "build", "command.json"),
		filepath.Join(dir, "command.yaml"),
		filepath.Join(dir, "deploy", "command.yml"),
	}

	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(files), files)
	}
	for i, f := range files {
		if f != expected[i] {
			t.Errorf("file[%d]: expected %s, got %s", i, expected[i], f)
		}
	}
}

func TestDiscoverSpecFilesEmpty(t *testing.T) {
	dir := t.TempDir()

	files, err := discoverSpecFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d: %v", len(files), files)
	}
}

func TestDirAndFileMutuallyExclusive(t *testing.T) {
	cmd := newPushCmd()
	cmd.SetArgs([]string{"--dir", ".", "--file", "foo.yaml"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --dir and --file are set")
	}
}
