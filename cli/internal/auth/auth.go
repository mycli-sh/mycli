package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mycli.sh/cli/internal/config"

	"github.com/zalando/go-keyring"
)

const (
	// Keychain identifiers for stored JWT credentials, kept clearly mycli-branded
	// so the item is recognizable in the OS keychain.
	keyringService = "mycli"
	keyringUser    = "mycli-tokens"

	// Legacy identifiers, migrated to the current ones on first read so an
	// existing login survives the rename.
	legacyKeyringService = "my-cli"
	legacyKeyringUser    = "tokens"
)

// storeMu serializes access to the credential store (keyring / credentials.json)
// so a background refresh writing tokens can't tear a concurrent read from a
// command's request middleware.
var storeMu sync.Mutex

type Tokens struct {
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	ExpiresAt       time.Time `json:"expires_at"`
	LoggedInAt      time.Time `json:"logged_in_at,omitempty"`
	LastRefreshedAt time.Time `json:"last_refreshed_at,omitempty"`
}

type DeviceCodeResponse struct {
	DeviceCode string `json:"device_code"`
	ExpiresIn  int    `json:"expires_in"`
	Interval   int    `json:"interval"`
	EmailSent  bool   `json:"email_sent"`
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

	storeMu.Lock()
	defer storeMu.Unlock()

	// Try keyring first
	if err := keyring.Set(keyringService, keyringUser, string(data)); err == nil {
		_ = keyring.Delete(legacyKeyringService, legacyKeyringUser) // drop any pre-rename item
		return nil
	}

	// Fall back to file
	return saveTokensToFile(data)
}

// IsAPIToken returns true if the token uses the myc_ prefix (API token, not JWT).
func IsAPIToken(token string) bool {
	return len(token) >= 4 && token[:4] == "myc_"
}

func LoadTokens() (*Tokens, error) {
	// API token env var takes priority (for CI)
	if token := os.Getenv("MY_API_TOKEN"); token != "" {
		return &Tokens{AccessToken: token}, nil
	}

	// Environment variable override (for dev scripts and CI)
	if token := os.Getenv("MY_ACCESS_TOKEN"); token != "" {
		return &Tokens{AccessToken: token}, nil
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	// Try the keyring first.
	if data, err := keyring.Get(keyringService, keyringUser); err == nil {
		return unmarshalTokens(data)
	}

	// Migrate a legacy keychain item (pre-rename) if one is present.
	if data, err := keyring.Get(legacyKeyringService, legacyKeyringUser); err == nil {
		_ = keyring.Set(keyringService, keyringUser, data)
		_ = keyring.Delete(legacyKeyringService, legacyKeyringUser)
		return unmarshalTokens(data)
	}

	// Fall back to file
	return loadTokensFromFile()
}

func unmarshalTokens(data string) (*Tokens, error) {
	var t Tokens
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func ClearTokens() error {
	storeMu.Lock()
	defer storeMu.Unlock()

	// Try keyring (current + legacy)
	_ = keyring.Delete(keyringService, keyringUser)
	_ = keyring.Delete(legacyKeyringService, legacyKeyringUser)

	// Also try file
	path := filepath.Join(config.DefaultDir(), "credentials.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func IsLoggedIn() bool {
	if os.Getenv("MY_API_TOKEN") != "" {
		return true
	}
	if os.Getenv("MY_ACCESS_TOKEN") != "" {
		return true
	}
	tokens, err := LoadTokens()
	if err != nil {
		return false
	}
	// Consider logged in if access token is valid OR a refresh token exists
	// (the client will transparently refresh on 401)
	if tokens.AccessToken != "" && time.Now().Before(tokens.ExpiresAt) {
		return true
	}
	return tokens.RefreshToken != ""
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
