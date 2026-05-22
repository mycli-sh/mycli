package commands

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles (named sets of libraries)",
		Long: `Profiles are server-synced named configurations that bundle a set of libraries.
Use them to organize libraries by context (work, personal, CI) and scope API tokens.`,
	}

	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileSetCmd())
	cmd.AddCommand(newProfileShowCmd())
	cmd.AddCommand(newProfileDeleteCmd())
	cmd.AddCommand(newProfileSyncCmd())
	cmd.AddCommand(newProfileAddLibraryCmd())
	cmd.AddCommand(newProfileRemoveLibraryCmd())

	return cmd
}

func newProfileCreateCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create <slug>",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
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

			slug := args[0]
			if name == "" {
				name = slug
			}

			profile, err := c.CreateProfile(slug, name, "")
			if err != nil {
				return fmt.Errorf("failed to create profile: %w", err)
			}

			fmt.Printf("Created profile %q (id: %s)\n", profile.Slug, profile.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "display name for the profile")

	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			activeProfile := cfg.GetActiveProfile()

			// Try API first when logged in; fall back to cached profiles on network failure.
			if auth.IsLoggedIn() {
				c := client.New(resolveAPIURL(cfg))
				defer c.Close()

				resp, listErr := c.ListProfiles()
				if listErr == nil {
					if len(resp.Profiles) == 0 {
						fmt.Println("No profiles. Create one with 'my cli profile create <slug>'.")
						return nil
					}
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					_, _ = fmt.Fprintln(w, "SLUG\tNAME\tDEFAULT\tACTIVE")
					for _, p := range resp.Profiles {
						isDefault := ""
						if p.IsDefault {
							isDefault = "yes"
						}
						isActive := ""
						if p.Slug == activeProfile {
							isActive = "*"
						}
						_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Slug, p.Name, isDefault, isActive)
					}
					_ = w.Flush()
					return nil
				}
				// If the server responded with a structured error, surface it.
				var apiErr *client.APIError
				if errors.As(listErr, &apiErr) {
					return fmt.Errorf("failed to list profiles: %w", listErr)
				}
				fmt.Fprintf(os.Stderr, "warning: API unreachable (%v); showing cached profiles\n", listErr)
			}

			cached := cache.ListCachedProfiles()
			if len(cached) == 0 {
				fmt.Println("No cached profiles. Connect to the server and run 'my cli profile sync'.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "SLUG\tACTIVE\tSOURCE")
			for _, slug := range cached {
				marker := ""
				if slug == activeProfile {
					marker = "*"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", slug, marker, "(cached)")
			}
			_ = w.Flush()
			return nil
		},
	}
}

func newProfileSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <slug>",
		Short: "Switch the locally active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			slug := args[0]

			// Offline-tolerant resolution:
			//   1. If the profile has a local cache, accept it immediately.
			//   2. If logged in, verify against the server.
			//   3. On a structured API error (e.g. 404) refuse to switch.
			//   4. On a network/transport error, save anyway with a warning so
			//      offline switching keeps working.
			switch {
			case cache.HasCachedProfile(slug):
				// trust the cache
			case auth.IsLoggedIn():
				c := client.New(resolveAPIURL(cfg))
				defer c.Close()
				if _, err := c.GetProfile(slug); err != nil {
					var apiErr *client.APIError
					if errors.As(err, &apiErr) {
						return fmt.Errorf("profile %q not found: %w", slug, err)
					}
					fmt.Fprintf(os.Stderr, "warning: API unreachable (%v); saving profile locally\n", err)
				}
			default:
				return fmt.Errorf("profile %q is not cached locally; log in and sync first", slug)
			}

			cfg.ActiveProfile = slug
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Printf("Active profile set to %q.\n", slug)
			return nil
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [slug]",
		Short: "Show profile details and libraries",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			slug := cfg.GetActiveProfile()
			if len(args) > 0 {
				slug = args[0]
			}
			if slug == "" {
				return fmt.Errorf("no profile specified")
			}

			if auth.IsLoggedIn() {
				c := client.New(resolveAPIURL(cfg))
				defer c.Close()

				resp, getErr := c.GetProfile(slug)
				if getErr == nil {
					printProfile(resp.Profile.Slug, resp.Profile.Name, resp.Profile.Description, resp.Profile.IsDefault)
					printProfileLibraries(resp.Libraries)
					return nil
				}
				var apiErr *client.APIError
				if errors.As(getErr, &apiErr) {
					return fmt.Errorf("profile %q not found: %w", slug, getErr)
				}
				fmt.Fprintf(os.Stderr, "warning: API unreachable (%v); showing cached view\n", getErr)
			}

			catalog, err := cache.GetCatalog(slug)
			if err != nil {
				return fmt.Errorf("no cached data for profile %q (run 'my cli profile sync %s' when online)", slug, slug)
			}
			printProfile(slug, "", "", slug == config.DefaultProfileSlug)
			seen := map[string]bool{}
			var libs []struct{ Owner, Slug string }
			for _, item := range catalog.Items {
				if item.Library == "" {
					continue
				}
				key := item.LibraryOwner + "/" + item.Library
				if seen[key] {
					continue
				}
				seen[key] = true
				libs = append(libs, struct{ Owner, Slug string }{item.LibraryOwner, item.Library})
			}
			if len(libs) == 0 {
				fmt.Println("\nNo libraries in this profile (cached).")
				return nil
			}
			fmt.Printf("\nLibraries (%d, cached):\n", len(libs))
			for _, lib := range libs {
				if lib.Owner != "" {
					fmt.Printf("  %s/%s\n", lib.Owner, lib.Slug)
				} else {
					fmt.Printf("  %s\n", lib.Slug)
				}
			}
			return nil
		},
	}
}

