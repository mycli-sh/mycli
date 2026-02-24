package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// DefaultAPIURL can be set at build time via -ldflags to override the default.
// When empty (local dev builds), it falls back to http://localhost:8080.
var DefaultAPIURL string

func DefaultAPI() string {
	if DefaultAPIURL != "" {
		return DefaultAPIURL
	}
	return "http://localhost:8080"
}

type Config struct {
	APIURL           string `json:"api_url"`
	TelemetryEnabled bool   `json:"telemetry_enabled"`
}

func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".my")
}

func Load() (*Config, error) {
	dir := DefaultDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		APIURL: DefaultAPI(),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPI()
	}
	return cfg, nil
}

// DeviceID returns a persistent device identifier. It reads from ~/.my/device_id
// if the file exists; otherwise it generates a new UUID, writes it, and returns it.
func DeviceID() string {
	dir := DefaultDir()
	path := filepath.Join(dir, "device_id")
	data, err := os.ReadFile(path)
	if err == nil {
		if id := string(data); id != "" {
			return id
		}
	}
	id := uuid.New().String()
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(path, []byte(id), 0600)
	return id
}

func (c *Config) Save() error {
	dir := DefaultDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0600)
}
