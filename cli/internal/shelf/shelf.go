package shelf

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mycli.sh/cli/internal/config"
	"mycli.sh/pkg/spec"
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func ShelvesDir() string {
	return filepath.Join(config.DefaultDir(), "shelves")
}

func ReposDir() string {
	return filepath.Join(ShelvesDir(), "repos")
}

func RegistryPath() string {
	return filepath.Join(ShelvesDir(), "shelves.json")
}

// LoadRegistry reads the shelves registry from disk.
// Returns an empty registry if the file doesn't exist.
func LoadRegistry() (*ShelfRegistry, error) {
	data, err := os.ReadFile(RegistryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &ShelfRegistry{}, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var reg ShelfRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return &reg, nil
}

// SaveRegistry writes the shelves registry to disk.
func SaveRegistry(reg *ShelfRegistry) error {
	if err := os.MkdirAll(ShelvesDir(), 0700); err != nil {
		return fmt.Errorf("create shelves dir: %w", err)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	return os.WriteFile(RegistryPath(), data, 0600)
}

// RepoLocalPath derives a local filesystem path from a git URL.
// For example: https://github.com/user/repo.git → github.com/user/repo
func RepoLocalPath(rawURL string) (string, error) {
	// Handle SSH-style URLs: git@github.com:user/repo.git
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, ":") && !strings.Contains(rawURL, "://") {
		parts := strings.SplitN(rawURL, ":", 2)
		host := strings.SplitN(parts[0], "@", 2)[1]
		path := strings.TrimSuffix(parts[1], ".git")
		return filepath.Join(ReposDir(), host, path), nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	host := u.Hostname()
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	// file:// URLs have no host; use "local" as a stand-in
	if host == "" && u.Scheme == "file" && path != "" {
		host = "local"
	}
	if host == "" || path == "" {
		return "", fmt.Errorf("cannot derive local path from URL %q", rawURL)
	}
	return filepath.Join(ReposDir(), host, path), nil
}

var manifestFileNames = []string{"shelf.yaml", "shelf.yml", "shelf.json"}

// detectManifestFile finds the first existing manifest file in repoPath.
// It tries shelf.yaml, shelf.yml, shelf.json in order.
// Returns the full path and the filename, or empty strings if none found.
func detectManifestFile(repoPath string) (string, string) {
	for _, name := range manifestFileNames {
		p := filepath.Join(repoPath, name)
		if _, err := os.Stat(p); err == nil {
			return p, name
		}
	}
	return "", ""
}

// LoadManifest reads and parses the shelf manifest from a repo directory.
// It looks for shelf.yaml, shelf.yml, or shelf.json (in that order).
func LoadManifest(repoPath string) (*ShelfManifest, error) {
	manifestPath, manifestName := detectManifestFile(repoPath)
	if manifestPath == "" {
		return nil, fmt.Errorf("no shelf manifest found (looked for %s)", strings.Join(manifestFileNames, ", "))
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", manifestName, err)
	}

	jsonData, err := spec.ToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestName, err)
	}

	var m ShelfManifest
	if err := json.Unmarshal(jsonData, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestName, err)
	}
	if m.ShelfVersion != 1 {
		return nil, fmt.Errorf("unsupported shelf version: %d (expected 1)", m.ShelfVersion)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("%s missing required field \"name\"", manifestName)
	}
	if len(m.Libraries) == 0 {
		return nil, fmt.Errorf("%s has no libraries", manifestName)
	}
	// Collect all library keys to check alias collisions
	libKeys := make(map[string]bool)
	for key := range m.Libraries {
		if !slugPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid library slug %q: must match %s", key, slugPattern.String())
		}
		libKeys[key] = true
	}

	// Validate library aliases
	allAliases := make(map[string]string) // alias -> owning library key
	for key, libDef := range m.Libraries {
		for _, alias := range libDef.Aliases {
			if !slugPattern.MatchString(alias) {
				return nil, fmt.Errorf("invalid library alias %q in %q: must match %s", alias, key, slugPattern.String())
			}
			if alias == key {
				return nil, fmt.Errorf("library alias %q cannot be the same as its own key in %q", alias, key)
			}
			if libKeys[alias] {
				return nil, fmt.Errorf("library alias %q in %q conflicts with library key", alias, key)
			}
			if owner, exists := allAliases[alias]; exists {
				return nil, fmt.Errorf("duplicate library alias %q in %q (already used by %q)", alias, key, owner)
			}
			allAliases[alias] = key
		}
	}
	return &m, nil
}

// DiscoverSpecs walks a library directory and returns valid command specs.
// It validates each spec using pkg/spec.Parse and checks that the filename matches the slug.
func DiscoverSpecs(repoPath string, libKey string, libDef LibraryDef) ([]ShelfCatalogItem, error) {
	libDir := filepath.Join(repoPath, libDef.Path)
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return nil, fmt.Errorf("read library dir %q: %w", libDef.Path, err)
	}

	var items []ShelfCatalogItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		slug, ok := specSlugFromFilename(entry.Name())
		if !ok {
			continue
		}

		specPath := filepath.Join(libDir, entry.Name())
		data, err := os.ReadFile(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", specPath, err)
			continue
		}

		s, err := spec.Parse(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid spec %s: %v\n", specPath, err)
			continue
		}

		// Filename (minus extension) must match the spec's slug
		if s.Metadata.Slug != slug {
			fmt.Fprintf(os.Stderr, "warning: %s slug %q doesn't match filename %q, skipping\n",
				specPath, s.Metadata.Slug, slug)
			continue
		}

		items = append(items, ShelfCatalogItem{
			Library:     libKey,
			Slug:        s.Metadata.Slug,
			Name:        s.Metadata.Name,
			Description: s.Metadata.Description,
			SpecPath:    specPath,
			Aliases:     s.Metadata.Aliases,
		})
	}
	return items, nil
}

// specSlugFromFilename extracts a slug from a spec filename.
// It accepts .json, .yaml, and .yml extensions. Returns the slug and true,
// or empty string and false if the filename is not a recognized spec file.
func specSlugFromFilename(name string) (string, bool) {
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext), true
		}
	}
	return "", false
}

// FindByName looks up a shelf by name in the registry.
func FindByName(reg *ShelfRegistry, name string) *ShelfEntry {
	for i := range reg.Shelves {
		if reg.Shelves[i].Name == name {
			return &reg.Shelves[i]
		}
	}
	return nil
}
