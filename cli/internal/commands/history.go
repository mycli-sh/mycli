package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/history"
)

func newHistoryCmd() *cobra.Command {
	var n int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show run history",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := history.List(n)
			if err != nil {
				return fmt.Errorf("failed to read history: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No history yet.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TIME\tSLUG\tVERSION\tEXIT\tDURATION")
			for _, e := range entries {
				_, _ = fmt.Fprintf(w, "%s\t%s\tv%d\t%d\t%dms\n",
					e.Timestamp.Format("01-02 15:04"),
					e.Slug,
					e.Version,
					e.ExitCode,
					e.DurationMs,
				)
			}
			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().IntVarP(&n, "last", "n", 20, "number of entries to show")
	return cmd
}
