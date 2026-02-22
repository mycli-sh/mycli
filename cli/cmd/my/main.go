package main

import (
	"errors"
	"fmt"
	"os"

	"mycli.sh/cli/internal/commands"
	"mycli.sh/cli/internal/engine"
)

func main() {
	if err := commands.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		var exitErr *engine.ExitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
