package shelf

import "time"

// ShelfRegistry is the on-disk format for ~/.my/shelves/shelves.json.
type ShelfRegistry struct {
	Shelves []ShelfEntry `json:"shelves"`
}

// ShelfEntry represents a single added shelf.
type ShelfEntry struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Ref         string    `json:"ref"`
	Pinned      bool      `json:"pinned"`
	AddedAt     time.Time `json:"added_at"`
	LastUpdated time.Time `json:"last_updated"`
	LastCommit  string    `json:"last_commit"`
	Libraries   []string  `json:"libraries"`
}

// ShelfManifest is the on-disk format for shelf.yaml (or shelf.json) in a shelf repo.
type ShelfManifest struct {
	ShelfVersion int                   `json:"shelfVersion" yaml:"shelfVersion"`
	Namespace    string                `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name         string                `json:"name" yaml:"name"`
	Description  string                `json:"description,omitempty" yaml:"description,omitempty"`
	Libraries    map[string]LibraryDef `json:"libraries" yaml:"libraries"`
}

// LibraryDef defines a library within a shelf manifest.
type LibraryDef struct {
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Path        string   `json:"path" yaml:"path"`
	Aliases     []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
}

// ShelfCatalogItem is a bridge type used to register shelf commands with Cobra.
type ShelfCatalogItem struct {
	ShelfName   string
	Library     string
	Slug        string
	Name        string
	Description string
	SpecPath    string // absolute path to the spec file
	Aliases     []string
}
