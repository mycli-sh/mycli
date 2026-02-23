package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/config"
)

func newSetAPIURLCmd() *cobra.Command {
	var reset bool

	cmd := &cobra.Command{
		Use:   "set-api-url [url]",
		Short: "Persist a custom API URL in config",
		Long:  "Sets the API URL in ~/.my/config.json. Pass --reset to revert to the build default.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if reset {
				cfg.APIURL = config.DefaultAPI()
			} else if len(args) == 1 {
				cfg.APIURL = args[0]
			} else {
				return fmt.Errorf("provide a URL or use --reset")
			}

			if err := cfg.Save(); err != nil {
				return err
			}

			fmt.Printf("API URL set to %s\n", cfg.APIURL)
			return nil
		},
	}

	cmd.Flags().BoolVar(&reset, "reset", false, "Reset to the build default URL")

	return cmd
}
