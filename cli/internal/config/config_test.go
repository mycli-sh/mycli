package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAPIFallsBackToLocalhost(t *testing.T) {
	orig := DefaultAPIURL
	t.Cleanup(func() { DefaultAPIURL = orig })

	DefaultAPIURL = ""
	if got := DefaultAPI(); got != "http://localhost:8080" {
		t.Fatalf("expected http://localhost:8080, got %s", got)
	}
}

func TestDefaultAPIReturnsOverride(t *testing.T) {
	orig := DefaultAPIURL
	t.Cleanup(func() { DefaultAPIURL = orig })

	DefaultAPIURL = "https://api.example.com"
	if got := DefaultAPI(); got != "https://api.example.com" {
		t.Fatalf("expected https://api.example.com, got %s", got)
	}
}

func TestLoadReturnsDefaultWhenNoFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIURL != DefaultAPI() {
		t.Fatalf("expected %s, got %s", DefaultAPI(), cfg.APIURL)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &Config{APIURL: "https://custom.example.com"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.APIURL != "https://custom.example.com" {
		t.Fatalf("expected https://custom.example.com, got %s", loaded.APIURL)
	}
}

func TestLoadWithEmptyAPIURLFallsBackToDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := DefaultDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{"api_url": ""})
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIURL != DefaultAPI() {
		t.Fatalf("expected %s, got %s", DefaultAPI(), cfg.APIURL)
	}
}
