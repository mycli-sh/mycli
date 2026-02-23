package commands

import (
	"testing"

	"mycli.sh/cli/internal/config"
)

func TestSetAPIURLSetsCustomURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := newSetAPIURLCmd()
	cmd.SetArgs([]string{"https://custom.example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != "https://custom.example.com" {
		t.Fatalf("expected https://custom.example.com, got %s", cfg.APIURL)
	}
}

func TestSetAPIURLReset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Set a custom URL first
	cmd := newSetAPIURLCmd()
	cmd.SetArgs([]string{"https://custom.example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error setting URL: %v", err)
	}

	// Reset to default
	cmd = newSetAPIURLCmd()
	cmd.SetArgs([]string{"--reset"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error resetting: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIURL != config.DefaultAPI() {
		t.Fatalf("expected %s, got %s", config.DefaultAPI(), cfg.APIURL)
	}
}

func TestSetAPIURLErrorOnNoArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := newSetAPIURLCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args and no --reset")
	}
}
