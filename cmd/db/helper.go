package db

import (
	"os"

	"github.com/gavindsouza/weg/tools"
)

// runBenchHelper runs a bench command in the given bench path
func runBenchHelper(benchPath string, args []string) error {
	cmd := tools.FrappeCmd(benchPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
