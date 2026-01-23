package db

import (
	"os"
	"os/exec"
	"path/filepath"
)

// runBenchHelper runs a bench command in the given bench path
func runBenchHelper(benchPath string, args []string) error {
	sitesDir := filepath.Join(benchPath, "sites")
	venvPython := filepath.Join(benchPath, "env", "bin", "python")

	// Build the command: python -m frappe.utils.bench_helper frappe <args>
	cmdArgs := append([]string{"-m", "frappe.utils.bench_helper"}, args...)

	// Check if devbox is available
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	var cmd *exec.Cmd
	if _, err := os.Stat(devboxJSON); err == nil {
		// Use devbox
		devboxArgs := append([]string{"run", "-c", benchPath, "--", venvPython}, cmdArgs...)
		cmd = exec.Command("devbox", devboxArgs...)
	} else {
		cmd = exec.Command(venvPython, cmdArgs...)
	}

	cmd.Dir = sitesDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
