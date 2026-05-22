//go:build integration

package integration

import (
	"os"
	"strings"
	"testing"

	"mycli.sh/test/integration/harness"
)

func TestMain(m *testing.M) {
	code := m.Run()
	harness.Shutdown()
	os.Exit(code)
}

// TestHappyPath_PublishInstallRun exercises the three end-to-end flows the
// user cares about:
//  1. publish a library (server-side POST /v1/libraries/{slug}/releases)
//  2. install that library via the CLI
//  3. run a command from the installed library and assert deterministic stdout
func TestHappyPath_PublishInstallRun(t *testing.T) {
	h := harness.Start(t)

	const marker = "INTEGRATION_OK"

	// PUBLISH: alice publishes a library "mylib" with one command "greet".
	userID := h.SeedUser(t, "alice@test.local", "alice")
	token := h.IssueAPIToken(t, userID, "integration")
	h.PublishLibraryRelease(t, token, "mylib", "My Library", "Integration fixture", "v1.0.0",
		harness.EchoSpec("greet", marker),
	)

	// INSTALL: same user installs the library through the real CLI binary.
	home := t.TempDir()
	res := h.Run(t, home, token, "cli", "library", "install", "alice/mylib")
	if res.ExitCode != 0 {
		t.Fatalf("install exit=%d stdout=%q stderr=%q", res.ExitCode, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "Installed") {
		t.Errorf("install stdout missing \"Installed\": %q", res.Stdout)
	}

	// RUN: invoke the command and confirm the spec's echo lands in stdout.
	res = h.Run(t, home, token, "cli", "run", "greet")
	if res.ExitCode != 0 {
		t.Fatalf("run exit=%d stdout=%q stderr=%q", res.ExitCode, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, marker) {
		t.Errorf("run stdout missing %q: %q", marker, res.Stdout)
	}
}
