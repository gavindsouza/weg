package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var pyCmd = &cobra.Command{
	Use:   "py [code or script.py]",
	Short: "Run Python with frappe context pre-loaded",
	Long: `Execute Python code with frappe already connected to the site.

This eliminates the boilerplate of setting PYTHONPATH, importing frappe,
and connecting to the site. Just write your code and go.

The following are pre-imported and available:
  - frappe (connected to site)
  - frappe.db (database access)

Examples:
  # Inline code
  weg py "print(frappe.get_all('User', pluck='name'))"
  weg py "frappe.db.sql('SELECT name FROM tabUser')"

  # Run a script file
  weg py script.py

  # Pipe code from stdin
  echo "print(frappe.db.count('User'))" | weg py -

  # Use a different site
  weg py --site dev.localhost "print(frappe.local.site)"`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runPy,
	SilenceUsage: true,
}

var pySite string

func init() {
	rootCmd.AddCommand(pyCmd)
	pyCmd.Flags().StringVar(&pySite, "site", "", "Site to connect to (default: current site)")
}

func runPy(cmd *cobra.Command, args []string) error {
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
	site := pySite
	if site == "" {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
	}
	if site == "" {
		return fmt.Errorf("no site specified and no default site found. Use --site flag")
	}

	// Determine what to execute
	var userCode string
	if len(args) == 0 || args[0] == "-" {
		// Read from stdin
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		userCode = string(data)
	} else if strings.HasSuffix(args[0], ".py") {
		// It's a script file
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read script: %w", err)
		}
		userCode = string(data)
	} else {
		// Inline code
		userCode = args[0]
	}

	// Build the wrapper code
	wrapperCode := fmt.Sprintf(`
import os
import sys

# Change to sites directory
os.chdir(%q)

# Import and connect frappe
import frappe
frappe.connect(site=%q)

try:
    # User code
%s
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site, indentCode(userCode))

	// Build PYTHONPATH
	appsDir := filepath.Join(benchPath, "apps")
	pythonPath := []string{}

	// Add each app to PYTHONPATH
	entries, _ := os.ReadDir(appsDir)
	for _, entry := range entries {
		if entry.IsDir() {
			pythonPath = append(pythonPath, filepath.Join(appsDir, entry.Name()))
		}
	}

	// Get the Python interpreter
	pythonBin := filepath.Join(benchPath, "env", "bin", "python")
	if _, err := os.Stat(pythonBin); os.IsNotExist(err) {
		pythonBin = "python3" // Fallback
	}

	// Execute
	pyCmd := exec.Command(pythonBin, "-c", wrapperCode)
	pyCmd.Dir = benchPath
	pyCmd.Stdout = os.Stdout
	pyCmd.Stderr = os.Stderr
	pyCmd.Stdin = os.Stdin

	// Set environment
	env := os.Environ()
	if len(pythonPath) > 0 {
		existingPath := os.Getenv("PYTHONPATH")
		newPath := strings.Join(pythonPath, ":")
		if existingPath != "" {
			newPath = newPath + ":" + existingPath
		}
		env = append(env, fmt.Sprintf("PYTHONPATH=%s", newPath))
	}
	pyCmd.Env = env

	return pyCmd.Run()
}

// indentCode adds proper indentation for the user code inside the try block
func indentCode(code string) string {
	lines := strings.Split(code, "\n")
	indented := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			indented[i] = "    " + line
		} else {
			indented[i] = ""
		}
	}
	return strings.Join(indented, "\n")
}
