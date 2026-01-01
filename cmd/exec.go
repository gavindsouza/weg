package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] -- <command> [args...]",
	Short: "Run a command in the bench context",
	Long: `Run a command in the bench environment context.

This ensures the command runs with the correct Python environment,
site configuration, and bench variables set.

Common shortcuts are available:
  weg exec migrate        → bench --site <default> migrate
  weg exec console        → bench --site <default> console
  weg exec mariadb        → bench --site <default> mariadb
  weg exec backup         → bench --site <default> backup

Examples:
  weg exec -- bench migrate
  weg exec --site mysite.localhost -- python -c "import frappe"
  weg exec migrate
  weg exec console`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE:               runExec,
}

var execSite string

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVar(&execSite, "site", "", "Site to use (default: current site)")
}

func runExec(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	// Determine site
	site := execSite
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			// Try to read from currentsite.txt
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			if data, err := os.ReadFile(currentSitePath); err == nil {
				site = string(data)
			}
		}
	}

	// Handle shortcut commands
	command := args[0]
	cmdArgs := args[1:]

	shortcutCommands := map[string][]string{
		"migrate":  {"bench", "--site", site, "migrate"},
		"console":  {"bench", "--site", site, "console"},
		"mariadb":  {"bench", "--site", site, "mariadb"},
		"backup":   {"bench", "--site", site, "backup"},
		"restore":  {"bench", "--site", site, "restore"},
		"set-config": {"bench", "--site", site, "set-config"},
		"clear-cache": {"bench", "--site", site, "clear-cache"},
		"scheduler": {"bench", "--site", site, "scheduler"},
	}

	var execArgs []string
	if shortcut, ok := shortcutCommands[command]; ok {
		execArgs = append(shortcut, cmdArgs...)
		command = shortcut[0]
	} else if command == "bench" {
		// Direct bench command
		execArgs = args
	} else {
		// Other command - just run it
		execArgs = args
	}

	// Find the command
	cmdPath, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("command not found: %s", command)
	}

	// Set up environment
	env := os.Environ()
	env = append(env, fmt.Sprintf("FRAPPE_BENCH_ROOT=%s", benchPath))
	if site != "" {
		env = append(env, fmt.Sprintf("FRAPPE_SITE=%s", site))
	}

	// Execute using syscall.Exec to replace the current process
	// This ensures proper signal handling and exit codes
	if err := syscall.Exec(cmdPath, execArgs, env); err != nil {
		// Fallback to regular exec if syscall fails
		execCmd := exec.Command(cmdPath, execArgs[1:]...)
		execCmd.Dir = benchPath
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin
		execCmd.Env = env
		return execCmd.Run()
	}

	return nil
}
