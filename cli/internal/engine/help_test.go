package engine

import (
	"strings"
	"testing"

	"mycli.sh/pkg/spec"
)

func boolPtr(b bool) *bool { return &b }

func TestFormatHelp_FullSpec(t *testing.T) {
	s := &spec.CommandSpec{
		Metadata: spec.Metadata{
			Name:        "port-forward",
			Slug:        "port-forward",
			Description: "Port-forward to a Kubernetes pod",
		},
		Args: spec.Args{
			Positional: []spec.PositionalArg{
				{Name: "query", Description: "Grep filter for resources", Required: boolPtr(false)},
			},
			Flags: []spec.FlagArg{
				{Name: "namespace", Short: "n", Description: "Target namespace", Type: "string"},
				{Name: "all-namespaces", Short: "A", Description: "Query all namespaces", Type: "bool"},
			},
		},
	}

	out := FormatHelp(s, "my kubernetes port-forward")

	// Check description
	if !strings.Contains(out, "Port-forward to a Kubernetes pod") {
		t.Error("expected description in output")
	}

	// Check usage line
	if !strings.Contains(out, "my kubernetes port-forward [flags] [query]") {
		t.Errorf("expected usage line, got:\n%s", out)
	}

	// Check positional arg
	if !strings.Contains(out, "query") && !strings.Contains(out, "Grep filter") {
		t.Error("expected positional arg in output")
	}
	if !strings.Contains(out, "optional") {
		t.Error("expected (optional) marker for non-required arg")
	}

	// Check flags
	if !strings.Contains(out, "-n, --namespace string") {
		t.Errorf("expected namespace flag, got:\n%s", out)
	}
	if !strings.Contains(out, "-A, --all-namespaces") {
		t.Errorf("expected all-namespaces flag, got:\n%s", out)
	}

	// Check help flag is appended
	if !strings.Contains(out, "-h, --help") {
		t.Error("expected built-in help flag in output")
	}
}

func TestFormatHelp_MinimalSpec(t *testing.T) {
	s := &spec.CommandSpec{
		Metadata: spec.Metadata{
			Name: "simple",
			Slug: "simple",
		},
		Args: spec.Args{},
	}

	out := FormatHelp(s, "my lib simple")

	// No description section
	if strings.HasPrefix(out, "\n") {
		t.Error("should not start with blank line for empty description")
	}

	// Should have usage
	if !strings.Contains(out, "Usage:\n  my lib simple\n") {
		t.Errorf("expected simple usage line, got:\n%s", out)
	}

	// No Arguments section
	if strings.Contains(out, "Arguments:") {
		t.Error("should not have Arguments section with no positional args")
	}

	// No Flags section
	if strings.Contains(out, "Flags:") {
		t.Error("should not have Flags section with no flags")
	}
}

func TestFormatHelp_RequiredArgs(t *testing.T) {
	s := &spec.CommandSpec{
		Metadata: spec.Metadata{
			Name: "deploy",
			Slug: "deploy",
		},
		Args: spec.Args{
			Positional: []spec.PositionalArg{
				{Name: "target", Description: "Deploy target", Required: boolPtr(true)},
				{Name: "tag", Description: "Image tag", Required: boolPtr(false), Default: "latest"},
			},
		},
	}

	out := FormatHelp(s, "my ops deploy")

	// Required arg uses <> in usage
	if !strings.Contains(out, "<target>") {
		t.Errorf("expected <target> for required arg, got:\n%s", out)
	}

	// Optional arg uses [] in usage
	if !strings.Contains(out, "[tag]") {
		t.Errorf("expected [tag] for optional arg, got:\n%s", out)
	}

	// Default shown
	if !strings.Contains(out, "default: latest") {
		t.Errorf("expected default value, got:\n%s", out)
	}
}

func TestFormatHelp_FlagDefaults(t *testing.T) {
	s := &spec.CommandSpec{
		Metadata: spec.Metadata{
			Name: "cmd",
			Slug: "cmd",
		},
		Args: spec.Args{
			Flags: []spec.FlagArg{
				{Name: "count", Description: "Number of items", Type: "string", Default: "10"},
				{Name: "verbose", Short: "v", Description: "Verbose output", Type: "bool"},
				{Name: "env", Description: "Environment", Type: "string", Required: boolPtr(true)},
			},
		},
	}

	out := FormatHelp(s, "my lib cmd")

	// Check default value shown
	if !strings.Contains(out, "default: 10") {
		t.Errorf("expected default value for count, got:\n%s", out)
	}

	// Bool flag should not show type
	if strings.Contains(out, "--verbose bool") {
		t.Error("bool flags should not show type label")
	}
	if !strings.Contains(out, "--verbose") {
		t.Error("expected verbose flag")
	}

	// Required flag
	if !strings.Contains(out, "required") {
		t.Errorf("expected required marker for env flag, got:\n%s", out)
	}
}

func TestFormatHelp_WithDependencies(t *testing.T) {
	s := &spec.CommandSpec{
		Metadata: spec.Metadata{
			Name: "port-forward",
			Slug: "port-forward",
		},
		Dependencies: []string{"kubectl", "fzf", "jq"},
		Args:         spec.Args{},
	}

	out := FormatHelp(s, "my kubernetes port-forward")

	if !strings.Contains(out, "Dependencies:") {
		t.Error("expected Dependencies section in output")
	}
	if !strings.Contains(out, "kubectl, fzf, jq") {
		t.Errorf("expected dependency list, got:\n%s", out)
	}
}

func TestFormatHelp_WithoutDependencies(t *testing.T) {
	s := &spec.CommandSpec{
		Metadata: spec.Metadata{
			Name: "simple",
			Slug: "simple",
		},
		Args: spec.Args{},
	}

	out := FormatHelp(s, "my lib simple")

	if strings.Contains(out, "Dependencies:") {
		t.Error("should not have Dependencies section when no dependencies")
	}
}

func TestFormatHelp_NoShortFlag(t *testing.T) {
	s := &spec.CommandSpec{
		Metadata: spec.Metadata{
			Name: "cmd",
			Slug: "cmd",
		},
		Args: spec.Args{
			Flags: []spec.FlagArg{
				{Name: "output", Description: "Output format"},
			},
		},
	}

	out := FormatHelp(s, "my lib cmd")

	// Flag without short should have padding
	if !strings.Contains(out, "    --output") {
		t.Errorf("expected padded long flag without short, got:\n%s", out)
	}
}
