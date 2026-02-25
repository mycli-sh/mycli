package library

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"mycli.sh/pkg/spec"
)

// Manifest is the on-disk format for mycli.yaml in a library repo.
type Manifest struct {
	Version     int                   `json:"schemaVersion" yaml:"schemaVersion"`
	Namespace   string                `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name        string                `json:"name" yaml:"name"`
	Description string                `json:"description,omitempty" yaml:"description,omitempty"`
	Libraries   map[string]LibraryDef `json:"libraries" yaml:"libraries"`
}

// LibraryDef defines a library within a manifest.
type LibraryDef struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Path        string   `json:"path" yaml:"path"`
	Aliases     []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
}

// CatalogItem is a bridge type used to register library commands with Cobra.
type CatalogItem struct {
	SourceName  string
	Library     string
	Slug        string
	Name        string
	Description string
	SpecPath    string // absolute path to the spec file
	Aliases     []string
}

// RepoLocalPath derives a local filesystem path from a git URL.
// For example: https://github.com/user/repo.git → ~/.my/sources/repos/github.com/user/repo
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

// manifestFileNames lists accepted manifest filenames in priority order.
// New names are tried first; old shelf.* names are kept as fallback for backward compatibility.
var manifestFileNames = []string{
	"mycli.yaml", "mycli.yml", "mycli.json",
	"shelf.yaml", "shelf.yml", "shelf.json",
}

// detectManifestFile finds the first existing manifest file in repoPath.
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

// LoadManifest reads and parses the library manifest from a repo directory.
// It looks for mycli.yaml, mycli.yml, mycli.json, shelf.yaml, shelf.yml, or shelf.json (in that order).
func LoadManifest(repoPath string) (*Manifest, error) {
	manifestPath, manifestName := detectManifestFile(repoPath)
	if manifestPath == "" {
		return nil, fmt.Errorf("no library manifest found (looked for %s)", strings.Join(manifestFileNames, ", "))
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", manifestName, err)
	}

	jsonData, err := spec.ToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestName, err)
	}

	if err := ValidateManifest(jsonData); err != nil {
		return nil, fmt.Errorf("validate %s: %w", manifestName, err)
	}

	var m Manifest
	if err := json.Unmarshal(jsonData, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestName, err)
	}

	return &m, nil
}

// DiscoverSpecs walks a library directory and returns valid command specs.
// It validates each spec using pkg/spec.Parse and checks that the filename matches the slug.
func DiscoverSpecs(repoPath string, libKey string, libDef LibraryDef) ([]CatalogItem, error) {
	libDir := filepath.Join(repoPath, libDef.Path)
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return nil, fmt.Errorf("read library dir %q: %w", libDef.Path, err)
	}

	var items []CatalogItem
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

		items = append(items, CatalogItem{
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
