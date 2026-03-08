package commands

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/update"
)

const installScriptURL = "https://raw.githubusercontent.com/mycli-sh/mycli/main/scripts/install.sh"

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update mycli to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if client.InstallMethod == "brew" {
				fmt.Println("Run 'brew upgrade mycli' to upgrade.")
				return nil
			}
			if client.Version != "dev" {
				latest, err := update.FetchLatestVersion()
				if err == nil {
					current := update.NormalizeVersion(client.Version)
					if current != "" && update.CompareVersions(current, latest) >= 0 {
						fmt.Printf("Already up to date (v%s)\n", current)
						return nil
					}
				}
			}
			return runInstallScript()
		},
	}
}

func runInstallScript() error {
	fmt.Println("Downloading install script...")

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get(installScriptURL)
	if err != nil {
		return fmt.Errorf("failed to download install script: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download install script: HTTP %d", resp.StatusCode)
	}

	script, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read install script: %w", err)
	}

	sh := exec.Command("sh")
	sh.Stdin = bytes.NewReader(script)
	sh.Stdout = os.Stdout
	sh.Stderr = os.Stderr

	if err := sh.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println("Update complete!")
	return nil
}
