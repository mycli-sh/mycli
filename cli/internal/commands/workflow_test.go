package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func init() {
	keyring.MockInit()
}

func TestInitPushRunWorkflow(t *testing.T) {
	// Step 1: Isolate environment
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MY_ACCESS_TOKEN", "test-token")

	// Step 2: Mock API server
	var capturedSpecJSON json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/commands":
			// Command lookup by slug — return empty (new command)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"commands":[]}`))

		case r.Method == "POST" && r.URL.Path == "/v1/commands":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"test-cmd-id","name":"deploy","slug":"deploy"}`))

		case r.Method == "POST" && r.URL.Path == "/v1/commands/test-cmd-id/versions":
			// Capture the spec_json from the request body
			var body struct {
				SpecJSON json.RawMessage `json:"spec_json"`
				Message  string          `json:"message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("failed to decode version request body: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			capturedSpecJSON = body.SpecJSON

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"test-ver-id","version":1,"spec_hash":"abc123"}`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Step 3: Write config.json with mock server URL
	myDir := filepath.Join(home, ".my")
	if err := os.MkdirAll(myDir, 0700); err != nil {
		t.Fatal(err)
	}
	configJSON := fmt.Sprintf(`{"api_url":%q}`, srv.URL)
	if err := os.WriteFile(filepath.Join(myDir, "config.json"), []byte(configJSON), 0600); err != nil {
		t.Fatal(err)
	}

	// Step 4: Run init deploy
	workDir := t.TempDir()
	chdir(t, workDir)

	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"deploy"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init deploy failed: %v", err)
	}

	specPath := filepath.Join(workDir, "deploy", "command.yaml")
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("deploy/command.yaml not created: %v", err)
	}

	// Step 5: Overwrite spec with a positional arg for meaningful run test
	specYAML := `schemaVersion: 1
kind: command
metadata:
  name: deploy
  slug: deploy
  description: Deploy to an environment
args:
  positional:
    - name: environment
      description: Target environment
steps:
  - name: deploy
    run: echo "deploying to {{.args.environment}}"
`
	if err := os.WriteFile(specPath, []byte(specYAML), 0600); err != nil {
		t.Fatal(err)
	}

	// Step 6: Push the spec file
	pushCmd := newPushCmd()
	pushCmd.SetArgs([]string{"-f", specPath})
	if err := pushCmd.Execute(); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	if capturedSpecJSON == nil {
		t.Fatal("mock server did not capture spec_json from push")
	}

	// Step 7: Seed cache for run-by-slug
	cacheDir := filepath.Join(home, ".my", "cache")
	specsDir := filepath.Join(cacheDir, "specs", "test-cmd-id")
	if err := os.MkdirAll(specsDir, 0700); err != nil {
		t.Fatal(err)
	}

	catalog := map[string]any{
		"items": []map[string]any{
			{
				"command_id": "test-cmd-id",
				"slug":       "deploy",
				"name":       "deploy",
				"version":    1,
			},
		},
		"etag":      "test-etag",
		"synced_at": "2025-01-01T00:00:00Z",
	}
	catalogData, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "catalog.json"), catalogData, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "1.json"), capturedSpecJSON, 0600); err != nil {
		t.Fatal(err)
	}

	// Step 8: Run deploy production
	runCmd := newRunCmd()
	runCmd.SetArgs([]string{"deploy", "production"})
	if err := runCmd.Execute(); err != nil {
		t.Fatalf("run deploy production failed: %v", err)
	}

	// Step 9: Verify history
	historyPath := filepath.Join(home, ".my", "history.jsonl")
	hf, err := os.Open(historyPath)
	if err != nil {
		t.Fatalf("failed to open history file: %v", err)
	}
	defer hf.Close()

	var found bool
	scanner := bufio.NewScanner(hf)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry["slug"] == "deploy" {
			if code, ok := entry["exit_code"].(float64); ok && int(code) == 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("history.jsonl does not contain deploy entry with exit_code 0")
	}
}
