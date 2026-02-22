package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out of mycli",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.ClearTokens(); err != nil {
				return fmt.Errorf("failed to logout: %w", err)
			}
			fmt.Println("Logged out successfully.")
			return nil
		},
	}
}
