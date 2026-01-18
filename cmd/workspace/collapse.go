/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package workspace

import (
	"fmt"
	"os"

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
		return fmt.Errorf("not a weg remote clone (no .weg directory)")
	}

	// Check if workspace exists
	if _, err := os.Stat(workspace.WorkspaceDir); os.IsNotExist(err) {
		return fmt.Errorf("no workspace found (run 'weg workspace expand' first)")
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
		fmt.Printf("%sUpdated %d files:\n", prefix, len(result.Updated))
		for _, f := range result.Updated {
			fmt.Printf("  ~ %s\n", f)
		}
	}

	if len(result.Unchanged) > 0 && cmd.Flags().Changed("verbose") {
		fmt.Printf("\nUnchanged: %d files\n", len(result.Unchanged))
	}

	if len(result.Conflicts) > 0 {
		fmt.Printf("\nConflicts (use --force to overwrite):\n")
		for _, f := range result.Conflicts {
			fmt.Printf("  ! %s\n", f)
		}
		fmt.Println("\nBoth the source JSON and workspace file were modified.")
		fmt.Println("Options:")
		fmt.Println("  weg workspace collapse --force  # Use workspace version")
		fmt.Println("  weg workspace expand --force    # Use JSON version")
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, e := range result.Errors {
			fmt.Printf("  ✗ %s\n", e)
		}
	}

	if len(result.Updated) == 0 && len(result.Conflicts) == 0 && len(result.Errors) == 0 {
		fmt.Println("Nothing to collapse. Workspace is in sync.")
	}

	return nil
}
