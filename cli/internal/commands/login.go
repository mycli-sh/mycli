package commands

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
	"mycli.sh/cli/internal/termui"
)

var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in to mycli",
		RunE: func(cmd *cobra.Command, args []string) error {
			if auth.IsLoggedIn() {
				fmt.Println("You are already logged in.")
				return nil
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			apiURL := resolveAPIURL(cfg)
			c := client.New(apiURL)
			defer c.Close()

			// Prompt for email
			fmt.Print("Enter your email: ")
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("failed to read email")
			}
			email := strings.TrimSpace(scanner.Text())
			if email == "" {
				return fmt.Errorf("email is required")
			}
			if !strings.Contains(email, "@") || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
				return fmt.Errorf("invalid email format")
			}

			// Start device flow with email
			resp, err := c.StartDeviceFlow(email)
			if err != nil {
				return fmt.Errorf("failed to start login: %w", err)
			}

			if !resp.EmailSent {
				return fmt.Errorf("failed to send verification email")
			}

			fmt.Println()

			// Set up timing
			interval := time.Duration(resp.Interval) * time.Second
			if interval == 0 {
				interval = 5 * time.Second
			}
			expiresIn := time.Duration(resp.ExpiresIn) * time.Second
			if expiresIn == 0 {
				expiresIn = 15 * time.Minute
			}

			tokenResp, err := runOTPVerification(c, resp.DeviceCode, email, interval, expiresIn)
			if err != nil {
				return err
			}
			return handleLoginSuccess(c, tokenResp)
		},
	}
}

func handleLoginSuccess(c *client.Client, tokenResp *auth.TokenResponse) error {
	tokens := &auth.Tokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}
	if err := auth.SaveTokens(tokens); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	fmt.Println()
	fmt.Printf("  %s\n", termui.Green("Logged in successfully!"))
	fmt.Println()

	// Prompt for username if needed (new user)
	if tokenResp.NeedsUsername {
		promptForUsername(c)
	}

	// Auto-sync commands
	fmt.Println("Syncing commands...")
	fetched, syncErr := cache.Sync(c, false)
	if syncErr != nil {
		fmt.Printf("Warning: sync failed: %v\n", syncErr)
	} else if fetched == 0 {
		fmt.Println("Already up to date.")
	} else {
		fmt.Printf("Synced %d command(s).\n", fetched)
	}

	return nil
}

func promptForUsername(c *client.Client) {
	fmt.Printf("  %s\n", termui.Bold("Choose a username"))
	fmt.Printf("  %s\n", termui.Dim("Your username will be used in library slugs (e.g. username/my-library)."))
	fmt.Printf("  %s\n", termui.Dim("Must be 3-39 characters: lowercase letters, numbers, and hyphens."))
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("  Username: ")
		if !scanner.Scan() {
			fmt.Println()
			fmt.Printf("  %s\n", termui.Yellow("Username is required."))
			continue
		}

		username := strings.TrimSpace(scanner.Text())
		if username == "" {
			fmt.Printf("  %s\n", termui.Yellow("Username is required."))
			continue
		}

		// Client-side validation
		if len(username) < 3 {
			fmt.Printf("  %s\n", termui.Yellow("Username must be at least 3 characters."))
			continue
		}
		if len(username) > 39 {
			fmt.Printf("  %s\n", termui.Yellow("Username must be at most 39 characters."))
			continue
		}
		if !usernameRegex.MatchString(username) {
			fmt.Printf("  %s\n", termui.Yellow("Must start with a letter and contain only lowercase letters, numbers, and hyphens."))
			continue
		}

		// Check availability
		avail, err := c.CheckUsernameAvailable(username)
		if err != nil {
			fmt.Printf("  %s\n", termui.Yellow("Could not check availability. Try again."))
			continue
		}
		if !avail.Available {
			fmt.Printf("  %s\n", termui.Yellow(avail.Reason))
			continue
		}

		// Set username
		if err := c.SetUsername(username); err != nil {
			fmt.Printf("  %s\n", termui.Yellow("Could not set username. Try again."))
			continue
		}

		fmt.Printf("  %s %s\n\n", termui.Green("Username set:"), termui.Bold(username))
		return
	}
}
