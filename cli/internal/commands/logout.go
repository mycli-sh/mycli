package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
)

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out of mycli",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Best-effort server-side session revocation
			tokens, _ := auth.LoadTokens()
			if tokens != nil && tokens.AccessToken != "" {
				cfg, _ := config.Load()
				if cfg != nil {
					c := client.New(resolveAPIURL(cfg))
					_ = c.Logout(tokens.RefreshToken) // ignore errors (e.g. offline)
				}
			}

			if err := auth.ClearTokens(); err != nil {
				return fmt.Errorf("failed to logout: %w", err)
			}
			fmt.Println("Logged out successfully.")
			return nil
		},
	}
}
