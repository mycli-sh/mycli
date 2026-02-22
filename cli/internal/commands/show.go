package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/library"
	"mycli.sh/cli/internal/shelf"
	"mycli.sh/pkg/spec"
)

func newShowCmd() *cobra.Command {
	var raw bool

	cmd := &cobra.Command{
		Use:   "show <slug>",
		Short: "Show details of a command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]

			s, err := resolveSpec(slug)
			if err != nil {
				return err
			}

			if raw {
				data, err := json.MarshalIndent(s, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal spec: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Name:        %s\n", s.Metadata.Name)
			fmt.Printf("Slug:        %s\n", s.Metadata.Slug)
			if s.Metadata.Description != "" {
				fmt.Printf("Description: %s\n", s.Metadata.Description)
			}
			if len(s.Metadata.Tags) > 0 {
				fmt.Printf("Tags:        %v\n", s.Metadata.Tags)
			}
			if len(s.Metadata.Aliases) > 0 {
				fmt.Printf("Aliases:     %v\n", s.Metadata.Aliases)
			}
			if len(s.Dependencies) > 0 {
				fmt.Printf("Deps:        %v\n", s.Dependencies)
			}

			if len(s.Args.Positional) > 0 {
				fmt.Println("\nPositional Arguments:")
				for _, a := range s.Args.Positional {
					req := "required"
					if !a.IsRequired() {
						req = "optional"
					}
					fmt.Printf("  %-15s %s (%s)\n", a.Name, a.Description, req)
				}
			}

			if len(s.Args.Flags) > 0 {
				fmt.Println("\nFlags:")
				for _, f := range s.Args.Flags {
					short := ""
					if f.Short != "" {
						short = "-" + f.Short + ", "
					}
					fmt.Printf("  %s--%s\t%s [%s]\n", short, f.Name, f.Description, f.GetType())
				}
			}

			fmt.Printf("\nSteps (%d):\n", len(s.Steps))
			for i, step := range s.Steps {
				fmt.Printf("  %d. %s\n", i+1, step.Name)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "output raw JSON spec")
	return cmd
}

// resolveSpec looks up a command spec by trying multiple resolution strategies:
// 1. If the argument contains "/", treat it as library/slug and try API library cache then git libraries.
// 2. Otherwise, try the API top-level cache then search all git libraries.
func resolveSpec(arg string) (*spec.CommandSpec, error) {
	if strings.Contains(arg, "/") {
		parts := strings.SplitN(arg, "/", 2)
		libName, cmdSlug := parts[0], parts[1]

		// Try API library cache
		if s, err := cache.GetLibrarySpec(libName, cmdSlug); err == nil {
			return s, nil
		}

		// Try git library lookup (with alias resolution)
		return resolveGitLibrarySpec(libName, cmdSlug)
	}

	// Try API top-level cache
	if s, err := cache.GetSpec(arg); err == nil {
		return s, nil
	}

	// Try git library search across all libraries
	reg, err := library.LoadRegistry()
	if err == nil && reg != nil {
		for _, entry := range reg.Libraries {
			if entry.Source != "git" || entry.LocalPath == "" {
				continue
			}
			manifest, err := shelf.LoadManifest(entry.LocalPath)
			if err != nil {
				continue
			}
			for libKey, libDef := range manifest.Libraries {
				items, err := shelf.DiscoverSpecs(entry.LocalPath, libKey, libDef)
				if err != nil {
					continue
				}
				for _, item := range items {
					if item.Slug == arg || containsAlias(item.Aliases, arg) {
						return shelf.GetSpec(item.SpecPath)
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("command %q not found", arg)
}

func resolveGitLibrarySpec(libName, cmdSlug string) (*spec.CommandSpec, error) {
	reg, err := library.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("command %q not found in library %q", cmdSlug, libName)
	}

	for _, entry := range reg.Libraries {
		if entry.Source != "git" || entry.LocalPath == "" {
			continue
		}
		manifest, err := shelf.LoadManifest(entry.LocalPath)
		if err != nil {
			continue
		}
		for libKey, libDef := range manifest.Libraries {
			if libKey != libName && !containsAlias(libDef.Aliases, libName) {
				continue
			}
			items, err := shelf.DiscoverSpecs(entry.LocalPath, libKey, libDef)
			if err != nil {
				continue
			}
			for _, item := range items {
				if item.Slug == cmdSlug || containsAlias(item.Aliases, cmdSlug) {
					return shelf.GetSpec(item.SpecPath)
				}
			}
		}
	}

	return nil, fmt.Errorf("command %q not found in library %q", cmdSlug, libName)
}

func containsAlias(aliases []string, name string) bool {
	for _, a := range aliases {
		if a == name {
			return true
		}
	}
	return false
}
