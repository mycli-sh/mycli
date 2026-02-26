package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
	"mycli.sh/cli/internal/library"
	"mycli.sh/pkg/spec"
)

var tagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func isSystemOwner(owner string) bool {
	return owner == "" || strings.EqualFold(owner, "system")
}

func newLibraryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "library",
		Aliases: []string{"lib"},
		Short:   "Discover, install, and manage command libraries",
		Long:    "Search public libraries, install from the registry, publish, and manage installed libraries.",
	}

	cmd.AddCommand(newLibrarySearchCmd())
	cmd.AddCommand(newLibraryInstallCmd())
	cmd.AddCommand(newLibraryUninstallCmd())
	cmd.AddCommand(newLibraryListCmd())
	cmd.AddCommand(newLibraryReleaseCmd())
	cmd.AddCommand(newLibraryInfoCmd())
	cmd.AddCommand(newLibraryExploreCmd())
	cmd.AddCommand(newLibrarySyncCmd())

	return cmd
}

func newLibrarySearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search public libraries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			resp, err := c.SearchPublicLibraries(args[0], 20, 0)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if len(resp.Libraries) == 0 {
				fmt.Println("No libraries found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tINSTALLS")
			for _, lib := range resp.Libraries {
				desc := lib.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", lib.Slug, desc, lib.InstallCount)
			}
			_ = w.Flush()

			if resp.Total > len(resp.Libraries) {
				fmt.Printf("\nShowing %d of %d results.\n", len(resp.Libraries), resp.Total)
			}
			return nil
		},
	}
}

func newLibraryInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <identifier>",
		Short: "Install a library from the registry",
		Long: `Install a library from the registry.

  my library install kubernetes
  my library install owner/name

For git-backed sources, use 'my source add <git-url>' instead.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			if isGitURL(identifier) {
				return fmt.Errorf("git URLs are not supported here; use 'my source add %s' instead", identifier)
			}
			return installRegistryLibrary(identifier)
		},
	}

	return cmd
}

func isGitURL(s string) bool {
	return strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "ssh://") ||
		strings.HasPrefix(s, "file://") ||
		(strings.Contains(s, "@") && strings.Contains(s, ":"))
}

func installRegistryLibrary(identifier string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	c := client.New(resolveAPIURL(cfg))
	defer c.Close()

	// Parse owner/slug
	owner, slug := parseOwnerSlug(identifier)

	// Verify library exists
	detail, err := c.GetPublicLibrary(owner, slug)
	if err != nil {
		return fmt.Errorf("library %q not found: %w", identifier, err)
	}

	// Install via API if logged in
	if auth.IsLoggedIn() {
		if err := c.InstallLibrary(owner, slug); err != nil {
			return fmt.Errorf("failed to install: %w", err)
		}

		// Sync so commands appear in catalog
		fmt.Println("Syncing commands...")
		fetched, syncErr := cache.Sync(c, false)
		if syncErr != nil {
			fmt.Fprintf(os.Stderr, "warning: sync failed: %v\n", syncErr)
		} else if fetched > 0 {
			fmt.Printf("Synced %d command(s).\n", fetched)
		}
	}

	// Register in local registry
	reg, err := library.LoadRegistry()
	if err != nil {
		return err
	}

	displayName := slug

	// Check for duplicate
	if library.FindByName(reg, displayName) != nil {
		fmt.Printf("Library %q is already installed.\n", displayName)
		return nil
	}

	reg.Sources = append(reg.Sources, library.SourceEntry{
		Name:        displayName,
		Owner:       detail.Owner,
		Slug:        detail.Library.Slug,
		Kind:        "registry",
		AddedAt:     time.Now(),
		LastUpdated: time.Now(),
	})
	if err := library.SaveRegistry(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("Installed %s (%d commands).\n", displayName, len(detail.Commands))
	return nil
}

func parseOwnerSlug(identifier string) (string, string) {
	if strings.Contains(identifier, "/") {
		parts := strings.SplitN(identifier, "/", 2)
		return parts[0], parts[1]
	}
	// Flat name (e.g., "kubernetes") - use system owner
	return "system", identifier
}

func newLibraryUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a registry library",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			entry := library.FindByName(reg, name)
			if entry == nil {
				return fmt.Errorf("library %q not found", name)
			}

			if entry.Kind == "git" {
				return fmt.Errorf("%q is a git source; use 'my source remove %s' instead", name, name)
			}

			// Uninstall from API if logged in
			if entry.Kind == "registry" && auth.IsLoggedIn() {
				cfg, err := config.Load()
				if err == nil {
					c := client.New(resolveAPIURL(cfg))
					defer c.Close()
					_ = c.UninstallLibrary(entry.Owner, entry.Slug)
				}
			}

			library.Remove(reg, name)
			if err := library.SaveRegistry(reg); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("Uninstalled library %q.\n", name)
			return nil
		},
	}
}

func newLibraryListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed libraries",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			// Merge API-installed libraries from the cached catalog
			catalog, _ := cache.GetCatalog()
			if catalog != nil {
				seen := map[string]bool{}
				for _, entry := range reg.Sources {
					seen[entry.Slug] = true
				}
				for _, item := range catalog.Items {
					if item.Library == "" {
						continue
					}
					if seen[item.Library] {
						continue
					}
					seen[item.Library] = true
					reg.Sources = append(reg.Sources, library.SourceEntry{
						Name:        item.Library,
						Owner:       item.LibraryOwner,
						Slug:        item.Library,
						Kind:        "registry",
						LastUpdated: catalog.SyncedAt,
					})
				}
			}

			if len(reg.Sources) == 0 {
				fmt.Println("No libraries installed. Run 'my library install <name>' or 'my source add <git-url>' to add one.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(reg.Sources, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tSOURCE\tUPDATED")
			for _, entry := range reg.Sources {
				updated := entry.LastUpdated.Format("2006-01-02 15:04")
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Name, entry.Kind, updated)
			}
			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

func newLibraryReleaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release <tag>",
		Short: "Create a versioned release of all libraries in the manifest",
		Long: `Create a release from a git tag (e.g., v1.0.0). Reads the manifest and specs
