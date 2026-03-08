package library

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestRepo creates a bare git repo with one commit containing a test file,
// and returns a file:// URL suitable for cloning.
func initTestRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	bareDir := filepath.Join(tmp, "bare.git")
	workDir := filepath.Join(tmp, "work")

	gitRun(t, tmp, "git", "init", "--bare", bareDir)
	gitRun(t, tmp, "git", "clone", bareDir, workDir)
	gitRun(t, workDir, "git", "config", "user.email", "test@test.com")
	gitRun(t, workDir, "git", "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, workDir, "git", "add", ".")
	gitRun(t, workDir, "git", "commit", "-m", "initial commit")
	gitRun(t, workDir, "git", "push")

	return "file://" + bareDir
}

// initTestRepoWithTag creates a bare repo with a tagged commit.
func initTestRepoWithTag(t *testing.T, tag string) string {
	t.Helper()
	tmp := t.TempDir()

	bareDir := filepath.Join(tmp, "bare.git")
	workDir := filepath.Join(tmp, "work")

	gitRun(t, tmp, "git", "init", "--bare", bareDir)
	gitRun(t, tmp, "git", "clone", bareDir, workDir)
	gitRun(t, workDir, "git", "config", "user.email", "test@test.com")
	gitRun(t, workDir, "git", "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, workDir, "git", "add", ".")
	gitRun(t, workDir, "git", "commit", "-m", "initial commit")
	gitRun(t, workDir, "git", "tag", tag)
	gitRun(t, workDir, "git", "push", "--tags")
	gitRun(t, workDir, "git", "push")

	return "file://" + bareDir
}

func gitRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s: %v", name, args, out, err)
	}
}

func TestClone(t *testing.T) {
	repoURL := initTestRepo(t)
	dest := filepath.Join(t.TempDir(), "clone")

	if err := Clone(repoURL, dest, ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Verify file exists in clone
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); err != nil {
		t.Errorf("expected hello.txt in clone: %v", err)
	}
}

func TestCloneWithRef(t *testing.T) {
	tag := "v1.0.0"
	repoURL := initTestRepoWithTag(t, tag)
	dest := filepath.Join(t.TempDir(), "clone")

	if err := Clone(repoURL, dest, tag); err != nil {
		t.Fatalf("Clone with ref: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); err != nil {
		t.Errorf("expected hello.txt in clone: %v", err)
	}
}

func TestCloneInvalidURL(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "clone")

	err := Clone("file:///nonexistent/repo.git", dest, "")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}

	// Verify dest was cleaned up
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("expected dest to be removed after failed clone")
	}
}

