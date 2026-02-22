package shelf

import (
	"fmt"
	"os"

	"mycli.sh/pkg/spec"
)

// LibraryCatalog groups shelf catalog items with library-level aliases.
type LibraryCatalog struct {
	Items   []ShelfCatalogItem
	Aliases []string
}

// GetAllLibraries iterates all shelves and returns catalog items grouped by library slug.
func GetAllLibraries() (map[string]LibraryCatalog, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	result := make(map[string]LibraryCatalog)
	for _, entry := range reg.Shelves {
		repoPath, err := RepoLocalPath(entry.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: bad shelf URL %q: %v\n", entry.URL, err)
			continue
		}

		manifest, err := LoadManifest(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot load shelf %q: %v\n", entry.Name, err)
			continue
		}

		for libKey, libDef := range manifest.Libraries {
			items, err := DiscoverSpecs(repoPath, libKey, libDef)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: cannot discover specs for %s/%s: %v\n", entry.Name, libKey, err)
				continue
			}
			for i := range items {
				items[i].ShelfName = entry.Name
			}
			cat := result[libKey]
			cat.Items = append(cat.Items, items...)
			if len(libDef.Aliases) > 0 {
				cat.Aliases = libDef.Aliases
			}
			result[libKey] = cat
		}
	}
	return result, nil
}

// GetSpec reads and parses a command spec from the given absolute path.
func GetSpec(specPath string) (*spec.CommandSpec, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	return spec.Parse(data)
}
