package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
	"mycli.sh/cli/internal/engine"
	"mycli.sh/cli/internal/history"
	"mycli.sh/cli/internal/library"
	"mycli.sh/cli/internal/termui"
	"mycli.sh/cli/internal/update"
)

var apiURL string

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "my",
		Short:         "CLI of CLIs — your personal command runner",
		Long:          "my is a remotely-configurable CLI tool for creating and running personal commands.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if termui.IsTTY() {
				update.CheckInBackground()
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if termui.IsTTY() {
				update.NotifyIfAvailable()
			}
		},
	}

	cmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "API server URL (overrides config)")

	// Register sub-commands
	cmd.AddCommand(newCliCmd())
	cmd.AddCommand(newLibraryCmd())
	cmd.AddCommand(newSourceCmd())

	// Register dynamic library commands from cached catalog
	registeredSlugs, registeredAliases := registerAPILibraryCommands(cmd)

	// Register git library commands (API libraries take precedence)
	registerGitLibraryCommands(cmd, registeredSlugs, registeredAliases)

	return cmd
}

func resolveAPIURL(cfg *config.Config) string {
	if apiURL != "" {
		return apiURL
	}
	return cfg.APIURL
}

// registerAPILibraryCommands registers library commands from the cached API catalog.
// Returns a map of library slug -> set of command slugs so git library commands can merge at the command level,
// and a set of registered aliases (including library slugs) for collision detection.
func registerAPILibraryCommands(root *cobra.Command) (map[string]map[string]bool, map[string]bool) {
	registered := make(map[string]map[string]bool)
	registeredAliases := make(map[string]bool)

	catalog, err := cache.GetCatalog()
	if err != nil || catalog == nil {
		return registered, registeredAliases
	}

	// Group catalog items by owner/slug key
	libraryItems := make(map[string][]client.CatalogItem)
	for _, item := range catalog.Items {
		if item.Library != "" {
			key := libraryKey(item.LibraryOwner, item.Library)
			libraryItems[key] = append(libraryItems[key], item)
		}
	}

	// Track slug usage to detect collisions
	slugCount := make(map[string]int)
	for key := range libraryItems {
		_, slug := splitLibraryKey(key)
		slugCount[slug]++
	}

	for libKey, items := range libraryItems {
		_, slug := splitLibraryKey(libKey)

		// Use slug-only as the primary command name (common case).
		// Fall back to owner/slug only when two installed libraries share the same slug.
		displayKey := slug
		if slugCount[slug] > 1 {
			displayKey = libKey
		}

		registered[displayKey] = make(map[string]bool)
		registeredAliases[displayKey] = true

		libCmd := &cobra.Command{
			Use:   displayKey,
			Short: fmt.Sprintf("Commands from the %s library", slug),
		}

		// Apply library-level aliases from the catalog (use the first item's LibraryAliases)
		if len(items) > 0 {
			var validAliases []string
			for _, alias := range items[0].LibraryAliases {
				if registeredAliases[alias] {
					fmt.Fprintf(os.Stderr, "warning: library alias %q for %q conflicts with existing command, skipping\n", alias, displayKey)
					continue
				}
				validAliases = append(validAliases, alias)
				registeredAliases[alias] = true
			}
			if len(validAliases) > 0 {
				libCmd.Aliases = validAliases
			}
		}

		for _, item := range items {
			item := item // capture loop variable
			libKeyCapture := libKey
			displayKeyCapture := displayKey
			registered[displayKey][item.Slug] = true
			cmdEntry := &cobra.Command{
				Use:                item.Slug,
				Aliases:            item.Aliases,
				Short:              item.Description,
				DisableFlagParsing: true,
				RunE: func(cmd *cobra.Command, args []string) error {
					s, err := cache.GetLibrarySpec(libKeyCapture, item.Slug)
					if err != nil {
						return err
					}

					if containsHelpFlag(args) {
						fmt.Print(engine.FormatHelp(s, "my "+displayKeyCapture+" "+item.Slug))
						return nil
					}

					catItem, _ := cache.GetLibraryCatalogItem(libKeyCapture, item.Slug)

					result, execErr := engine.Execute(s, args, engine.ExecOpts{})

					if catItem != nil && result != nil {
						_ = history.Record(history.Entry{
							Timestamp:  time.Now(),
							Slug:       displayKeyCapture + "/" + item.Slug,
							CommandID:  catItem.CommandID,
							Version:    catItem.Version,
							ExitCode:   result.ExitCode,
							DurationMs: result.DurationMs,
						})
					}

					if execErr != nil {
						exitCode := 1
						if result != nil && result.ExitCode != 0 {
							exitCode = result.ExitCode
						}
						return &engine.ExitCodeError{Code: exitCode, Err: execErr}
					}
					return nil
				},
			}
			libCmd.AddCommand(cmdEntry)
		}

		root.AddCommand(libCmd)
	}

	return registered, registeredAliases
}

