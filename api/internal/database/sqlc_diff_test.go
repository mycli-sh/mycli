package database

import (
	"os/exec"
	"testing"
)

// TestSqlcGeneratedFilesUpToDate fails if the committed files under
// api/internal/database/generated/ are out of sync with the SQL queries and
// migrations they were generated from. Run `make sqlc-generate` to regenerate.
//
// Skipped automatically when the sqlc binary is not installed locally; CI
// must install sqlc to enforce this check.
func TestSqlcGeneratedFilesUpToDate(t *testing.T) {
	if _, err := exec.LookPath("sqlc"); err != nil {
		t.Skip("sqlc binary not found in PATH; install sqlc to run this test")
	}

	cmd := exec.Command("sqlc", "diff", "-f", "sqlc/sqlc.yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sqlc diff reported drift; run `make sqlc-generate` and commit the changes.\n%s", out)
	}
}
