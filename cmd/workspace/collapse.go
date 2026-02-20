/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	wegoutput "github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	collapseDryRun   bool
	collapseForce    bool
	collapseValidate bool
)

var collapseCmd = &cobra.Command{
	Use:   "collapse",
	Short: "Pack workspace changes back into JSON",
	Long: `Pack code changes from weg_workspace/ back into their source JSON files.

This updates the canonical JSON files with your edits from the workspace.

Examples:
  weg workspace collapse              # Collapse all changes
  weg workspace collapse --dry-run    # Show what would change
  weg workspace collapse --validate   # Run linters before collapse
  weg workspace collapse --force      # Overwrite even if conflicts`,
	RunE: runCollapse,
}

func init() {
	collapseCmd.Flags().BoolVar(&collapseDryRun, "dry-run", false, "Show what would change without modifying files")
	collapseCmd.Flags().BoolVar(&collapseForce, "force", false, "Overwrite files even if there are conflicts")
	collapseCmd.Flags().BoolVar(&collapseValidate, "validate", false, "Run linters before collapse")
}

func runCollapse(cmd *cobra.Command, args []string) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if we're in a weg clone
	if _, err := os.Stat(".weg"); os.IsNotExist(err) {
		return wegerrors.NotFound("remote clone", ".weg")
	}

	// Check if workspace exists
	if _, err := os.Stat(workspace.WorkspaceDir); os.IsNotExist(err) {
		return wegerrors.NotFound("workspace", "")
	}

	// Run validation if requested
	if collapseValidate {
		wegoutput.Print("Running linters...")
		if err := runLinters(cwd); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		wegoutput.Print("Validation passed")
		wegoutput.Print("")
	}

	result, err := workspace.Collapse(workspace.CollapseOptions{
		BaseDir:  cwd,
		DryRun:   collapseDryRun,
		Force:    collapseForce,
		Validate: collapseValidate,
	})
	if err != nil {
		return err
	}

	// Print results
	prefix := ""
	if collapseDryRun {
		prefix = "[dry-run] "
	}

	if len(result.Updated) > 0 {
		wegoutput.Printf("%sUpdated %d files:", prefix, len(result.Updated))
		for _, f := range result.Updated {
			wegoutput.Printf("  ~ %s", f)
		}
	}

	if len(result.Unchanged) > 0 && cmd.Flags().Changed("verbose") {
		wegoutput.Printf("\nUnchanged: %d files", len(result.Unchanged))
	}

	if len(result.Conflicts) > 0 {
		wegoutput.Printf("\nConflicts (use --force to overwrite):")
		for _, f := range result.Conflicts {
			wegoutput.Printf("  ! %s", f)
		}
		wegoutput.Print("\nBoth the source JSON and workspace file were modified.")
		wegoutput.Print("Options:")
		wegoutput.Print("  weg workspace collapse --force  # Use workspace version")
		wegoutput.Print("  weg workspace expand --force    # Use JSON version")
	}

	if len(result.Errors) > 0 {
		wegoutput.Printf("\nErrors:")
		for _, e := range result.Errors {
			wegoutput.Printf("  x %s", e)
		}
	}

	if len(result.Updated) == 0 && len(result.Conflicts) == 0 && len(result.Errors) == 0 {
		wegoutput.Print("Nothing to collapse. Workspace is in sync.")
	}

	return nil
}

// runLinters runs ruff (Python) and eslint (JavaScript) on workspace files
func runLinters(baseDir string) error {
	workspaceDir := filepath.Join(baseDir, workspace.WorkspaceDir)
	var hasErrors bool

	// Run ruff on Python files if ruff is available
	if _, err := exec.LookPath("ruff"); err == nil {
		wegoutput.Print("  Running ruff...")
		cmd := exec.Command("ruff", "check", workspaceDir)
		cmd.Dir = baseDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s", output)
			hasErrors = true
		} else {
			wegoutput.Print("  ruff: no issues")
		}
	} else {
		wegoutput.Print("  (ruff not found, skipping Python linting)")
	}

	// Run eslint on JavaScript files if eslint is available
	if _, err := exec.LookPath("eslint"); err == nil {
		wegoutput.Print("  Running eslint...")
		cmd := exec.Command("eslint", workspaceDir, "--ext", ".js")
		cmd.Dir = baseDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			// ESLint returns non-zero even for warnings
			if len(output) > 0 {
				fmt.Printf("%s", output)
				hasErrors = true
			}
		} else {
			wegoutput.Print("  eslint: no issues")
		}
	} else {
		wegoutput.Print("  (eslint not found, skipping JavaScript linting)")
	}

	if hasErrors {
		return wegerrors.Operation("lint", "errors found", nil)
	}

	return nil
}
