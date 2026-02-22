package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const defaultAPIURL = "http://localhost:8080"

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
		APIURL: defaultAPIURL,
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
		cfg.APIURL = defaultAPIURL
	}
	return cfg, nil
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
