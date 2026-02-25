package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"mycli.sh/cli/internal/library"
)

// initBareSourceRepo creates a bare git repo containing a valid mycli.yaml
// and one spec file, returning a file:// URL suitable for source add.
func initBareSourceRepo(t *testing.T) string {
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

	// Write mycli.yaml
	manifestYAML := `schemaVersion: 1
name: test-library
description: A test library
libraries:
  ops:
    name: Operations
    description: Ops commands
    path: ops
`
	if err := os.WriteFile(filepath.Join(workDir, "mycli.yaml"), []byte(manifestYAML), 0644); err != nil {
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

// --- source add tests ---

func TestSourceAddGitFromLocalRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	repoURL := initBareSourceRepo(t)

	cmd := newSourceCmd()
	cmd.SetArgs([]string{"add", repoURL})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify registry has 1 entry with the manifest name
	reg, err := library.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(reg.Sources))
	}
	if reg.Sources[0].Name != "test-library" {
		t.Fatalf("expected name %q, got %q", "test-library", reg.Sources[0].Name)
	}
	if reg.Sources[0].Kind != "git" {
		t.Fatalf("expected kind %q, got %q", "git", reg.Sources[0].Kind)
	}
}

func TestSourceAddGitDuplicateNameExplicit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{
		{Name: "my-lib", Slug: "my-lib", Kind: "git"},
	})

	cmd := newSourceCmd()
	// Use a dummy URL; the fast-fail on --name should trigger before clone
	cmd.SetArgs([]string{"add", "--name", "my-lib", "https://example.com/repo.git"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when adding duplicate source name")
	}
}

// --- source remove tests ---

func TestSourceRemoveGitSuccess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{
		{Name: "test-lib", Slug: "test-lib", Kind: "git", LocalPath: "/nonexistent/path"},
	})

	cmd := newSourceCmd()
	cmd.SetArgs([]string{"remove", "test-lib"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify registry now has 0 entries
	reg, err := library.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Sources) != 0 {
		t.Fatalf("expected 0 sources after remove, got %d", len(reg.Sources))
	}
}

func TestSourceRemoveNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{})

	cmd := newSourceCmd()
	cmd.SetArgs([]string{"remove", "nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when removing nonexistent source")
	}
}

// --- source update tests ---

func TestSourceUpdateNoSources(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := newSourceCmd()
	cmd.SetArgs([]string{"update"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSourceUpdateNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{
		{Name: "other", Slug: "other", Kind: "git"},
	})

	cmd := newSourceCmd()
	cmd.SetArgs([]string{"update", "nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when updating nonexistent source")
	}
}

// --- source list tests ---

func TestSourceListEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := newSourceCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSourceListWithEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeSourceRegistry(t, []library.SourceEntry{
		{Name: "test-lib", Slug: "test-lib", Kind: "git", GitURL: "https://example.com/repo.git"},
	})

	cmd := newSourceCmd()
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
