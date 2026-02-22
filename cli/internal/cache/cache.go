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

var validID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type CachedCatalog struct {
	Items    []client.CatalogItem `json:"items"`
	ETag     string               `json:"etag"`
	SyncedAt time.Time            `json:"synced_at"`
}

func cacheDir() string {
	return filepath.Join(config.DefaultDir(), "cache")
}

func catalogPath() string {
	return filepath.Join(cacheDir(), "catalog.json")
}

func specDir() string {
	return filepath.Join(cacheDir(), "specs")
}

func Sync(c *client.Client, force bool) (int, error) {
	current, _ := loadCatalog()
	etag := ""
	if !force && current != nil {
		etag = current.ETag
	}

	resp, err := c.GetCatalog(etag)
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
		if err := saveCatalog(cached); err != nil {
			return 0, fmt.Errorf("save catalog: %w", err)
		}
	}

	return fetched, nil
}

func GetSpec(slug string) (*spec.CommandSpec, error) {
	catalog, err := loadCatalog()
	if err != nil {
		return nil, fmt.Errorf("no cached catalog (run 'my cli sync' first): %w", err)
	}

	for _, item := range catalog.Items {
		if item.Slug == slug && item.Library == "" {
			data, err := loadSpecFile(item.CommandID, item.Version)
			if err != nil {
				return nil, fmt.Errorf("spec not cached (run 'my cli sync'): %w", err)
			}
			return spec.Parse(data)
		}
	}

	return nil, fmt.Errorf("command %q not found in catalog", slug)
}

func GetLibrarySpec(libraryKey, slug string) (*spec.CommandSpec, error) {
	catalog, err := loadCatalog()
	if err != nil {
		return nil, fmt.Errorf("no cached catalog (run 'my cli sync' first): %w", err)
	}

	for _, item := range catalog.Items {
		if matchLibraryKey(item, libraryKey) && item.Slug == slug {
			data, err := loadSpecFile(item.CommandID, item.Version)
			if err != nil {
				return nil, fmt.Errorf("spec not cached (run 'my cli sync'): %w", err)
			}
			return spec.Parse(data)
		}
	}

	return nil, fmt.Errorf("command %q not found in library %q", slug, libraryKey)
}

func GetLibraryCatalogItem(libraryKey, slug string) (*client.CatalogItem, error) {
	catalog, err := loadCatalog()
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

func GetCatalog() (*CachedCatalog, error) {
	return loadCatalog()
}

func LastSyncTime() time.Time {
	catalog, err := loadCatalog()
	if err != nil {
		return time.Time{}
	}
	return catalog.SyncedAt
}

func GetCatalogItem(slug string) (*client.CatalogItem, error) {
	catalog, err := loadCatalog()
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

func loadCatalog() (*CachedCatalog, error) {
	data, err := os.ReadFile(catalogPath())
	if err != nil {
		return nil, err
	}
	var c CachedCatalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveCatalog(c *CachedCatalog) error {
	if err := os.MkdirAll(cacheDir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(catalogPath(), data, 0600)
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
