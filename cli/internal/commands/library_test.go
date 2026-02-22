package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"mycli.sh/cli/internal/library"
)

// initBareShelfRepo creates a bare git repo containing a valid shelf.yaml
// and one spec file, returning a file:// URL suitable for library add.
func initBareShelfRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	bareDir := filepath.Join(tmp, "bare.git")
	workDir := filepath.Join(tmp, "work")

	// Create bare repo
	run(t, tmp, "git", "init", "--bare", bareDir)

	// Clone it to a working copy
	run(t, tmp, "git", "clone", bareDir, workDir)

	// Configure git user for the working copy
	run(t, workDir, "git", "config", "user.email", "test@test.com")
	run(t, workDir, "git", "config", "user.name", "Test")

	// Write shelf.yaml
	shelfYAML := `shelfVersion: 1
name: test-shelf
description: A test shelf
libraries:
  ops:
    name: Operations
    description: Ops commands
    path: ops
`
	if err := os.WriteFile(filepath.Join(workDir, "shelf.yaml"), []byte(shelfYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Write ops/hello.yaml (minimal valid spec)
	opsDir := filepath.Join(workDir, "ops")
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `schemaVersion: 1
kind: command
metadata:
  name: Hello
  slug: hello
  description: A hello command
steps:
  - name: greet
    run: echo hello
`
	if err := os.WriteFile(filepath.Join(opsDir, "hello.yaml"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit and push
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "init")
	run(t, workDir, "git", "push")

	return "file://" + bareDir
}

// run executes a command in the given directory, failing the test on error.
func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s: %v", name, args, out, err)
	}
}

// writeLibraryRegistry writes a libraries.json file with the given entries.
func writeLibraryRegistry(t *testing.T, entries []library.LibraryEntry) {
	t.Helper()
	reg := library.LibraryRegistry{Libraries: entries}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dir := library.LibrariesDir()
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

	writeLibraryRegistry(t, []library.LibraryEntry{
		{Name: "kubernetes", Owner: "system", Slug: "kubernetes", Source: "registry"},
	})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLibraryListJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeLibraryRegistry(t, []library.LibraryEntry{
		{Name: "kubernetes", Owner: "system", Slug: "kubernetes", Source: "registry"},
	})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- library remove tests ---

func TestLibraryRemoveNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeLibraryRegistry(t, []library.LibraryEntry{})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"remove", "nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when removing nonexistent library")
	}
}

func TestLibraryRemoveGitSuccess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeLibraryRegistry(t, []library.LibraryEntry{
		{Name: "test-lib", Slug: "test-lib", Source: "git", LocalPath: "/nonexistent/path"},
	})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"remove", "test-lib"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify registry now has 0 entries
	reg, err := library.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Libraries) != 0 {
		t.Fatalf("expected 0 libraries after remove, got %d", len(reg.Libraries))
	}
}

// --- library add git tests ---

func TestLibraryAddGitFromLocalRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	repoURL := initBareShelfRepo(t)

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"add", repoURL})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify registry has 1 entry with the manifest name
	reg, err := library.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Libraries) != 1 {
		t.Fatalf("expected 1 library, got %d", len(reg.Libraries))
	}
	if reg.Libraries[0].Name != "test-shelf" {
		t.Fatalf("expected name %q, got %q", "test-shelf", reg.Libraries[0].Name)
	}
	if reg.Libraries[0].Source != "git" {
		t.Fatalf("expected source %q, got %q", "git", reg.Libraries[0].Source)
	}
}

func TestLibraryAddGitDuplicateNameExplicit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeLibraryRegistry(t, []library.LibraryEntry{
		{Name: "my-lib", Slug: "my-lib", Source: "git"},
	})

	cmd := newLibraryCmd()
	// Use a dummy URL; the fast-fail on --name should trigger before clone
	cmd.SetArgs([]string{"add", "--name", "my-lib", "https://example.com/repo.git"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when adding duplicate library name")
	}
}

// --- library update tests ---

func TestLibraryUpdateNoLibraries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"update"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLibraryUpdateNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeLibraryRegistry(t, []library.LibraryEntry{
		{Name: "other", Slug: "other", Source: "git"},
	})

	cmd := newLibraryCmd()
	cmd.SetArgs([]string{"update", "nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when updating nonexistent library")
	}
}
