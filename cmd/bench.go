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
	Short: "Manage benches and run bench commands",
	Long: `Manage Frappe benches and run bench CLI commands.

Subcommands:
  weg bench list      List all weg-managed benches
  weg bench new       Create a new bench
  weg bench current   Show current bench
  weg bench drop      Remove a bench

Pass-through (run any bench command):
  weg bench -- <command>   Run bench <command>
  weg bench -- migrate     Run bench migrate
  weg bench -- console     Run bench console

Examples:
  weg bench list
  weg bench new ~/my-bench --version 15
  weg bench -- --site mysite migrate`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no args, show help
		if len(args) == 0 {
			cmd.Help()
			return
		}

		// Pass through to bench CLI
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
	// Add subcommands from bench package
	benchCmd.AddCommand(bench.BenchCmd.Commands()...)
	rootCmd.AddCommand(benchCmd)
}
