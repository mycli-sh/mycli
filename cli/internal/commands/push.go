package commands

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
	"mycli.sh/pkg/spec"
)

// specFileNames is the set of recognized spec file basenames.
var specFileNames = map[string]bool{
	"command.yaml": true,
	"command.yml":  true,
	"command.json": true,
}

func newPushCmd() *cobra.Command {
	var message string
	var file string
	var dir string

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push a command spec to the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir != "" && cmd.Flags().Changed("file") {
				return fmt.Errorf("--dir and --file are mutually exclusive")
			}

			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))

			if dir != "" {
				return pushDir(c, dir, message)
			}

			// Resolve spec file: if --file was not explicitly set, auto-detect
			specFile := file
			if !cmd.Flags().Changed("file") {
				specFile = detectSpecFile()
			}

			return pushSpecFile(c, specFile, message)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "version message")
	cmd.Flags().StringVarP(&file, "file", "f", "command.json", "spec file to push")
	cmd.Flags().StringVar(&dir, "dir", "", "push all spec files found in directory tree")
	return cmd
}

// pushSpecFile reads, validates, and pushes a single spec file.
func pushSpecFile(c *client.Client, specFile string, message string) error {
	data, err := os.ReadFile(specFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", specFile, err)
	}

	s, err := spec.Parse(data)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}

	jsonData, err := spec.ToJSON(data)
	if err != nil {
		return fmt.Errorf("failed to convert spec to JSON: %w", err)
	}

	var commandID string
	existing, err := c.GetCommandBySlug(s.Metadata.Slug)
	if err != nil {
		return fmt.Errorf("failed to look up command: %w", err)
	}
	if existing != nil {
		commandID = existing.ID
	}

	if commandID == "" {
		created, err := c.CreateCommand(&client.CreateCommandRequest{
			Name:        s.Metadata.Name,
			Slug:        s.Metadata.Slug,
			Description: s.Metadata.Description,
			Tags:        s.Metadata.Tags,
		})
		if err != nil {
			return fmt.Errorf("failed to create command: %w", err)
		}
		commandID = created.ID
		fmt.Printf("Created command %q (%s)\n", s.Metadata.Name, commandID)
	}

	version, err := c.PublishVersion(commandID, &client.PublishVersionRequest{
		SpecJSON: json.RawMessage(jsonData),
		Message:  message,
	})
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.Code == "SPEC_IDENTICAL" {
			fmt.Println("No changes — spec is identical to the latest version.")
			return nil
		}
		return fmt.Errorf("failed to publish version: %w", err)
	}

	fmt.Printf("Published version %d (hash: %s)\n", version.Version, version.SpecHash)
	return nil
}

// discoverSpecFiles walks dir and returns paths to all recognized spec files.
func discoverSpecFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if specFileNames[d.Name()] {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// pushDir discovers spec files in a directory tree and pushes each one.
func pushDir(c *client.Client, dir string, message string) error {
	files, err := discoverSpecFiles(dir)
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No spec files found.")
		return nil
	}

	var succeeded, failed int
	var firstErr error
	for _, f := range files {
		fmt.Printf("Pushing %s ...\n", f)
		if err := pushSpecFile(c, f, message); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		succeeded++
	}

	fmt.Printf("\n%d succeeded, %d failed\n", succeeded, failed)
	if firstErr != nil {
		return fmt.Errorf("some pushes failed")
	}
	return nil
}

// detectSpecFile returns the first existing spec file from the preferred order.
func detectSpecFile() string {
	for _, name := range []string{"command.yaml", "command.yml", "command.json"} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return "command.yaml"
}
