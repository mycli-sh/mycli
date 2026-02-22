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
	"mycli.sh/cli/internal/shelf"
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
		Long:    "Search public libraries, install from the registry or git, update, publish, and manage installed libraries.",
	}

	cmd.AddCommand(newLibrarySearchCmd())
	cmd.AddCommand(newLibraryAddCmd())
	cmd.AddCommand(newLibraryRemoveCmd())
	cmd.AddCommand(newLibraryListCmd())
	cmd.AddCommand(newLibraryUpdateCmd())
	cmd.AddCommand(newLibraryReleaseCmd())
	cmd.AddCommand(newLibraryInfoCmd())
	cmd.AddCommand(newLibraryExploreCmd())

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

func newLibraryAddCmd() *cobra.Command {
	var ref string
	var name string

	cmd := &cobra.Command{
		Use:   "add <identifier>",
		Short: "Install a library from the registry or git",
		Long: `Install a library. The identifier determines the source:

  my library add name            Registry install (e.g., kubernetes)
  my library add owner/name      Registry install with disambiguation (e.g., fernando/devops)
  my library add https://...     Git clone from URL
  my library add git@...         Git clone from SSH URL`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			if isGitURL(identifier) {
				return addGitLibrary(identifier, ref, name)
			}
			return addRegistryLibrary(identifier)
		},
	}

	cmd.Flags().StringVar(&ref, "ref", "", "git branch or tag to checkout (git libraries only)")
	cmd.Flags().StringVar(&name, "name", "", "alias for the library (git libraries only)")
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

