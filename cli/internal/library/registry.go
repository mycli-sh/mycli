package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mycli.sh/cli/internal/config"
)

// SourceRegistry is the on-disk format for ~/.my/sources/sources.json.
type SourceRegistry struct {
	Sources []SourceEntry `json:"sources"`
}

// SourceEntry represents a single installed source (a git repo or registry entry).
type SourceEntry struct {
	Name        string    `json:"name"`            // local display name
	Owner       string    `json:"owner,omitempty"` // owner username (empty for git)
	Slug        string    `json:"slug"`            // library slug
	Kind        string    `json:"kind"`            // "registry" or "git"
	GitURL      string    `json:"git_url,omitempty"`
	Ref         string    `json:"ref,omitempty"`
	LocalPath   string    `json:"local_path,omitempty"`
	AddedAt     time.Time `json:"added_at"`
	LastUpdated time.Time `json:"last_updated"`
	LastCommit  string    `json:"last_commit,omitempty"`
	Libraries   []string  `json:"libraries,omitempty"` // library slugs within this source (for git with multiple libs)
}

func SourcesDir() string {
	return filepath.Join(config.DefaultDir(), "sources")
}

func ReposDir() string {
	return filepath.Join(SourcesDir(), "repos")
}

func RegistryPath() string {
	return filepath.Join(SourcesDir(), "sources.json")
}

// LoadRegistry reads the source registry from disk.
// Returns an empty registry if the file doesn't exist.
func LoadRegistry() (*SourceRegistry, error) {
	data, err := os.ReadFile(RegistryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &SourceRegistry{}, nil
		}
		return nil, fmt.Errorf("read source registry: %w", err)
	}
	var reg SourceRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse source registry: %w", err)
	}
	return &reg, nil
}

// SaveRegistry writes the source registry to disk.
func SaveRegistry(reg *SourceRegistry) error {
	if err := os.MkdirAll(SourcesDir(), 0700); err != nil {
		return fmt.Errorf("create sources dir: %w", err)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal source registry: %w", err)
	}
	return os.WriteFile(RegistryPath(), data, 0600)
}

// FindByName looks up a source entry by its local name.
func FindByName(reg *SourceRegistry, name string) *SourceEntry {
	for i := range reg.Sources {
		if reg.Sources[i].Name == name {
			return &reg.Sources[i]
		}
	}
	return nil
}

// FindByOwnerSlug looks up a source entry by owner/slug.
func FindByOwnerSlug(reg *SourceRegistry, owner, slug string) *SourceEntry {
	for i := range reg.Sources {
		if reg.Sources[i].Owner == owner && reg.Sources[i].Slug == slug {
			return &reg.Sources[i]
		}
	}
	return nil
}

// FindBySlug looks up a source entry by slug alone.
func FindBySlug(reg *SourceRegistry, slug string) *SourceEntry {
	for i := range reg.Sources {
		if reg.Sources[i].Slug == slug {
			return &reg.Sources[i]
		}
	}
	return nil
}

// Remove removes a source entry by name and returns true if found.
func Remove(reg *SourceRegistry, name string) bool {
	for i := range reg.Sources {
		if reg.Sources[i].Name == name {
			reg.Sources = append(reg.Sources[:i], reg.Sources[i+1:]...)
			return true
		}
	}
	return false
}