// libraryKey constructs a library identifier. If owner is set, returns "owner/slug"; otherwise just "slug".
func libraryKey(owner, slug string) string {
	if owner != "" {
		return owner + "/" + slug
	}
	return slug
}

// splitLibraryKey splits "owner/slug" into (owner, slug). If no "/" is present, returns ("", key).
func splitLibraryKey(key string) (string, string) {
	if i := strings.Index(key, "/"); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

// containsHelpFlag returns true if args contain --help or -h.
func containsHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
		// Stop scanning after "--" (end of flags)
		if a == "--" {
			return false
		}
	}
	return false
}

// registerGitLibraryCommands registers commands from git-backed libraries.
// Commands are merged at the command level — only individual slug collisions are skipped,
// not entire libraries. API commands take precedence on per-command collisions.
func registerGitLibraryCommands(root *cobra.Command, registeredCmds map[string]map[string]bool, registeredAliases map[string]bool) {
	reg, err := library.LoadRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load source registry: %v\n", err)
		return
	}

	for _, entry := range reg.Sources {
		if entry.Kind != "git" || entry.LocalPath == "" {
			continue
		}

		manifest, err := library.LoadManifest(entry.LocalPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot load source %q: %v\n", entry.Name, err)
			continue
		}

		for libSlug, libDef := range manifest.Libraries {
			registeredAliases[libSlug] = true

			items, err := library.DiscoverSpecs(entry.LocalPath, libSlug, libDef)
			if err != nil {
				continue
			}

			// Determine if a Cobra command already exists for this library (from API)
			var libCmd *cobra.Command
			isNewLib := registeredCmds[libSlug] == nil

			if !isNewLib {
				for _, c := range root.Commands() {
					if c.Name() == libSlug {
						libCmd = c
						break
					}
				}
			}

			if libCmd == nil {
				libCmd = &cobra.Command{
					Use:   libSlug,
					Short: fmt.Sprintf("Commands from the %s library", libSlug),
				}
			}

			// Set library-level aliases (filter out collisions)
			var validLibAliases []string
			for _, alias := range libDef.Aliases {
				if registeredAliases[alias] {
					fmt.Fprintf(os.Stderr, "warning: library alias %q for %q conflicts with existing command, skipping\n", alias, libSlug)
					continue
				}
				validLibAliases = append(validLibAliases, alias)
				registeredAliases[alias] = true
			}
			if len(validLibAliases) > 0 {
				libCmd.Aliases = validLibAliases
			}

			if registeredCmds[libSlug] == nil {
				registeredCmds[libSlug] = make(map[string]bool)
			}

			for _, item := range items {
				if registeredCmds[libSlug][item.Slug] {
					fmt.Fprintf(os.Stderr, "warning: git library command %s/%s shadowed by existing command\n",
						libSlug, item.Slug)
					continue
				}
				registeredCmds[libSlug][item.Slug] = true

				item := item // capture loop variable
				cmdEntry := &cobra.Command{
					Use:                item.Slug,
					Aliases:            item.Aliases,
					Short:              item.Description,
					DisableFlagParsing: true,
					RunE: func(cmd *cobra.Command, args []string) error {
						s, err := library.GetSpec(item.SpecPath)
						if err != nil {
							return err
						}

						if containsHelpFlag(args) {
							fmt.Print(engine.FormatHelp(s, "my "+item.Library+" "+item.Slug))
							return nil
						}

						result, execErr := engine.Execute(s, args, engine.ExecOpts{})

						if result != nil {
							_ = history.Record(history.Entry{
								Timestamp:  time.Now(),
								Slug:       item.Library + "/" + item.Slug,
								ExitCode:   result.ExitCode,
								DurationMs: result.DurationMs,
							})
						}

						if execErr != nil {
							exitCode := 1
							if result != nil && result.ExitCode != 0 {
								exitCode = result.ExitCode
							}
							return &engine.ExitCodeError{Code: exitCode, Err: execErr}
						}
						return nil
					},
				}
				libCmd.AddCommand(cmdEntry)
			}

			if isNewLib {
				root.AddCommand(libCmd)
			}
		}
	}
}
