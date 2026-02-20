/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package workspace

import (
	"fmt"
	"os"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
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
		return wegerrors.NotFound("remote clone", ".weg")
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
		output.Printf("Expanded %d files:", len(result.Expanded))
		for _, f := range result.Expanded {
			output.Printf("  + %s", f)
		}
	}

	if len(result.Conflicts) > 0 {
		output.Printf("\nConflicts (use --force to overwrite):")
		for _, f := range result.Conflicts {
			output.Printf("  ! %s", f)
		}
	}

	if len(result.Errors) > 0 {
		output.Printf("\nErrors:")
		for _, e := range result.Errors {
			output.Printf("  x %s", e)
		}
	}

	if len(result.Expanded) == 0 && len(result.Conflicts) == 0 {
		output.Print("No code fields found to expand.")
	}

	return nil
}
