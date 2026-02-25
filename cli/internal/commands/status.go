package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/config"
	"mycli.sh/cli/internal/library"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show mycli status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			fmt.Printf("API URL:    %s\n", resolveAPIURL(cfg))
			fmt.Printf("Logged in:  %v\n", auth.IsLoggedIn())

			lastSync := cache.LastSyncTime()
			if lastSync.IsZero() {
				fmt.Println("Last sync:  never")
			} else {
				fmt.Printf("Last sync:  %s\n", lastSync.Format("2006-01-02 15:04:05"))
			}

			catalog, err := cache.GetCatalog()
			if err == nil {
				fmt.Printf("Commands:   %d cached\n", len(catalog.Items))
			} else {
				fmt.Println("Commands:   0 cached")
			}

			// Library info
			reg, err := library.LoadRegistry()
			if err == nil && len(reg.Sources) > 0 {
				allLibs, _ := library.GetAllLibraries()
				totalCmds := 0
				for _, libCatalog := range allLibs {
					totalCmds += len(libCatalog.Items)
				}
				fmt.Printf("Libraries:  %d (%d commands)\n", len(reg.Sources), totalCmds)
			} else {
				fmt.Println("Libraries:  0")
			}

			return nil
		},
	}
}