at the tagged commit and publishes them to the registry. All libraries in the
manifest are released under the same tag. Requires login.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := args[0]

			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}

			// Validate tag format
			if !tagPattern.MatchString(tag) {
				return fmt.Errorf("invalid tag format %q (must match vX.Y.Z)", tag)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			// Verify tag exists
			if !library.TagExists(cwd, tag) {
				return fmt.Errorf("tag %q not found (run 'git tag %s' first)", tag, tag)
			}

			// Get commit hash for the tag
			commitHash, err := library.TagCommitHash(cwd, tag)
			if err != nil {
				return fmt.Errorf("get tag commit: %w", err)
			}

			// Extract tag contents to a temp directory
			tmpDir, err := os.MkdirTemp("", "my-release-*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if err := library.ArchiveTag(cwd, tag, tmpDir); err != nil {
				return fmt.Errorf("extract tag: %w", err)
			}

			// Load manifest from the tagged content
			manifest, err := library.LoadManifest(tmpDir)
			if err != nil {
				return fmt.Errorf("no valid manifest at tag %s: %w", tag, err)
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			// Detect git remote URL for metadata
			var gitURL string
			if remote, err := getGitRemoteURL(cwd); err == nil {
				gitURL = remote
			}

			// Release each library in the manifest
			for libKey, libDef := range manifest.Libraries {
				items, err := library.DiscoverSpecs(tmpDir, libKey, libDef)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: %v\n", err)
					continue
				}

				// Read all spec files as raw JSON
				var specJSONs []json.RawMessage
				for _, item := range items {
					data, err := os.ReadFile(item.SpecPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", item.SpecPath, err)
						continue
					}
					jsonData, err := spec.ToJSON(data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: cannot convert %s to JSON: %v\n", item.SpecPath, err)
						continue
					}
					specJSONs = append(specJSONs, jsonData)
				}

				req := &client.CreateReleaseRequest{
					Tag:         tag,
					CommitHash:  commitHash,
					Namespace:   manifest.Namespace,
					Name:        libDef.Name,
					Description: libDef.Description,
					GitURL:      gitURL,
					Commands:    specJSONs,
				}

				resp, err := c.CreateRelease(libKey, req)
				if err != nil {
					// Check for 409 conflict (already released)
					if apiErr, ok := err.(*client.APIError); ok && apiErr.Code == "RELEASE_EXISTS" {
						fmt.Printf("Skipped %s %s (already exists)\n", libKey, tag)
						continue
					}
					return fmt.Errorf("failed to release %s: %w", libKey, err)
				}

				fmt.Printf("Released %s %s (%d commands)\n", libKey, tag, resp.Published)
			}

			return nil
		},
	}
}

