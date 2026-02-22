package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"mycli.sh/cli/internal/config"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "my-cli"
	keyringUser    = "tokens"
)

type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	EmailSent       bool   `json:"email_sent"`
}

type TokenResponse struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	ExpiresIn     int    `json:"expires_in"`
	NeedsUsername bool   `json:"needs_username"`
}

func SaveTokens(tokens *Tokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return err
	}

	// Try keyring first
	if err := keyring.Set(keyringService, keyringUser, string(data)); err == nil {
		return nil
	}

	// Fall back to file
	return saveTokensToFile(data)
}

func LoadTokens() (*Tokens, error) {
	// Environment variable override (for dev scripts and CI)
	if token := os.Getenv("MY_ACCESS_TOKEN"); token != "" {
		return &Tokens{AccessToken: token}, nil
	}

	// Try keyring first
	data, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		var t Tokens
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			return nil, err
		}
		return &t, nil
	}

	// Fall back to file
	return loadTokensFromFile()
}

func ClearTokens() error {
	// Try keyring
	_ = keyring.Delete(keyringService, keyringUser)

	// Also try file
	path := filepath.Join(config.DefaultDir(), "credentials.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func IsLoggedIn() bool {
	if os.Getenv("MY_ACCESS_TOKEN") != "" {
		return true
	}
	tokens, err := LoadTokens()
	if err != nil {
		return false
	}
	return tokens.AccessToken != "" && time.Now().Before(tokens.ExpiresAt)
}

func saveTokensToFile(data []byte) error {
	dir := config.DefaultDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "credentials.json"), data, 0600)
}

func loadTokensFromFile() (*Tokens, error) {
	path := filepath.Join(config.DefaultDir(), "credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Tokens
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
