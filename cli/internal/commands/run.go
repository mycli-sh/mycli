package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/engine"
	"mycli.sh/cli/internal/history"
	"mycli.sh/pkg/spec"
)

func newRunCmd() *cobra.Command {
	var yes bool
	var file string

	cmd := &cobra.Command{
		Use:   "run [slug] [args...]",
		Short: "Run a command",
		Long:  "Run a command from the cache by slug, or directly from a file with -f.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file != "" {
				return runFromFile(file, args, yes)
			}

			if len(args) == 0 {
				return fmt.Errorf("requires a slug argument or --file flag")
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
