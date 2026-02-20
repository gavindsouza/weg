package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] -- <command> [args...]",
	Short: "Run a command in the bench context",
	Long: `Run a command in the bench environment context.

Examples:
  weg exec -- bench migrate
  weg exec --site mysite.localhost -- python -c "import frappe"`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE:               runExec,
	SilenceUsage:       true,
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

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return errors.NotInProject(absPath)
	}

	// Determine site
	site := execSite
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
	}

	command := args[0]

	// For bench commands, use RunBench
	if command == "bench" && len(args) > 1 {
		return RunBench(args[1:])
	}

	// For other commands, run in the bench context
	cmdPath, err := exec.LookPath(command)
	if err != nil {
		return errors.NotFound("command", command)
	}

	env := os.Environ()
	env = append(env, fmt.Sprintf("FRAPPE_BENCH_ROOT=%s", benchPath))
	if site != "" {
		env = append(env, fmt.Sprintf("FRAPPE_SITE=%s", site))
	}

	if err := syscall.Exec(cmdPath, args, env); err != nil {
		execCmd := exec.Command(cmdPath, args[1:]...)
		execCmd.Dir = benchPath
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin
		execCmd.Env = env
		return execCmd.Run()
	}

	return nil
}
