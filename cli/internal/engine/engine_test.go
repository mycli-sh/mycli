package engine

import (
	"strings"
	"testing"

	"mycli.sh/pkg/spec"
)

func TestParseArgs_Positional(t *testing.T) {
	s := &spec.CommandSpec{
		Args: spec.Args{
			Positional: []spec.PositionalArg{
				{Name: "service"},
				{Name: "env", Default: "staging"},
			},
		},
	}

	data, err := parseArgs(s, []string{"myapp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Args["service"] != "myapp" {
		t.Errorf("expected service=myapp, got %v", data.Args["service"])
	}
	if data.Args["env"] != "staging" {
		t.Errorf("expected env=staging, got %v", data.Args["env"])
	}
}

func TestParseArgs_MissingRequired(t *testing.T) {
	s := &spec.CommandSpec{
		Args: spec.Args{
			Positional: []spec.PositionalArg{
				{Name: "service"},
			},
		},
	}

	_, err := parseArgs(s, []string{})
	if err == nil {
		t.Fatal("expected error for missing required arg")
	}
}

func TestParseArgs_Flags(t *testing.T) {
	s := &spec.CommandSpec{
		Args: spec.Args{
			Flags: []spec.FlagArg{
				{Name: "env", Short: "e", Type: "string", Default: "staging"},
				{Name: "verbose", Short: "v", Type: "bool"},
			},
		},
	}

	data, err := parseArgs(s, []string{"--env", "production", "-v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Args["env"] != "production" {
		t.Errorf("expected env=production, got %v", data.Args["env"])
	}
	if data.Args["verbose"] != "true" {
		t.Errorf("expected verbose=true, got %v", data.Args["verbose"])
	}
}

func TestRenderTemplate(t *testing.T) {
	data := templateData{
		Args: map[string]any{"service": "myapp", "env": "staging"},
		Cwd:  "/tmp",
		Home: "/home/user",
	}

	result, err := renderTemplate("echo deploying {{.args.service}} to {{.args.env}}", data, "{{", "}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "echo deploying myapp to staging" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestRenderTemplate_CustomDelimiters(t *testing.T) {
	data := templateData{
		Args: map[string]any{"ns": "default"},
		Cwd:  "/tmp",
		Home: "/home/user",
	}

	// With custom delimiters, {{range .items}} should pass through as literal text
	text := `kubectl get pods -n <% .args.ns %> -o go-template='{{range .items}}{{.metadata.name}}{{end}}'`
	result, err := renderTemplate(text, data, "<%", "%>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `kubectl get pods -n default -o go-template='{{range .items}}{{.metadata.name}}{{end}}'`
	if result != expected {
		t.Errorf("unexpected result:\n  got:  %s\n  want: %s", result, expected)
	}
}

func TestRenderTemplate_ErrorHintForConflictingTemplates(t *testing.T) {
	data := templateData{
		Args: map[string]any{},
		Cwd:  "/tmp",
		Home: "/home/user",
	}

	// This contains {{range .items}} which conflicts with default delimiters
	text := `kubectl get pods -o go-template='{{range .items}}{{.metadata.name}}{{end}}'`
	_, err := renderTemplate(text, data, "{{", "}}")
	if err == nil {
		t.Fatal("expected error for conflicting template syntax")
	}
	if !strings.Contains(err.Error(), "hint:") {
		t.Errorf("expected error hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "templateDelimiters") {
		t.Errorf("expected error to mention templateDelimiters, got: %v", err)
	}
}

func TestGetTemplateDelimiters_Default(t *testing.T) {
	// nil Defaults
	var d *spec.Defaults
	left, right := d.GetTemplateDelimiters()
	if left != "{{" || right != "}}" {
		t.Errorf("expected default delimiters, got %q %q", left, right)
	}

	// Empty Defaults
	d = &spec.Defaults{}
	left, right = d.GetTemplateDelimiters()
	if left != "{{" || right != "}}" {
		t.Errorf("expected default delimiters, got %q %q", left, right)
	}
}

func TestGetTemplateDelimiters_Custom(t *testing.T) {
	d := &spec.Defaults{
		TemplateDelimiters: []string{"<%", "%>"},
	}
	left, right := d.GetTemplateDelimiters()
	if left != "<%" || right != "%>" {
		t.Errorf("expected custom delimiters, got %q %q", left, right)
	}
}

func TestCheckDependencies_AllPresent(t *testing.T) {
	// "sh" and "echo" should exist on any system
	err := checkDependencies([]string{"sh", "echo"})
	if err != nil {
		t.Errorf("expected no error for present binaries, got: %v", err)
	}
}

func TestCheckDependencies_Missing(t *testing.T) {
	err := checkDependencies([]string{"sh", "nonexistent-binary-xyz"})
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "nonexistent-binary-xyz") {
		t.Errorf("expected error to mention missing binary, got: %v", err)
	}
	if !strings.Contains(err.Error(), "missing required dependencies") {
		t.Errorf("expected 'missing required dependencies' message, got: %v", err)
	}
}

func TestCheckDependencies_Empty(t *testing.T) {
	err := checkDependencies(nil)
	if err != nil {
		t.Errorf("expected no error for empty deps, got: %v", err)
	}
}

func TestExecuteStep_ZeroTimeoutNoDeadline(t *testing.T) {
	s := &spec.CommandSpec{
		Steps: []spec.Step{
			{Name: "wait", Run: "sleep 0.2", Timeout: "0s"},
		},
	}
	exitCode, err := executeStep(s, &s.Steps[0], templateData{
		Args: map[string]any{},
		Env:  map[string]string{},
	}, ExecOpts{})
	if err != nil {
		t.Fatalf("expected no error with 0s timeout, got: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestCheckAllowedExecutables(t *testing.T) {
	// Allowed
	err := checkAllowedExecutables("echo hello\nkubectl apply -f x.yaml", []string{"echo", "kubectl"})
	if err != nil {
		t.Errorf("expected allowed, got error: %v", err)
	}

	// Not allowed
	err = checkAllowedExecutables("rm -rf /", []string{"echo", "kubectl"})
	if err == nil {
		t.Error("expected error for disallowed executable")
	}
}

func TestCheckAllowedExecutables_EmptyLines(t *testing.T) {
	err := checkAllowedExecutables("\n\n# comment\necho hello\n", []string{"echo"})
	if err != nil {
		t.Errorf("expected allowed, got error: %v", err)
	}
}
