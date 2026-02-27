package termui

import (
	"os"

	"golang.org/x/term"
)

var isTTY = term.IsTerminal(int(os.Stdout.Fd()))

// IsTTY reports whether stdout is connected to a terminal.
func IsTTY() bool { return isTTY }

func Bold(s string) string {
	if !isTTY {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func Green(s string) string {
	if !isTTY {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func Yellow(s string) string {
	if !isTTY {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func Dim(s string) string {
	if !isTTY {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}

func Violet(s string) string {
	if !isTTY {
		return s
	}
	return "\033[38;2;139;92;246m" + s + "\033[0m"
}
