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
	expandType  string
	expandClean bool
	expandForce bool
)

var expandCmd = &cobra.Command{
	Use:   "expand",
	Short: "Extract code from JSON files into workspace",
	Long: `Extract code fields from JSON entity files into the weg_workspace/ directory.

This creates properly-typed source files that you can edit with full IDE support:
  - Syntax highlighting
  - Linting and error checking
  - Autocomplete (with proper stubs)

Examples:
  weg workspace expand                    # Expand all entities
  weg workspace expand --type server_script  # Only server scripts
  weg workspace expand --force            # Overwrite even if conflicts`,
	RunE: runExpand,
}

func init() {
	expandCmd.Flags().StringVar(&expandType, "type", "", "Filter by entity type (server_script, client_script, report, print_format)")
	expandCmd.Flags().BoolVar(&expandClean, "clean", false, "Remove stale workspace files first")
	expandCmd.Flags().BoolVar(&expandForce, "force", false, "Overwrite files even if there are conflicts")
}

func runExpand(cmd *cobra.Command, args []string) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if we're in a weg clone
	if _, err := os.Stat(".weg"); os.IsNotExist(err) {
		return fmt.Errorf("not a weg remote clone (no .weg directory)")
	}

	result, err := workspace.Expand(workspace.ExpandOptions{
		BaseDir:    cwd,
		EntityType: expandType,
		Clean:      expandClean,
		Force:      expandForce,
	})
	if err != nil {
		return err
	}

	// Print results
	if len(result.Expanded) > 0 {
		fmt.Printf("Expanded %d files:\n", len(result.Expanded))
		for _, f := range result.Expanded {
			fmt.Printf("  + %s\n", f)
		}
	}

	if len(result.Conflicts) > 0 {
		fmt.Printf("\nConflicts (use --force to overwrite):\n")
		for _, f := range result.Conflicts {
			fmt.Printf("  ! %s\n", f)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, e := range result.Errors {
			fmt.Printf("  x %s\n", e)
		}
	}

	if len(result.Expanded) == 0 && len(result.Conflicts) == 0 {
		fmt.Println("No code fields found to expand.")
	}

	return nil
}
