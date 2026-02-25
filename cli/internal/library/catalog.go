package library

import (
	"fmt"
	"os"

	"mycli.sh/pkg/spec"
)

// LibraryCatalog groups catalog items with library-level aliases.
type LibraryCatalog struct {
	Items   []CatalogItem
	Aliases []string
}

// GetAllLibraries iterates all installed sources and returns catalog items grouped by library slug.
func GetAllLibraries() (map[string]LibraryCatalog, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	result := make(map[string]LibraryCatalog)
	for _, entry := range reg.Sources {
		if entry.Kind != "git" || entry.LocalPath == "" {
			continue
		}

		manifest, err := LoadManifest(entry.LocalPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot load source %q: %v\n", entry.Name, err)
			continue
		}

		for libKey, libDef := range manifest.Libraries {
			items, err := DiscoverSpecs(entry.LocalPath, libKey, libDef)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: cannot discover specs for %s/%s: %v\n", entry.Name, libKey, err)
				continue
			}
			for i := range items {
				items[i].SourceName = entry.Name
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