func printProfile(slug, name, description string, isDefault bool) {
	fmt.Printf("Profile: %s\n", slug)
	if name != "" {
		fmt.Printf("Name:    %s\n", name)
	}
	if description != "" {
		fmt.Printf("About:   %s\n", description)
	}
	def := "no"
	if isDefault {
		def = "yes"
	}
	fmt.Printf("Default: %s\n", def)
}

func printProfileLibraries(raw json.RawMessage) {
	var libs []json.RawMessage
	if err := json.Unmarshal(raw, &libs); err != nil || len(libs) == 0 {
		fmt.Println("\nNo libraries in this profile.")
		return
	}
	fmt.Printf("\nLibraries (%d):\n", len(libs))
	for _, raw := range libs {
		var lib struct {
			Slug  string `json:"slug"`
			Name  string `json:"name"`
			Owner string `json:"owner"`
		}
		if json.Unmarshal(raw, &lib) == nil {
			if lib.Owner != "" {
				fmt.Printf("  %s/%s — %s\n", lib.Owner, lib.Slug, lib.Name)
			} else {
				fmt.Printf("  %s — %s\n", lib.Slug, lib.Name)
			}
		}
	}
}

func newProfileDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <slug>",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
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

			slug := args[0]
			if err := c.DeleteProfile(slug, force); err != nil {
				var apiErr *client.APIError
				if errors.As(err, &apiErr) && apiErr.Code == "HAS_SCOPED_TOKENS" {
					fmt.Fprintf(os.Stderr, "%s\n", apiErr.Message)
					fmt.Fprint(os.Stderr, "Continue? [y/N] ")
					reader := bufio.NewReader(os.Stdin)
					answer, _ := reader.ReadString('\n')
					if strings.TrimSpace(strings.ToLower(answer)) != "y" {
						return fmt.Errorf("aborted")
					}
					if err := c.DeleteProfile(slug, true); err != nil {
						return fmt.Errorf("failed to delete profile: %w", err)
					}
				} else {
					return fmt.Errorf("failed to delete profile: %w", err)
				}
			}

			// Clear active profile if it matches
			if cfg.ActiveProfile == slug {
				cfg.ActiveProfile = ""
				_ = cfg.Save()
			}

			fmt.Printf("Deleted profile %q.\n", slug)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force deletion even if scoped tokens exist")

	return cmd
}

func newProfileSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [slug]",
		Short: "Sync libraries for a profile (key CI command)",
		Long: `Downloads all library specs for a profile so commands are available locally.

The profile is resolved from: positional arg > MY_PROFILE env > active profile config > "default".

Examples:
  my cli profile sync ci-deploy
  MY_PROFILE=ci-deploy my cli profile sync
  my cli profile sync                       # syncs the active profile`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (set MY_API_TOKEN or run 'my cli login')")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			profile := cfg.GetActiveProfile()
			if len(args) > 0 {
				profile = args[0]
			}

			fmt.Printf("Syncing profile %q...\n", profile)

			fetched, err := cache.SyncProfile(c, profile, true)
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

func newProfileAddLibraryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-library <library>",
		Short: "Add a library to the active profile",
		Long: `Add a library to the currently active profile.

Examples:
  my cli profile add-library kubernetes
  my cli profile add-library owner/name`,
		Args: cobra.ExactArgs(1),
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

			profileSlug := cfg.GetActiveProfile()
			library := args[0]
			if err := c.AddLibraryToProfile(profileSlug, library); err != nil {
				return fmt.Errorf("failed to add library: %w", err)
			}

			fmt.Printf("Added %q to profile %q.\n", library, profileSlug)
			return nil
		},
	}
}

func newProfileRemoveLibraryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove-library <owner/slug>",
		Short: "Remove a library from the active profile",
		Args:  cobra.ExactArgs(1),
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

			profileSlug := cfg.GetActiveProfile()
			identifier := args[0]
			owner, libSlug := parseOwnerSlug(identifier)

			if err := c.RemoveLibraryFromProfile(profileSlug, owner, libSlug); err != nil {
				return fmt.Errorf("failed to remove library: %w", err)
			}

			fmt.Printf("Removed %q from profile %q.\n", identifier, profileSlug)
			return nil
		},
	}
}
