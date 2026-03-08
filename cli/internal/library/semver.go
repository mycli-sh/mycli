package library

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSemver parses a "vX.Y.Z" tag into its components.
func ParseSemver(tag string) (major, minor, patch int, err error) {
	if !strings.HasPrefix(tag, "v") {
		return 0, 0, 0, fmt.Errorf("tag %q must start with 'v'", tag)
	}
	parts := strings.Split(tag[1:], ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("tag %q must have exactly three parts (vX.Y.Z)", tag)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version in %q: %w", tag, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version in %q: %w", tag, err)
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version in %q: %w", tag, err)
	}
	return major, minor, patch, nil
}

// FormatTag returns a "vX.Y.Z" string.
func FormatTag(major, minor, patch int) string {
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch)
}

// BumpSemver bumps the given part ("patch", "minor", or "major") of a semver tag.
func BumpSemver(tag, part string) (string, error) {
	major, minor, patch, err := ParseSemver(tag)
	if err != nil {
		return "", err
	}
	switch part {
	case "patch":
		patch++
	case "minor":
		minor++
		patch = 0
	case "major":
		major++
		minor = 0
		patch = 0
	default:
		return "", fmt.Errorf("unknown bump part %q (use patch, minor, or major)", part)
	}
	return FormatTag(major, minor, patch), nil
}
