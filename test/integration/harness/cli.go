//go:build integration

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the repository root, derived from this
// source file's location (test/integration/harness → ../../..).
func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("expected go.mod at %s: %w", root, err)
	}
	return root, nil
}

// buildCLI compiles ./cli/cmd/my and returns the path to the resulting binary.
// The binary is placed in os.TempDir() so each test session uses a fresh build.
func buildCLI(ctx context.Context) (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}

	out := filepath.Join(os.TempDir(), fmt.Sprintf("my-integration-%d", os.Getpid()))
	cmd := exec.CommandContext(ctx, "go", "build",
		"-ldflags", "-X mycli.sh/cli/internal/client.Version=integration",
		"-o", out,
		"./cli/cmd/my",
	)
	cmd.Dir = root
	out2, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build cli: %w\n%s", err, string(out2))
	}
	return out, nil
}

// CLIResult is the outcome of a CLI invocation.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run shells out to the compiled CLI with an isolated HOME and the given API
// token set on MY_API_TOKEN. A config.json pointing at the test API URL is
// pre-written so the CLI doesn't fall back to the production default.
//
// Subsequent calls with the same `home` reuse its state (useful when chaining
// install → run within one test).
func (h *Harness) Run(t *testing.T, home, token string, args ...string) CLIResult {
	t.Helper()

	if home == "" {
		home = t.TempDir()
	}
	if err := writeConfig(home, h.APIURL); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	cmd := exec.Command(h.cliPath, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"MY_API_TOKEN="+token,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("exec %s %s: %v", h.cliPath, strings.Join(args, " "), err)
		}
	}

	return CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// writeConfig pre-seeds ~/.my/config.json with the test API URL.
func writeConfig(home, apiURL string) error {
	dir := filepath.Join(home, ".my")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	cfg := map[string]any{"api_url": apiURL}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0600)
}
