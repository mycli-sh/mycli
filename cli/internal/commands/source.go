package commands

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/library"
)

func newSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Manage git-backed command sources",
		Long:  "Add, remove, list, and update git-backed command sources.",
	}

	cmd.AddCommand(newSourceAddCmd())
	cmd.AddCommand(newSourceRemoveCmd())
	cmd.AddCommand(newSourceListCmd())
	cmd.AddCommand(newSourceUpdateCmd())

	return cmd
}

func newSourceAddCmd() *cobra.Command {
	var ref string
	var name string

	cmd := &cobra.Command{
		Use:   "add <git-url>",
		Short: "Add a git-backed command source",
		Long: `Clone a git repository as a command source.

  my source add https://github.com/user/my-library.git
  my source add git@github.com:user/my-library.git`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			if !isGitURL(url) {
				return fmt.Errorf("expected a git URL (use 'my library install' for registry libraries)")
			}
			return addGitSource(url, ref, name)
		},
	}

	cmd.Flags().StringVar(&ref, "ref", "", "git branch or tag to checkout")
	cmd.Flags().StringVar(&name, "name", "", "alias for the source (defaults to manifest name)")
	return cmd
}

func addGitSource(url, ref, nameOverride string) error {
	reg, err := library.LoadRegistry()
	if err != nil {
		return err
	}

	// Derive local path
	dest, err := library.RepoLocalPath(url)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("directory already exists: %s", dest)
	}

	// Clone
	fmt.Printf("Cloning %s...\n", url)
	if err := library.Clone(url, dest, ref); err != nil {
		return err
	}

	// Parse manifest
	manifest, err := library.LoadManifest(dest)
	if err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("invalid source: %w", err)
	}

	name := nameOverride
	if name == "" {
		name = manifest.Name
	}

	// Check for duplicate name
	if library.FindByName(reg, name) != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("source %q already exists (use --name to set a different alias)", name)
	}

	// Discover and validate all specs
	totalSpecs := 0
	var libKeys []string
	for libKey, libDef := range manifest.Libraries {
		items, err := library.DiscoverSpecs(dest, libKey, libDef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		totalSpecs += len(items)
		libKeys = append(libKeys, libKey)
	}

	if totalSpecs == 0 {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("no valid commands found in source (check warnings above)")
	}

	// Get HEAD commit
	commit, _ := library.HeadCommit(dest)

	// Save to registry
	entry := library.SourceEntry{
		Name:        name,
		Slug:        name,
		Kind:        "git",
		GitURL:      url,
		Ref:         ref,
		LocalPath:   dest,
		AddedAt:     time.Now(),
		LastUpdated: time.Now(),
		LastCommit:  commit,
		Libraries:   libKeys,
	}
	reg.Sources = append(reg.Sources, entry)
	if err := library.SaveRegistry(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("Added source %q (%d libraries, %d commands)\n", name, len(libKeys), totalSpecs)
	for _, key := range libKeys {
		lib := manifest.Libraries[key]
		fmt.Printf("  %s — %s\n", key, lib.Name)
	}
	return nil
}

func newSourceRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a git-backed source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			entry := library.FindByName(reg, name)
			if entry == nil {
				return fmt.Errorf("source %q not found", name)
			}

			if entry.Kind != "git" {
				return fmt.Errorf("%q is not a git source (use 'my library uninstall' for registry libraries)", name)
			}

			// Clean up git clone directory
			if entry.LocalPath != "" {
				_ = os.RemoveAll(entry.LocalPath)
			}

			library.Remove(reg, name)
			if err := library.SaveRegistry(reg); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("Removed source %q.\n", name)
			return nil
		},
	}
}

func newSourceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed git sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			// Filter to git sources only
			var gitSources []library.SourceEntry
			for _, entry := range reg.Sources {
				if entry.Kind == "git" {
					gitSources = append(gitSources, entry)
				}
			}

			if len(gitSources) == 0 {
				fmt.Println("No git sources installed. Run 'my source add <git-url>' to add one.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tURL\tUPDATED")
			for _, entry := range gitSources {
				updated := entry.LastUpdated.Format("2006-01-02 15:04")
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Name, entry.GitURL, updated)
			}
			_ = w.Flush()
			return nil
		},
	}
}

func newSourceUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update [name]",
		Short: "Update git sources",
		Long:  "Pull the latest changes from all git sources, or a specific one by name.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			var targets []int
			if len(args) == 1 {
				for i := range reg.Sources {
					if reg.Sources[i].Name == args[0] {
						if reg.Sources[i].Kind != "git" {
							return fmt.Errorf("%q is not a git source", args[0])
						}
						targets = append(targets, i)
						break
					}
				}
				if len(targets) == 0 {
					return fmt.Errorf("source %q not found", args[0])
				}
			} else {
				for i := range reg.Sources {
					if reg.Sources[i].Kind == "git" {
						targets = append(targets, i)
					}
				}
			}

			if len(targets) == 0 {
				fmt.Println("No git sources to update.")
				return nil
			}

			updated := 0
			for _, i := range targets {
				entry := &reg.Sources[i]

				dest := entry.LocalPath
				if dest == "" {
					continue
				}

				oldCommit := entry.LastCommit

				fmt.Printf("Updating %s...\n", entry.Name)
				if err := library.Pull(dest); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to update %q: %v\n", entry.Name, err)
					continue
				}

				newCommit, _ := library.HeadCommit(dest)
				if newCommit == oldCommit {
					fmt.Printf("  %s is already up to date (%s)\n", entry.Name, oldCommit)
					continue
				}

				// Re-validate manifest
				manifest, err := library.LoadManifest(dest)
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

			if err := library.SaveRegistry(reg); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			if updated == 0 {
				fmt.Println("All sources are up to date.")
			}

			return nil
		},
	}
}
