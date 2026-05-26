package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
	"mycli.sh/pkg/spec"
)

var (
	validID      = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	validProfile = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

type CachedCatalog struct {
	Items    []client.CatalogItem `json:"items"`
	ETag     string               `json:"etag"`
	SyncedAt time.Time            `json:"synced_at"`
}

func cacheDir() string {
	return filepath.Join(config.DefaultDir(), "cache")
}

func profilesDir() string {
	return filepath.Join(cacheDir(), "profiles")
}

func profileCatalogPath(profile string) (string, error) {
	if !validProfile.MatchString(profile) {
		return "", fmt.Errorf("invalid profile name: %q", profile)
	}
	return filepath.Join(profilesDir(), profile, "catalog.json"), nil
}

func specDir() string {
	return filepath.Join(cacheDir(), "specs")
}

// migrateLegacyCache moves a pre-profile ~/.my/cache/catalog.json into the
// "default" profile slot. One-shot; subsequent calls are no-ops.
func migrateLegacyCache() {
	legacy := filepath.Join(cacheDir(), "catalog.json")
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	target, err := profileCatalogPath(config.DefaultProfileSlug)
	if err != nil {
		return
	}
	if _, err := os.Stat(target); err == nil {
		// New layout already populated — drop the legacy file
		_ = os.Remove(legacy)
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return
	}
	if err := os.Rename(legacy, target); err != nil {
		// Best-effort: copy if rename fails across filesystems
		data, rerr := os.ReadFile(legacy)
		if rerr != nil {
			return
		}
		if werr := os.WriteFile(target, data, 0600); werr != nil {
			return
		}
		_ = os.Remove(legacy)
	}
}

// SyncProfile syncs the catalog scoped to the given profile.
func SyncProfile(c *client.Client, profile string, force bool) (int, error) {
	if profile == "" {
		return 0, fmt.Errorf("profile is required")
	}

	current, _ := loadCatalog(profile)
	etag := ""
	if !force && current != nil {
		etag = current.ETag
	}

	resp, err := c.GetCatalog(etag, profile)
	if err != nil {
		return 0, fmt.Errorf("fetch catalog: %w", err)
	}

	// Determine which items to check for missing specs
	var items []client.CatalogItem
	if resp != nil {
		items = resp.Items
	} else if current != nil {
		items = current.Items
	}

	// Fetch any specs not yet cached
	fetched := 0
	for _, item := range items {
		existing, err := loadSpecFile(item.CommandID, item.Version)
		if err == nil && existing != nil {
			continue
		}

		specJSON, err := c.GetVersionSpec(item.CommandID, item.Version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to fetch spec for %s: %v\n", item.Slug, err)
			continue
		}

		if err := saveSpecFile(item.CommandID, item.Version, specJSON); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to cache spec for %s: %v\n", item.Slug, err)
			continue
		}
		fetched++
	}

	// Save catalog only when it's new
	if resp != nil {
		cached := &CachedCatalog{
			Items:    resp.Items,
			ETag:     resp.ETag,
			SyncedAt: time.Now(),
		}
		if err := saveCatalog(profile, cached); err != nil {
			return 0, fmt.Errorf("save catalog: %w", err)
		}
	}

	return fetched, nil
}

func GetSpec(profile, slug string) (*spec.CommandSpec, error) {
	catalog, err := loadCatalog(profile)
	if err != nil {
		return nil, fmt.Errorf("no cached catalog for profile %q (run 'my cli profile sync %s' first): %w", profile, profile, err)
	}

	for _, item := range catalog.Items {
		if item.Slug == slug && item.Library == "" {
			data, err := loadSpecFile(item.CommandID, item.Version)
			if err != nil {
				return nil, fmt.Errorf("spec not cached (run 'my cli profile sync %s'): %w", profile, err)
			}
			return spec.Parse(data)
		}
	}

	return nil, fmt.Errorf("command %q not found in profile %q", slug, profile)
}