func getGitRemoteURL(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func newLibraryInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <identifier>",
		Short: "Show details about a library",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			// Try local registry first
			reg, _ := library.LoadRegistry()
			if reg != nil {
				if entry := library.FindByName(reg, identifier); entry != nil {
					fmt.Printf("Name:    %s\n", entry.Name)
					fmt.Printf("Source:  %s\n", entry.Kind)
					if isSystemOwner(entry.Owner) {
						fmt.Printf("Status:  official\n")
					} else {
						fmt.Printf("Owner:   %s\n", entry.Owner)
					}
					if entry.GitURL != "" {
						fmt.Printf("Git URL: %s\n", entry.GitURL)
					}
					if entry.LastCommit != "" {
						fmt.Printf("Commit:  %s\n", entry.LastCommit)
					}
					fmt.Printf("Updated: %s\n", entry.LastUpdated.Format("2006-01-02 15:04"))

					if entry.Kind == "git" && entry.LocalPath != "" {
						// Show libraries and commands from manifest
						manifest, err := library.LoadManifest(entry.LocalPath)
						if err == nil {
							totalCmds := 0
							for libKey, libDef := range manifest.Libraries {
								items, _ := library.DiscoverSpecs(entry.LocalPath, libKey, libDef)
								totalCmds += len(items)
								fmt.Printf("\nLibrary %s (%d commands):\n", libKey, len(items))
								for _, item := range items {
									fmt.Printf("  %s — %s\n", item.Slug, item.Description)
								}
							}
							fmt.Printf("\nTotal: %d commands\n", totalCmds)
						}
					}
					return nil
				}
			}

			// Try API
			owner, slug := parseOwnerSlug(identifier)
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("library %q not found", identifier)
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			detail, err := c.GetPublicLibrary(owner, slug)
			if err != nil {
				return fmt.Errorf("library %q not found", identifier)
			}

			fmt.Printf("Name:        %s\n", detail.Library.Name)
			fmt.Printf("Slug:        %s\n", detail.Library.Slug)
			if isSystemOwner(detail.Owner) {
				fmt.Printf("Status:      official\n")
			} else {
				fmt.Printf("Owner:       %s\n", detail.Owner)
			}
			if detail.Library.Description != "" {
				fmt.Printf("Description: %s\n", detail.Library.Description)
			}
			fmt.Printf("Installs:    %d\n", detail.Library.InstallCount)
			if len(detail.Commands) > 0 {
				fmt.Printf("\nCommands (%d):\n", len(detail.Commands))
				for _, cmd := range detail.Commands {
					fmt.Printf("  %s — %s\n", cmd.Slug, cmd.Description)
				}
			}

			// Show releases
			releases, err := c.ListReleases(owner, slug)
			if err == nil && len(releases) > 0 {
				fmt.Printf("\nReleases:\n")
				for _, rel := range releases {
					fmt.Printf("  %s  %s  (%d commands)\n", rel.Tag, rel.ReleasedAt, rel.CommandCount)
				}
			}
			return nil
		},
	}
}

func newLibrarySyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync registry libraries with the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			fmt.Println("Syncing...")
			fetched, err := cache.Sync(c, true)
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}
			if fetched == 0 {
				fmt.Println("Already up to date.")
			} else {
				fmt.Printf("Synced %d command(s).\n", fetched)
			}
			return nil
		},
	}
}
