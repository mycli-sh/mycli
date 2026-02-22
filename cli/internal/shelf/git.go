package shelf

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const credentialHint = `If this is a private repository, configure credentials before retrying:
  HTTPS: git credential-osxkeychain, gh auth setup-git, or ~/.netrc
  SSH:   my shelf add git@github.com:user/repo.git`

// Clone clones a git repository to dest. If ref is non-empty, it checks out that branch/tag.
// Interactive prompts are suppressed; git must authenticate via pre-configured credentials
// (SSH agent, credential helpers, ~/.netrc, etc.). Fails with a helpful hint on auth errors.
func Clone(url, dest, ref string) error {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, dest)

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -oBatchMode=yes",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("git clone failed: %s: %w\n\n"+credentialHint, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Pull runs git pull in the given repo directory.
// Interactive prompts are suppressed; git must authenticate via pre-configured credentials.
func Pull(repoPath string) error {
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -oBatchMode=yes",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %s: %w\n\n"+credentialHint, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// HeadCommit returns the short HEAD commit hash for a repo.
func HeadCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// TagExists returns true if the given git tag exists in the repo.
func TagExists(repoDir, tag string) bool {
	cmd := exec.Command("git", "tag", "--list", tag)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == tag
}

// TagCommitHash returns the full commit hash for a given tag.
func TagCommitHash(repoDir, tag string) (string, error) {
	cmd := exec.Command("git", "rev-parse", tag+"^{commit}")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s failed: %w", tag, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ArchiveTag extracts the contents of a git tag to destDir using git archive.
func ArchiveTag(repoDir, tag, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}
	cmd := exec.Command("sh", "-c", fmt.Sprintf("git archive %s | tar -x -C %s", tag, destDir))
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git archive failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
