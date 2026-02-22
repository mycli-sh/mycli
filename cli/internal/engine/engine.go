package engine

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"mycli.sh/pkg/spec"
)

type ExecOpts struct {
	Yes     bool   // bypass confirmation
	WorkDir string // override working directory
}

type ExecResult struct {
	ExitCode   int
	DurationMs int64
}

// ExitCodeError wraps an error with a specific exit code for propagation to the shell.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string { return e.Err.Error() }
func (e *ExitCodeError) Unwrap() error { return e.Err }

func Execute(s *spec.CommandSpec, args []string, opts ExecOpts) (*ExecResult, error) {
	start := time.Now()

	// Parse arguments
	data, err := parseArgs(s, args)
	if err != nil {
		return nil, fmt.Errorf("argument error: %w", err)
	}

	// Check dependencies before proceeding
	if err := checkDependencies(s.Dependencies); err != nil {
		return nil, err
	}

	// Policy: confirmation gate
	if s.Policy != nil && s.Policy.RequireConfirmation && !opts.Yes {
		fmt.Fprintf(os.Stderr, "Command %q requires confirmation.\n", s.Metadata.Name)
		fmt.Fprintf(os.Stderr, "Steps to execute:\n")
		for i, step := range s.Steps {
			fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, step.Name)
		}
		fmt.Fprintf(os.Stderr, "Proceed? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			return &ExecResult{ExitCode: 1, DurationMs: time.Since(start).Milliseconds()}, fmt.Errorf("aborted by user")
		}
	}

	// Execute steps
	var lastExitCode int
	var failedExitCode int
	for _, step := range s.Steps {
		exitCode, err := executeStep(s, &step, data, opts)
		lastExitCode = exitCode
		if err != nil {
			if step.ContinueOnError {
				fmt.Fprintf(os.Stderr, "Step %q failed (continuing): %v\n", step.Name, err)
				if failedExitCode == 0 {
					failedExitCode = exitCode
				}
				continue
			}
			return &ExecResult{
				ExitCode:   exitCode,
				DurationMs: time.Since(start).Milliseconds(),
			}, fmt.Errorf("step %q failed: %w", step.Name, err)
		}
	}

	finalCode := lastExitCode
	if failedExitCode != 0 {
		finalCode = failedExitCode
	}
	var finalErr error
	if failedExitCode != 0 {
		finalErr = fmt.Errorf("one or more steps failed")
	}
	return &ExecResult{
		ExitCode:   finalCode,
		DurationMs: time.Since(start).Milliseconds(),
	}, finalErr
}

func executeStep(s *spec.CommandSpec, step *spec.Step, data templateData, opts ExecOpts) (int, error) {
	// Determine shell
	shell := "/bin/sh"
	if step.Shell != "" {
		shell = step.Shell
	} else if s.Defaults != nil && s.Defaults.Shell != "" {
		shell = s.Defaults.Shell
	}

	// Validate shell against known set
	if !isAllowedShell(shell) {
		return 1, fmt.Errorf("unsupported shell %q: must be one of /bin/sh, /bin/bash, /bin/zsh, /usr/bin/env", shell)
	}

	// Determine timeout
	timeout := 5 * time.Minute // default
	timeoutStr := ""
	if step.Timeout != "" {
		timeoutStr = step.Timeout
	} else if s.Defaults != nil && s.Defaults.Timeout != "" {
		timeoutStr = s.Defaults.Timeout
	}
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return 1, fmt.Errorf("invalid timeout %q: %w", timeoutStr, err)
		}
		timeout = d
	}

	// Render the run script
	script, err := renderTemplate(step.Run, data)
	if err != nil {
		return 1, fmt.Errorf("template render error: %w", err)
	}

	// Policy check: allowed executables
	if s.Policy != nil && len(s.Policy.AllowedExecutables) > 0 {
		if err := checkAllowedExecutables(script, s.Policy.AllowedExecutables); err != nil {
			return 1, err
		}
	}

	// Build env
	env := os.Environ()
	if s.Defaults != nil {
		for k, v := range s.Defaults.Env {
			rendered, err := renderTemplate(v, data)
			if err != nil {
				return 1, fmt.Errorf("env template render error for %s: %w", k, err)
			}
			env = append(env, k+"="+rendered)
		}
	}
	for k, v := range step.Env {
		rendered, err := renderTemplate(v, data)
		if err != nil {
			return 1, fmt.Errorf("env template render error for %s: %w", k, err)
		}
		env = append(env, k+"="+rendered)
	}

	// Create command
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout == 0 {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-c", script)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	// Catch SIGINT so the Go process survives while the child runs.
	// The child still receives SIGINT from the terminal (signal.Notify
	// only affects the Go runtime, not child processes).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	err = cmd.Run()

	signal.Stop(sigCh)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), err
		}
		if ctx.Err() == context.DeadlineExceeded {
			return 1, fmt.Errorf("step timed out after %s", timeout)
		}
		return 1, err
	}

	return 0, nil
}

