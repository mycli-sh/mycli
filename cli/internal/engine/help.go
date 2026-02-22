package engine

import (
	"fmt"
	"strings"

	"mycli.sh/pkg/spec"
)

// FormatHelp generates a help string for a command spec, similar to Cobra's help output.
func FormatHelp(s *spec.CommandSpec, usagePrefix string) string {
	var b strings.Builder

	// Description
	if s.Metadata.Description != "" {
		b.WriteString(s.Metadata.Description)
		b.WriteString("\n\n")
	}

	// Aliases
	if len(s.Metadata.Aliases) > 0 {
		b.WriteString("Aliases:\n")
		b.WriteString("  ")
		b.WriteString(strings.Join(s.Metadata.Aliases, ", "))
		b.WriteString("\n\n")
	}

	// Dependencies
	if len(s.Dependencies) > 0 {
		b.WriteString("Dependencies:\n")
		b.WriteString("  ")
		b.WriteString(strings.Join(s.Dependencies, ", "))
		b.WriteString("\n\n")
	}

	// Usage
	b.WriteString("Usage:\n")
	b.WriteString("  ")
	b.WriteString(usagePrefix)
	if len(s.Args.Flags) > 0 {
		b.WriteString(" [flags]")
	}
	for _, p := range s.Args.Positional {
		if p.IsRequired() {
			fmt.Fprintf(&b, " <%s>", p.Name)
		} else {
			fmt.Fprintf(&b, " [%s]", p.Name)
		}
	}
	b.WriteString("\n")

	// Positional args
	if len(s.Args.Positional) > 0 {
		b.WriteString("\nArguments:\n")

		// Compute column width
		nameWidth := 0
		for _, p := range s.Args.Positional {
			if len(p.Name) > nameWidth {
				nameWidth = len(p.Name)
			}
		}

		for _, p := range s.Args.Positional {
			desc := p.Description
			extras := []string{}
			if !p.IsRequired() {
				extras = append(extras, "optional")
			}
			if p.Default != "" {
				extras = append(extras, fmt.Sprintf("default: %s", p.Default))
			}
			if len(extras) > 0 {
				if desc != "" {
					desc += " "
				}
				desc += "(" + strings.Join(extras, ", ") + ")"
			}
			fmt.Fprintf(&b, "  %-*s   %s\n", nameWidth, p.Name, desc)
		}
	}

	// Flags
	if len(s.Args.Flags) > 0 {
		b.WriteString("\nFlags:\n")

		type flagLine struct {
			left  string
			right string
		}

		var lines []flagLine
		maxLeft := 0

		for _, f := range s.Args.Flags {
			var left string
			shortPart := "    "
			if f.Short != "" {
				shortPart = fmt.Sprintf("-%s, ", f.Short)
			}
			typePart := ""
			if f.GetType() != "bool" {
				typePart = " " + f.GetType()
			}
			left = fmt.Sprintf("%s--%s%s", shortPart, f.Name, typePart)
			if len(left) > maxLeft {
				maxLeft = len(left)
			}

			desc := f.Description
			extras := []string{}
			if f.Default != nil {
				extras = append(extras, fmt.Sprintf("default: %v", f.Default))
			}
			if f.IsRequired() {
				extras = append(extras, "required")
			}
			if len(extras) > 0 {
				if desc != "" {
					desc += " "
				}
				desc += "(" + strings.Join(extras, ", ") + ")"
			}
			lines = append(lines, flagLine{left: left, right: desc})
		}

		// Add built-in help flag
		helpLeft := "-h, --help"
		if len(helpLeft) > maxLeft {
			maxLeft = len(helpLeft)
		}
		lines = append(lines, flagLine{left: helpLeft, right: "Show help for this command"})

		for _, l := range lines {
			fmt.Fprintf(&b, "  %-*s   %s\n", maxLeft, l.left, l.right)
		}
	}

	return b.String()
}
