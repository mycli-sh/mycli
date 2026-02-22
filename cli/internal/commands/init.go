package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var invalidSlugChars = regexp.MustCompile(`[^a-z0-9-]`)

func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Create a new command spec file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "my-command"
			hasName := len(args) > 0
			if hasName {
				name = args[0]
			}

			// Normalize slug: lowercase, spaces to hyphens, strip invalid chars
			slug := strings.ToLower(name)
			slug = strings.ReplaceAll(slug, " ", "-")
			slug = invalidSlugChars.ReplaceAllString(slug, "")
			// Trim leading hyphens/digits to ensure it starts with a letter
			slug = strings.TrimLeft(slug, "-0123456789")
			if slug == "" {
				slug = "my-command"
			}
			if !slugRe.MatchString(slug) {
				return fmt.Errorf("could not derive a valid slug from %q", name)
			}
			if slug != name {
				fmt.Printf("Note: slug normalized to %q\n", slug)
			}

			yaml := fmt.Sprintf(`schemaVersion: 1
kind: command
metadata:
  name: %s
  slug: %s
  description: A new command
args:
  positional: []
  flags: []
steps:
  - name: run
    run: echo 'Hello from %s'
`, name, slug, name)

			var filename string
			if hasName {
				// Create in a subdirectory named after the slug
				dir := slug
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
				filename = filepath.Join(dir, "command.yaml")
			} else {
				filename = "command.yaml"
			}

			// Check if file already exists (unless --force)
			if !force {
				if _, err := os.Stat(filename); err == nil {
					return fmt.Errorf("%s already exists (use --force to overwrite)", filename)
				}
			}

			if err := os.WriteFile(filename, []byte(yaml), 0600); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}

			fmt.Printf("Created %s\n", filename)
			fmt.Println("Edit the file, then run 'my cli push' to publish.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing spec file")
	return cmd
}
