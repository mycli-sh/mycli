package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current user info",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))

			user, err := c.GetMe()
			if err != nil {
				return fmt.Errorf("failed to get user info: %w", err)
			}

			fmt.Printf("ID:    %s\n", user.ID)
			fmt.Printf("Email: %s\n", user.Email)
			return nil
		},
	}
}
