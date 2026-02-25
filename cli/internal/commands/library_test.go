package commands

import (
	"encoding/json"
	"os"
	"testing"

	"mycli.sh/cli/internal/library"
)

// writeSourceRegistry writes a sources.json file with the given entries.
func writeSourceRegistry(t *testing.T, entries []library.SourceEntry) {
	t.Helper()
	reg := library.SourceRegistry{Sources: entries}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dir := library.SourcesDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(library.RegistryPath(), data, 0600); err != nil {
		t.Fatal(err)
	}
}

// --- library list tests ---

func TestLibraryListEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLibraryListWithEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{
		{Name: "kubernetes", Owner: "system", Slug: "kubernetes", Kind: "registry"},
	})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLibraryListJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{
		{Name: "kubernetes", Owner: "system", Slug: "kubernetes", Kind: "registry"},
	})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- library uninstall tests ---

func TestLibraryUninstallNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"uninstall", "nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when uninstalling nonexistent library")
	}
}

func TestLibraryUninstallGitRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{
		{Name: "test-lib", Slug: "test-lib", Kind: "git", LocalPath: "/nonexistent/path"},
	})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"uninstall", "test-lib"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when uninstalling a git source via library uninstall")
	}
}
