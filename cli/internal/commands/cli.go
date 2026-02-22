package commands

import "github.com/spf13/cobra"

func newCliCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Manage your commands and auth",
	}

	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newWhoamiCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newHistoryCmd())

	return cmd
}
