package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mycli.sh/cli/internal/config"
)

// LibraryRegistry is the on-disk format for ~/.my/libraries/libraries.json.
type LibraryRegistry struct {
	Libraries []LibraryEntry `json:"libraries"`
}

// LibraryEntry represents a single installed library.
type LibraryEntry struct {
	Name        string    `json:"name"`            // local display name
	Owner       string    `json:"owner,omitempty"` // owner username (empty for git)
	Slug        string    `json:"slug"`            // library slug
	Source      string    `json:"source"`          // "registry" or "git"
	GitURL      string    `json:"git_url,omitempty"`
	Ref         string    `json:"ref,omitempty"`
	LocalPath   string    `json:"local_path,omitempty"`
	AddedAt     time.Time `json:"added_at"`
	LastUpdated time.Time `json:"last_updated"`
	LastCommit  string    `json:"last_commit,omitempty"`
	Libraries   []string  `json:"libraries,omitempty"` // library slugs within this source (for git with multiple libs)
}

func LibrariesDir() string {
	return filepath.Join(config.DefaultDir(), "libraries")
}

func ReposDir() string {
	return filepath.Join(LibrariesDir(), "repos")
}

func RegistryPath() string {
	return filepath.Join(LibrariesDir(), "libraries.json")
}

// LoadRegistry reads the library registry from disk.
// Returns an empty registry if the file doesn't exist.
func LoadRegistry() (*LibraryRegistry, error) {
	data, err := os.ReadFile(RegistryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &LibraryRegistry{}, nil
		}
		return nil, fmt.Errorf("read library registry: %w", err)
	}
	var reg LibraryRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse library registry: %w", err)
	}
	return &reg, nil
}

// SaveRegistry writes the library registry to disk.
func SaveRegistry(reg *LibraryRegistry) error {
	if err := os.MkdirAll(LibrariesDir(), 0700); err != nil {
		return fmt.Errorf("create libraries dir: %w", err)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal library registry: %w", err)
	}
	return os.WriteFile(RegistryPath(), data, 0600)
}

// FindByName looks up a library entry by its local name.
func FindByName(reg *LibraryRegistry, name string) *LibraryEntry {
	for i := range reg.Libraries {
		if reg.Libraries[i].Name == name {
			return &reg.Libraries[i]
		}
	}
	return nil
}

// FindByOwnerSlug looks up a library entry by owner/slug.
func FindByOwnerSlug(reg *LibraryRegistry, owner, slug string) *LibraryEntry {
	for i := range reg.Libraries {
		if reg.Libraries[i].Owner == owner && reg.Libraries[i].Slug == slug {
			return &reg.Libraries[i]
		}
	}
	return nil
}

// FindBySlug looks up a library entry by slug alone.
func FindBySlug(reg *LibraryRegistry, slug string) *LibraryEntry {
	for i := range reg.Libraries {
		if reg.Libraries[i].Slug == slug {
			return &reg.Libraries[i]
		}
	}
	return nil
}

// Remove removes a library entry by name and returns true if found.
func Remove(reg *LibraryRegistry, name string) bool {
	for i := range reg.Libraries {
		if reg.Libraries[i].Name == name {
			reg.Libraries = append(reg.Libraries[:i], reg.Libraries[i+1:]...)
			return true
		}
	}
	return false
}
