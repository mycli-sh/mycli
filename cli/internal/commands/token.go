package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage API tokens for CI/CD and automation",
	}

	cmd.AddCommand(newTokenCreateCmd())
	cmd.AddCommand(newTokenListCmd())
	cmd.AddCommand(newTokenRevokeCmd())

	return cmd
}

func newTokenCreateCmd() *cobra.Command {
	var expiresIn string
	var profileSlug string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new API token",
		Long: `Create a long-lived API token for non-interactive use (CI/CD, scripts).

The raw token is shown only once — save it securely.

Examples:
  my cli token create ci-deploy
  my cli token create ci-deploy --expires-in 90d
  my cli token create ci-deploy --profile ci-deploy`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			req := &client.CreateTokenRequest{
				Name:      args[0],
				ExpiresIn: expiresIn,
			}

			// Resolve profile slug to profile ID if provided
			if profileSlug != "" {
				profile, err := c.GetProfile(profileSlug)
				if err != nil {
					return fmt.Errorf("profile %q not found: %w", profileSlug, err)
				}
				req.ProfileID = profile.Profile.ID
			}

			resp, err := c.CreateToken(req)
			if err != nil {
				return fmt.Errorf("failed to create token: %w", err)
			}

			fmt.Println("Token created successfully!")
			fmt.Println()
			fmt.Printf("  Token: %s\n", resp.Token)
			fmt.Printf("  Name:  %s\n", resp.Name)
			if resp.ExpiresAt != nil {
				fmt.Printf("  Expires: %s\n", *resp.ExpiresAt)
			} else {
				fmt.Printf("  Expires: never\n")
			}
			fmt.Println()
			fmt.Println("Save this token — it won't be shown again.")
			fmt.Println("Use it with: export MY_API_TOKEN=<token>")

			return nil
		},
	}

	cmd.Flags().StringVar(&expiresIn, "expires-in", "", "token expiry (e.g. 30d, 90d, 1y)")
	cmd.Flags().StringVar(&profileSlug, "profile", "", "scope token to a profile")

	return cmd
}

func newTokenListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List your API tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			resp, err := c.ListTokens()
			if err != nil {
				return fmt.Errorf("failed to list tokens: %w", err)
			}

			if len(resp.Tokens) == 0 {
				fmt.Println("No API tokens.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tNAME\tPREFIX\tLAST USED\tEXPIRES")
			for _, t := range resp.Tokens {
				expires := "never"
				if t.ExpiresAt != nil {
					expires = *t.ExpiresAt
				}
				lastUsed := "never"
				if t.LastUsedAt != nil {
					lastUsed = *t.LastUsedAt
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					t.ID, t.Name, t.TokenPrefix, lastUsed, expires)
			}
			_ = w.Flush()
			return nil
		},
	}
}

func newTokenRevokeCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "revoke <id|name>",
		Short: "Revoke an API token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			idOrName := args[0]

			// Resolve to a (tokenID, name, prefix) triple by listing tokens.
			// The name/prefix is what we display in the confirmation prompt.
			tokens, err := c.ListTokens()
			if err != nil {
				return fmt.Errorf("failed to list tokens: %w", err)
			}
			var match *client.APITokenInfo
			if _, parseErr := uuid.Parse(idOrName); parseErr == nil {
				for i := range tokens.Tokens {
					if tokens.Tokens[i].ID == idOrName {
						match = &tokens.Tokens[i]
						break
					}
				}
			} else {
				for i := range tokens.Tokens {
					if tokens.Tokens[i].Name == idOrName {
						match = &tokens.Tokens[i]
						break
					}
				}
			}
			if match == nil {
				return fmt.Errorf("token %q not found", idOrName)
			}

			if !force {
				fmt.Fprintf(os.Stderr, "Revoke token %q (%s)? [y/N] ", match.Name, match.TokenPrefix)
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(strings.ToLower(answer)) != "y" {
					return fmt.Errorf("aborted")
				}
			}

			if err := c.RevokeToken(match.ID); err != nil {
				return fmt.Errorf("failed to revoke token: %w", err)
			}

			fmt.Printf("Token %q revoked.\n", match.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompt")
	return cmd
}
