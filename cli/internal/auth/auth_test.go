package auth

import (
	"testing"

	"github.com/zalando/go-keyring"
)

// TestLoadTokens_MigratesLegacyKeychainItem verifies that credentials stored
// under the old keychain identifiers are transparently migrated to the current
// ones, so an existing login survives the rename.
func TestLoadTokens_MigratesLegacyKeychainItem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = ClearTokens()

	if err := keyring.Set(legacyKeyringService, legacyKeyringUser, `{"access_token":"A","refresh_token":"R"}`); err != nil {
		t.Fatalf("seed legacy item: %v", err)
	}

	got, err := LoadTokens()
	if err != nil {
		t.Fatalf("LoadTokens: %v", err)
	}
	if got.AccessToken != "A" || got.RefreshToken != "R" {
		t.Fatalf("migrated tokens = %+v, want access=A refresh=R", got)
	}

	// The data now lives under the current key; the legacy key is gone.
	if _, err := keyring.Get(keyringService, keyringUser); err != nil {
		t.Errorf("expected tokens under current key after migration: %v", err)
	}
	if _, err := keyring.Get(legacyKeyringService, legacyKeyringUser); err == nil {
		t.Error("expected legacy key to be deleted after migration")
	}
}