func GetLibrarySpec(profile, libraryKey, slug string) (*spec.CommandSpec, error) {
	catalog, err := loadCatalog(profile)
	if err != nil {
		return nil, fmt.Errorf("no cached catalog for profile %q (run 'my cli profile sync %s' first): %w", profile, profile, err)
	}

	for _, item := range catalog.Items {
		if matchLibraryKey(item, libraryKey) && item.Slug == slug {
			data, err := loadSpecFile(item.CommandID, item.Version)
			if err != nil {
				return nil, fmt.Errorf("spec not cached (run 'my cli profile sync %s'): %w", profile, err)
			}
			return spec.Parse(data)
		}
	}

	return nil, fmt.Errorf("command %q not found in library %q for profile %q", slug, libraryKey, profile)
}

func GetLibraryCatalogItem(profile, libraryKey, slug string) (*client.CatalogItem, error) {
	catalog, err := loadCatalog(profile)
	if err != nil {
		return nil, err
	}
	for _, item := range catalog.Items {
		if matchLibraryKey(item, libraryKey) && item.Slug == slug {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("command %q not found in library %q", slug, libraryKey)
}

// matchLibraryKey matches a catalog item against a library key.
// The key can be "owner/slug" (matches on LibraryOwner+Library) or just "slug" (matches on Library).
func matchLibraryKey(item client.CatalogItem, key string) bool {
	if item.Library == "" {
		return false
	}
	if strings.Contains(key, "/") {
		parts := strings.SplitN(key, "/", 2)
		return item.LibraryOwner == parts[0] && item.Library == parts[1]
	}
	return item.Library == key
}

func GetCatalog(profile string) (*CachedCatalog, error) {
	return loadCatalog(profile)
}

// HasCachedProfile reports whether a profile's catalog file exists on disk.
// Used by offline-tolerant commands to decide whether to allow operations
// without a network round-trip.
func HasCachedProfile(profile string) bool {
	path, err := profileCatalogPath(profile)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// ListCachedProfiles returns the slugs of profiles that have a local catalog
// cached. Used as a fallback when `my cli profile list` is offline.
func ListCachedProfiles() []string {
	migrateLegacyCache()
	entries, err := os.ReadDir(profilesDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !validProfile.MatchString(name) {
			continue
		}
		if _, err := os.Stat(filepath.Join(profilesDir(), name, "catalog.json")); err == nil {
			out = append(out, name)
		}
	}
	return out
}

func LastSyncTime(profile string) time.Time {
	catalog, err := loadCatalog(profile)
	if err != nil {
		return time.Time{}
	}
	return catalog.SyncedAt
}

func GetCatalogItem(profile, slug string) (*client.CatalogItem, error) {
	catalog, err := loadCatalog(profile)
	if err != nil {
		return nil, err
	}
	for _, item := range catalog.Items {
		if item.Slug == slug && item.Library == "" {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("command %q not found", slug)
}

// Internal helpers

func loadCatalog(profile string) (*CachedCatalog, error) {
	migrateLegacyCache()
	path, err := profileCatalogPath(profile)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c CachedCatalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveCatalog(profile string, c *CachedCatalog) error {
	path, err := profileCatalogPath(profile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func validateCommandID(commandID string) error {
	if !validID.MatchString(commandID) {
		return fmt.Errorf("invalid command ID: %q", commandID)
	}
	// Double-check the resolved path stays under specDir
	resolved := filepath.Join(specDir(), commandID)
	if !strings.HasPrefix(resolved, specDir()+string(filepath.Separator)) {
		return fmt.Errorf("invalid command ID: %q", commandID)
	}
	return nil
}

func loadSpecFile(commandID string, version int) ([]byte, error) {
	if err := validateCommandID(commandID); err != nil {
		return nil, err
	}
	path := filepath.Join(specDir(), commandID, fmt.Sprintf("%d.json", version))
	return os.ReadFile(path)
}

func saveSpecFile(commandID string, version int, data json.RawMessage) error {
	if err := validateCommandID(commandID); err != nil {
		return err
	}
	dir := filepath.Join(specDir(), commandID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", version))
	return os.WriteFile(path, data, 0600)
}
