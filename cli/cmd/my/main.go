package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/commands"
	"mycli.sh/cli/internal/engine"
)

// backgroundRefreshExitWait bounds the wait at exit for an in-flight background
// refresh to persist its rotation. Incurred at most once per refresh interval.
const backgroundRefreshExitWait = 10 * time.Second

func main() {
	err := commands.NewRootCmd().Execute()

	// Let any in-flight background refresh finish saving its rotated token
	// before we exit, so a local-only command doesn't drop it mid-flight.
	auth.WaitForBackgroundRefresh(backgroundRefreshExitWait)

	if err != nil {
		// A definitively-expired session gets a clean, actionable message
		// instead of a raw "UNAUTHORIZED: invalid token".
		if errors.Is(err, client.ErrSessionExpired) {
			fmt.Fprintln(os.Stderr, client.ErrSessionExpired.Error())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		var exitErr *engine.ExitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