type templateData struct {
	Args map[string]any
	Env  map[string]string
	Cwd  string
	Home string
}

func parseArgs(s *spec.CommandSpec, rawArgs []string) (templateData, error) {
	data := templateData{
		Args: make(map[string]any),
		Env:  make(map[string]string),
	}

	data.Cwd, _ = os.Getwd()
	data.Home, _ = os.UserHomeDir()

	// Copy environment
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			data.Env[parts[0]] = parts[1]
		}
	}

	// Parse flags first (extract from rawArgs)
	remaining := make([]string, 0, len(rawArgs))
	flagValues := make(map[string]string)

	i := 0
	for i < len(rawArgs) {
		arg := rawArgs[i]
		if strings.HasPrefix(arg, "--") {
			name := strings.TrimPrefix(arg, "--")
			if eqIdx := strings.Index(name, "="); eqIdx >= 0 {
				flagValues[name[:eqIdx]] = name[eqIdx+1:]
			} else if i+1 < len(rawArgs) {
				// Check if this is a bool flag
				isBool := false
				for _, f := range s.Args.Flags {
					if f.Name == name && f.GetType() == "bool" {
						isBool = true
						break
					}
				}
				if isBool {
					flagValues[name] = "true"
				} else {
					i++
					flagValues[name] = rawArgs[i]
				}
			} else {
				flagValues[name] = "true"
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) == 2 {
			short := string(arg[1])
			// Find the flag by short name
			var flagName string
			isBool := false
			for _, f := range s.Args.Flags {
				if f.Short == short {
					flagName = f.Name
					isBool = f.GetType() == "bool"
					break
				}
			}
			if flagName != "" {
				if isBool {
					flagValues[flagName] = "true"
				} else if i+1 < len(rawArgs) {
					i++
					flagValues[flagName] = rawArgs[i]
				}
			} else {
				return data, fmt.Errorf("unknown flag: -%s", short)
			}
		} else {
			remaining = append(remaining, arg)
		}
		i++
	}

	// Process flag definitions
	for _, flag := range s.Args.Flags {
		if v, ok := flagValues[flag.Name]; ok {
			data.Args[flag.Name] = v
		} else if flag.Default != nil {
			data.Args[flag.Name] = fmt.Sprintf("%v", flag.Default)
		} else if flag.IsRequired() {
			return data, fmt.Errorf("missing required flag: --%s", flag.Name)
		} else {
			data.Args[flag.Name] = ""
		}
	}

	// Process positional args
	for idx, pos := range s.Args.Positional {
		if idx < len(remaining) {
			data.Args[pos.Name] = remaining[idx]
		} else if pos.Default != "" {
			data.Args[pos.Name] = pos.Default
		} else if pos.IsRequired() {
			return data, fmt.Errorf("missing required argument: %s", pos.Name)
		} else {
			data.Args[pos.Name] = ""
		}
	}

	return data, nil
}

func renderTemplate(text string, data templateData) (string, error) {
	// Replace {{.args.X}} style with {{.Args.X}} for Go templates
	// We use a custom delim-free approach: just use Go templates directly
	tmpl, err := template.New("cmd").Option("missingkey=error").Parse(text)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf strings.Builder
	// Create context with lowercase field aliases
	ctx := map[string]any{
		"args": data.Args,
		"env":  data.Env,
		"cwd":  data.Cwd,
		"home": data.Home,
	}
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("template execute error: %w", err)
	}
	return buf.String(), nil
}

func checkAllowedExecutables(script string, allowed []string) error {
	// Reject shell metacharacters that can be used to bypass the allowlist
	shellMetachars := []string{"|", ";", "&&", "||", "$(", "`", ">", "<"}
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, meta := range shellMetachars {
			if strings.Contains(line, meta) {
				return fmt.Errorf("shell metacharacter %q not allowed when executable allowlist is active", meta)
			}
		}
	}

	allowedSet := make(map[string]bool)
	for _, a := range allowed {
		allowedSet[a] = true
		allowedSet[filepath.Base(a)] = true
	}

	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		exe := parts[0]
		if !allowedSet[exe] && !allowedSet[filepath.Base(exe)] {
			return fmt.Errorf("executable %q not in allowed list: %v", exe, allowed)
		}
	}
	return nil
}

func checkDependencies(deps []string) error {
	var missing []string
	for _, dep := range deps {
		if _, err := exec.LookPath(dep); err != nil {
			missing = append(missing, dep)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required dependencies: %s", strings.Join(missing, ", "))
	}
	return nil
}

var allowedShells = map[string]bool{
	"/bin/sh":      true,
	"/bin/bash":    true,
	"/bin/zsh":     true,
	"/usr/bin/env": true,
}

func isAllowedShell(shell string) bool {
	// Allow "/usr/bin/env bash", "/usr/bin/env sh", etc.
	base := strings.Fields(shell)[0]
	return allowedShells[base]
}
