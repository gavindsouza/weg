package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Runner executes commands with consistent environment handling.
// It automatically detects whether to use devbox based on the working directory.
type Runner struct {
	Dir        string
	Verbose    bool
	useDevbox  bool
	devboxPath string
}

// NewRunner creates a Runner for the given directory.
// It detects whether devbox is configured in the directory.
func NewRunner(dir string) *Runner {
	r := &Runner{
		Dir:     dir,
		Verbose: os.Getenv("WEG_VERBOSE") != "",
	}
	r.detectDevbox()
	return r
}

// NewRunnerWithOptions creates a Runner with explicit options.
func NewRunnerWithOptions(dir string, verbose bool) *Runner {
	r := &Runner{
		Dir:     dir,
		Verbose: verbose,
	}
	r.detectDevbox()
	return r
}

// detectDevbox checks if devbox.json exists in the directory.
func (r *Runner) detectDevbox() {
	devboxJSON := filepath.Join(r.Dir, "devbox.json")
	if info, err := os.Stat(devboxJSON); err == nil && !info.IsDir() {
		r.useDevbox = true
		r.devboxPath = r.Dir
	}
}

// UsesDevbox returns true if the runner will use devbox for commands.
func (r *Runner) UsesDevbox() bool {
	return r.useDevbox
}

// Run executes a command, wrapping it with devbox run if necessary.
// Returns the combined stdout/stderr and any error.
func (r *Runner) Run(name string, args ...string) error {
	cmd := r.command(name, args...)

	if r.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &RunError{
			Command: name,
			Args:    args,
			Output:  output,
			Err:     err,
		}
	}
	return nil
}

// RunWithOutput executes a command and returns its output.
func (r *Runner) RunWithOutput(name string, args ...string) ([]byte, error) {
	cmd := r.command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, &RunError{
			Command: name,
			Args:    args,
			Output:  output,
			Err:     err,
		}
	}
	return output, nil
}

// RunInteractive runs a command with stdin/stdout/stderr attached.
func (r *Runner) RunInteractive(name string, args ...string) error {
	cmd := r.command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// command creates the exec.Cmd, wrapping with devbox if needed.
func (r *Runner) command(name string, args ...string) *exec.Cmd {
	return r.commandInDir(r.Dir, name, args...)
}

// commandInDir creates the exec.Cmd for a specific directory.
func (r *Runner) commandInDir(dir, name string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd

	if r.useDevbox {
		// Wrap command with devbox run
		// Use -c to specify devbox config directory
		devboxArgs := []string{"run", "-c", r.devboxPath, "--"}
		devboxArgs = append(devboxArgs, name)
		devboxArgs = append(devboxArgs, args...)
		cmd = exec.Command("devbox", devboxArgs...)
	} else {
		cmd = exec.Command(name, args...)
	}

	cmd.Dir = dir
	return cmd
}

// RunInDir executes a command in a specific directory.
// Uses devbox from the runner's configured path if available.
func (r *Runner) RunInDir(dir, name string, args ...string) error {
	cmd := r.commandInDir(dir, name, args...)

	if r.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &RunError{
			Command: name,
			Args:    args,
			Output:  output,
			Err:     err,
		}
	}
	return nil
}

// RunError provides detailed error information for failed commands.
type RunError struct {
	Command string
	Args    []string
	Output  []byte
	Err     error
}

func (e *RunError) Error() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s failed: %v", e.Command, e.Err))
	if len(e.Output) > 0 {
		buf.WriteString("\nOutput:\n")
		buf.Write(e.Output)
	}
	return buf.String()
}

func (e *RunError) Unwrap() error {
	return e.Err
}

// RunInDevbox creates a runner that always uses devbox if available.
// Useful for explicit devbox requirement.
func RunInDevbox(dir string, name string, args ...string) error {
	r := NewRunner(dir)
	if !r.useDevbox {
		return fmt.Errorf("devbox.json not found in %s", dir)
	}
	return r.Run(name, args...)
}

// EnsureDevbox checks if devbox is configured in the directory.
// Returns an error with helpful message if not configured.
func EnsureDevbox(dir string) error {
	devboxJSON := filepath.Join(dir, "devbox.json")
	if _, err := os.Stat(devboxJSON); os.IsNotExist(err) {
		return fmt.Errorf("devbox not initialized. Run 'weg init' to set up the environment")
	}
	return nil
}
