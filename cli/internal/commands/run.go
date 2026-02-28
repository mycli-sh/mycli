package commands

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/engine"
	"mycli.sh/cli/internal/history"
	"mycli.sh/cli/internal/library"
	"mycli.sh/cli/internal/termui"
	"mycli.sh/pkg/spec"
)

func newRunCmd() *cobra.Command {
	var yes bool
	var file string

	cmd := &cobra.Command{
		Use:   "run [slug] [args...]",
		Short: "Run a command",
		Long: `Run a command by slug, directly from a file with -f, or pick interactively.

Without arguments, opens an interactive picker to search and select a command.
Use -f to run a spec file directly without pushing:

  my cli run -f command.yaml
  my cli run -f .                  (auto-detect spec in current directory)`,
		Args: cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			catalog, err := cache.GetCatalog()
			if err != nil || catalog == nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var slugs []string
			for _, item := range catalog.Items {
				if item.Library == "" {
					slugs = append(slugs, item.Slug)
				}
			}
			return slugs, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if file != "" {
				if file == "." {
					file = detectLocalSpecFile()
					if file == "" {
						return fmt.Errorf("no spec file found in current directory")
					}
				}
				return runFromFile(file, args, yes)
			}

			if len(args) == 0 {
				return runInteractiveOrError(yes)
			}

			slug := args[0]
			cmdArgs := args[1:]

			s, err := resolveSpec(slug)
			if err != nil {
				return err
			}

			item, _ := cache.GetCatalogItem(slug)
			return executeAndRecord(s, slug, cmdArgs, yes, item)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompts")
	cmd.Flags().StringVarP(&file, "file", "f", "", "run a command directly from a spec file")
	return cmd
}

// detectLocalSpecFile checks CWD for command.yaml/.yml/.json and returns the filename, or "" if none found.
func detectLocalSpecFile() string {
	for _, name := range []string{"command.yaml", "command.yml", "command.json"} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}

// runInteractiveOrError launches the interactive picker on a TTY, or prints a fallback table.
func runInteractiveOrError(yes bool) error {
	if !termui.IsTTY() {
		return runNonTTYFallback()
	}

	localFile := detectLocalSpecFile()
	item, err := runPicker(localFile)
	if err != nil {
		return err
	}
	if item == nil {
		return nil // user cancelled
	}
	return runPickerSelection(item, yes)
}

// runPickerSelection dispatches execution based on the picker item's source.
func runPickerSelection(item *pickerItem, yes bool) error {
	switch item.Source {
	case sourceLocal:
		return runFromFile(item.FilePath, nil, yes)
	case sourcePersonal:
		s, err := resolveSpec(item.CatalogItem.Slug)
		if err != nil {
			return err
		}
		return executeAndRecord(s, item.CatalogItem.Slug, nil, yes, item.CatalogItem)
	case sourceLibrary:
		s, err := cache.GetLibrarySpec(item.LibraryKey, item.CatalogItem.Slug)
		if err != nil {
			return err
		}
		return executeAndRecord(s, item.Slug, nil, yes, item.CatalogItem)
	case sourceGit:
		s, err := library.GetSpec(item.FilePath)
		if err != nil {
			return err
		}
		return executeAndRecord(s, item.Slug, nil, yes, nil)
	default:
		return fmt.Errorf("unknown source type")
	}
}

// runNonTTYFallback prints a tabular list of commands for non-interactive use.
func runNonTTYFallback() error {
	items := loadPickerItems()
	if len(items) == 0 {
		return fmt.Errorf("no commands available; provide a slug argument: my cli run <slug>")
	}

	fmt.Fprintln(os.Stderr, "Available commands:")
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SLUG\tSOURCE\tDESCRIPTION")
	for _, item := range items {
		desc := item.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", item.Slug, item.SourceLabel, desc)
	}
	w.Flush()
	return fmt.Errorf("provide a slug argument: my cli run <slug>")
}

func runFromFile(file string, args []string, yes bool) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", file, err)
	}

	s, err := spec.Parse(data)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}

	return executeAndRecord(s, s.Metadata.Slug, args, yes, nil)
}

func executeAndRecord(s *spec.CommandSpec, slug string, args []string, yes bool, item *client.CatalogItem) error {
	result, err := engine.Execute(s, args, engine.ExecOpts{Yes: yes})

	if result != nil {
		entry := history.Entry{
			Timestamp:  time.Now(),
			Slug:       slug,
			ExitCode:   result.ExitCode,
			DurationMs: result.DurationMs,
		}
		if item != nil {
			entry.CommandID = item.CommandID
			entry.Version = item.Version
		}
		if herr := history.Record(entry); herr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to record history: %v\n", herr)
		}
	}

	if err != nil {
		exitCode := 1
		if result != nil && result.ExitCode != 0 {
			exitCode = result.ExitCode
		}
		return &engine.ExitCodeError{Code: exitCode, Err: err}
	}
	return nil
}