func addRegistryLibrary(identifier string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	c := client.New(resolveAPIURL(cfg))

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

	reg.Libraries = append(reg.Libraries, library.LibraryEntry{
		Name:        displayName,
		Owner:       detail.Owner,
		Slug:        detail.Library.Slug,
		Source:      "registry",
		AddedAt:     time.Now(),
		LastUpdated: time.Now(),
	})
	if err := library.SaveRegistry(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("Installed %s (%d commands).\n", displayName, len(detail.Commands))
	return nil
}

func addGitLibrary(url, ref, nameOverride string) error {
	reg, err := library.LoadRegistry()
	if err != nil {
		return err
	}

	// Derive local path
	dest, err := shelf.RepoLocalPath(url)
	if err != nil {
		return err
	}
	// Use library repos dir instead of shelf repos dir
	dest = strings.Replace(dest, shelf.ReposDir(), library.ReposDir(), 1)

	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("directory already exists: %s", dest)
	}

	// Clone
	fmt.Printf("Cloning %s...\n", url)
	if err := shelf.Clone(url, dest, ref); err != nil {
		return err
	}

	// Parse manifest
	manifest, err := shelf.LoadManifest(dest)
	if err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("invalid library: %w", err)
	}

	name := nameOverride
	if name == "" {
		name = manifest.Name
	}

	// Check for duplicate name
	if library.FindByName(reg, name) != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("library %q already exists (use --name to set a different alias)", name)
	}

	// Discover and validate all specs
	totalSpecs := 0
	var libKeys []string
	for libKey, libDef := range manifest.Libraries {
		items, err := shelf.DiscoverSpecs(dest, libKey, libDef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		totalSpecs += len(items)
		libKeys = append(libKeys, libKey)
	}

	if totalSpecs == 0 {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("no valid commands found in library (check warnings above)")
	}

	// Get HEAD commit
	commit, _ := shelf.HeadCommit(dest)

	// Save to registry
	entry := library.LibraryEntry{
		Name:        name,
		Slug:        name,
		Source:      "git",
		GitURL:      url,
		Ref:         ref,
		LocalPath:   dest,
		AddedAt:     time.Now(),
		LastUpdated: time.Now(),
		LastCommit:  commit,
		Libraries:   libKeys,
	}
	reg.Libraries = append(reg.Libraries, entry)
	if err := library.SaveRegistry(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("Added library %q (%d libraries, %d commands)\n", name, len(libKeys), totalSpecs)
	for _, key := range libKeys {
		lib := manifest.Libraries[key]
		fmt.Printf("  %s — %s\n", key, lib.Name)
	}
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

func newLibraryRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed library",
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

			// Clean up git clone directory
			if entry.Source == "git" && entry.LocalPath != "" {
				_ = os.RemoveAll(entry.LocalPath)
			}

			// Uninstall from API if registry-backed and logged in
			if entry.Source == "registry" && auth.IsLoggedIn() {
				cfg, err := config.Load()
				if err == nil {
					c := client.New(resolveAPIURL(cfg))
					_ = c.UninstallLibrary(entry.Owner, entry.Slug)
				}
			}

			library.Remove(reg, name)
			if err := library.SaveRegistry(reg); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("Removed library %q.\n", name)
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
				for _, entry := range reg.Libraries {
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
					reg.Libraries = append(reg.Libraries, library.LibraryEntry{
						Name:        item.Library,
						Owner:       item.LibraryOwner,
						Slug:        item.Library,
						Source:      "registry",
						LastUpdated: catalog.SyncedAt,
					})
				}
			}

			if len(reg.Libraries) == 0 {
				fmt.Println("No libraries installed. Run 'my library add <identifier>' to install one.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(reg.Libraries, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tSOURCE\tUPDATED")
			for _, entry := range reg.Libraries {
				updated := entry.LastUpdated.Format("2006-01-02 15:04")
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Name, entry.Source, updated)
			}
			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

func newLibraryUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update [name]",
		Short: "Update installed libraries",
		Long:  "Update all libraries, or a specific one by name. For git libraries, runs git pull. For registry libraries, syncs from the API.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			// API sync: always sync catalog when logged in (includes all server-side installs)
			if auth.IsLoggedIn() {
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				c := client.New(resolveAPIURL(cfg))
				fetched, err := cache.Sync(c, true)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: API sync failed: %v\n", err)
				} else if fetched == 0 {
					fmt.Println("Registry: already up to date.")
				} else {
					fmt.Printf("Registry: synced %d command(s).\n", fetched)
				}
			} else {
				fmt.Println("Registry: skipped (not logged in)")
			}

			// Git libraries: update
			var targets []int
			if len(args) == 1 {
				for i := range reg.Libraries {
					if reg.Libraries[i].Name == args[0] {
						targets = append(targets, i)
						break
					}
				}
				if len(targets) == 0 {
					return fmt.Errorf("library %q not found", args[0])
				}
			} else {
				for i := range reg.Libraries {
					if reg.Libraries[i].Source == "git" {
						targets = append(targets, i)
					}
				}
			}

			updated := 0
			for _, i := range targets {
				entry := &reg.Libraries[i]
				if entry.Source != "git" {
					continue
				}

				dest := entry.LocalPath
				if dest == "" {
					continue
				}

				oldCommit := entry.LastCommit

				fmt.Printf("Updating %s...\n", entry.Name)
				if err := shelf.Pull(dest); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to update %q: %v\n", entry.Name, err)
					continue
				}

				newCommit, _ := shelf.HeadCommit(dest)
				if newCommit == oldCommit {
					fmt.Printf("  %s is already up to date (%s)\n", entry.Name, oldCommit)
					continue
				}

				// Re-validate manifest
				manifest, err := shelf.LoadManifest(dest)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: %q manifest invalid after update: %v\n", entry.Name, err)
					continue
				}

				entry.LastUpdated = time.Now()
				entry.LastCommit = newCommit

				var libKeys []string
				for key := range manifest.Libraries {
					libKeys = append(libKeys, key)
				}
				entry.Libraries = libKeys

				fmt.Printf("  Updated %s: %s -> %s\n", entry.Name, oldCommit, newCommit)
				updated++
			}

			if len(targets) > 0 {
				if err := library.SaveRegistry(reg); err != nil {
					return fmt.Errorf("save registry: %w", err)
				}
			}

			if updated == 0 && len(targets) > 0 {
				fmt.Println("Git libraries: already up to date.")
			}

			return nil
		},
	}
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
			if !shelf.TagExists(cwd, tag) {
				return fmt.Errorf("tag %q not found (run 'git tag %s' first)", tag, tag)
			}

			// Get commit hash for the tag
			commitHash, err := shelf.TagCommitHash(cwd, tag)
			if err != nil {
				return fmt.Errorf("get tag commit: %w", err)
			}

			// Extract tag contents to a temp directory
			tmpDir, err := os.MkdirTemp("", "my-release-*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if err := shelf.ArchiveTag(cwd, tag, tmpDir); err != nil {
				return fmt.Errorf("extract tag: %w", err)
			}

			// Load manifest from the tagged content
			manifest, err := shelf.LoadManifest(tmpDir)
			if err != nil {
				return fmt.Errorf("no valid manifest at tag %s: %w", tag, err)
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))

			// Detect git remote URL for metadata
			var gitURL string
			if remote, err := getGitRemoteURL(cwd); err == nil {
				gitURL = remote
			}

			// Release each library in the manifest
			for libKey, libDef := range manifest.Libraries {
				items, err := shelf.DiscoverSpecs(tmpDir, libKey, libDef)
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
					fmt.Printf("Source:  %s\n", entry.Source)
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

					if entry.Source == "git" && entry.LocalPath != "" {
						// Show libraries and commands from manifest
						manifest, err := shelf.LoadManifest(entry.LocalPath)
						if err == nil {
							totalCmds := 0
							for libKey, libDef := range manifest.Libraries {
								items, _ := shelf.DiscoverSpecs(entry.LocalPath, libKey, libDef)
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