func TestPull(t *testing.T) {
	repoURL := initTestRepo(t)
	dest := filepath.Join(t.TempDir(), "clone")

	if err := Clone(repoURL, dest, ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	if err := Pull(dest); err != nil {
		t.Fatalf("Pull: %v", err)
	}
}

func TestHeadCommit(t *testing.T) {
	repoURL := initTestRepo(t)
	dest := filepath.Join(t.TempDir(), "clone")

	if err := Clone(repoURL, dest, ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	hash, err := HeadCommit(dest)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty commit hash")
	}
	if len(hash) < 7 {
		t.Errorf("expected short hash of at least 7 chars, got %q", hash)
	}
}

func TestTagExists(t *testing.T) {
	tag := "v1.0.0"
	repoURL := initTestRepoWithTag(t, tag)
	dest := filepath.Join(t.TempDir(), "clone")

	if err := Clone(repoURL, dest, ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	// Fetch tags (shallow clone may not have them)
	gitRun(t, dest, "git", "fetch", "--tags")

	if !TagExists(dest, tag) {
		t.Errorf("expected tag %q to exist", tag)
	}
	if TagExists(dest, "v99.99.99") {
		t.Error("expected non-existent tag to return false")
	}
}

func TestTagCommitHash(t *testing.T) {
	tag := "v1.0.0"
	repoURL := initTestRepoWithTag(t, tag)
	dest := filepath.Join(t.TempDir(), "clone")

	if err := Clone(repoURL, dest, ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	gitRun(t, dest, "git", "fetch", "--tags")

	hash, err := TagCommitHash(dest, tag)
	if err != nil {
		t.Fatalf("TagCommitHash: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if len(hash) < 40 {
		t.Errorf("expected full hash (40 chars), got %q (%d chars)", hash, len(hash))
	}
}

func TestLatestSemverTag(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, "repo")

	gitRun(t, tmp, "git", "init", workDir)
	gitRun(t, workDir, "git", "config", "user.email", "test@test.com")
	gitRun(t, workDir, "git", "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "f.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, workDir, "git", "add", ".")
	gitRun(t, workDir, "git", "commit", "-m", "c1")

	// No tags yet
	tag, err := LatestSemverTag(workDir)
	if err != nil {
		t.Fatalf("LatestSemverTag (no tags): %v", err)
	}
	if tag != "" {
		t.Errorf("expected empty tag, got %q", tag)
	}

	// Add tags in non-sorted order
	gitRun(t, workDir, "git", "tag", "v0.0.1")
	gitRun(t, workDir, "git", "tag", "v0.0.10")
	gitRun(t, workDir, "git", "tag", "v0.1.0")
	gitRun(t, workDir, "git", "tag", "v0.0.2")
	gitRun(t, workDir, "git", "tag", "not-semver")

	tag, err = LatestSemverTag(workDir)
	if err != nil {
		t.Fatalf("LatestSemverTag: %v", err)
	}
	if tag != "v0.1.0" {
		t.Errorf("expected v0.1.0, got %q", tag)
	}
}

func TestCreateTag(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, "repo")

	gitRun(t, tmp, "git", "init", workDir)
	gitRun(t, workDir, "git", "config", "user.email", "test@test.com")
	gitRun(t, workDir, "git", "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "f.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, workDir, "git", "add", ".")
	gitRun(t, workDir, "git", "commit", "-m", "c1")

	if err := CreateTag(workDir, "v1.0.0"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	if !TagExists(workDir, "v1.0.0") {
		t.Error("expected tag v1.0.0 to exist after CreateTag")
	}

	// Creating duplicate tag should fail
	if err := CreateTag(workDir, "v1.0.0"); err == nil {
		t.Error("expected error when creating duplicate tag")
	}
}

func TestIsWorkingTreeClean(t *testing.T) {
	tmp := t.TempDir()
	workDir := filepath.Join(tmp, "repo")

	gitRun(t, tmp, "git", "init", workDir)
	gitRun(t, workDir, "git", "config", "user.email", "test@test.com")
	gitRun(t, workDir, "git", "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "f.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, workDir, "git", "add", ".")
	gitRun(t, workDir, "git", "commit", "-m", "c1")

	// Clean tree
	clean, err := IsWorkingTreeClean(workDir)
	if err != nil {
		t.Fatalf("IsWorkingTreeClean: %v", err)
	}
	if !clean {
		t.Error("expected clean working tree")
	}

	// Dirty tree
	if err := os.WriteFile(filepath.Join(workDir, "f.txt"), []byte("modified\n"), 0644); err != nil {
		t.Fatal(err)
	}
	clean, err = IsWorkingTreeClean(workDir)
	if err != nil {
		t.Fatalf("IsWorkingTreeClean: %v", err)
	}
	if clean {
		t.Error("expected dirty working tree")
	}
}

func TestArchiveTag(t *testing.T) {
	tag := "v1.0.0"
	repoURL := initTestRepoWithTag(t, tag)
	cloneDir := filepath.Join(t.TempDir(), "clone")

	// Clone with full history (archive needs the tag ref)
	gitRun(t, t.TempDir(), "git", "clone", repoURL, cloneDir)
	gitRun(t, cloneDir, "git", "fetch", "--tags")

	destDir := filepath.Join(t.TempDir(), "archive")
	if err := ArchiveTag(cloneDir, tag, destDir); err != nil {
		t.Fatalf("ArchiveTag: %v", err)
	}

	// Verify extracted file exists
	if _, err := os.Stat(filepath.Join(destDir, "hello.txt")); err != nil {
		t.Errorf("expected hello.txt in archive output: %v", err)
	}
}
