package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

var benchCmd = &cobra.Command{
	Use:     "bench [args]",
	Short:   "Run bench commands via weg",
	Aliases: []string{"b"},
	Run: func(cmd *cobra.Command, args []string) {
		benchCli, err := exec.LookPath("bench")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		execArgs := append([]string{"bench"}, args...)
		syscall.Exec(benchCli, execArgs, os.Environ())
	},
}

func init() {
	rootCmd.AddCommand(benchCmd)
}
