package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/gavindsouza/weg/cmd/bench"
	"github.com/spf13/cobra"
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Run bench commands",
	Long: `Run bench CLI commands in the current project context.

Examples:
  weg bench migrate           # Run migrations
  weg bench console           # Open Python console
  weg bench list-apps         # List installed apps
  weg bench frappe --help     # Show all frappe commands`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			return
		}

		// Flag parsing is disabled so args are forwarded verbatim, but a
		// bare 'weg bench --help' should show weg's own help rather than
		// forwarding to the bench CLI.
		if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
			cmd.Help()
			return
		}

		// Skip "--" separator if present
		if args[0] == "--" {
			args = args[1:]
		}
		if len(args) == 0 {
			cmd.Help()
			return
		}

		// Try to run via devbox in weg-managed project
		if err := RunBench(args); err == nil {
			return
		}

		// Fallback: pass through to system bench CLI
		benchCli, err := exec.LookPath("bench")
		if err != nil {
			fmt.Fprintln(os.Stderr, "bench CLI not found:", err)
			os.Exit(1)
		}
		execArgs := append([]string{"bench"}, args...)
		syscall.Exec(benchCli, execArgs, os.Environ())
	},
	DisableFlagParsing: true,
}

func init() {
	benchCmd.AddCommand(bench.BenchCmd.Commands()...)
	rootCmd.AddCommand(benchCmd)
}
