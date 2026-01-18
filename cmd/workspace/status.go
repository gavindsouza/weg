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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status between workspace and JSON",
	Long: `Show the sync status of all files in the workspace.

Status indicators:
  synced   - No changes (workspace matches JSON)
  modified - Workspace file was modified
  conflict - Both workspace and JSON were modified
  stale    - Source JSON was deleted`,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
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
		fmt.Println("No workspace found. Run 'weg workspace expand' to create one.")
		return nil
	}

	statuses, err := workspace.Status(cwd)
	if err != nil {
		return err
	}

	if len(statuses) == 0 {
		fmt.Println("Workspace is empty. Run 'weg workspace expand' to extract code files.")
		return nil
	}

	// Group by status
	var modified, conflicts, stale, synced []string

	for path, status := range statuses {
		switch status {
		case workspace.StatusModified:
			modified = append(modified, path)
		case workspace.StatusConflict:
			conflicts = append(conflicts, path)
		case workspace.StatusStale:
			stale = append(stale, path)
		case workspace.StatusSynced:
			synced = append(synced, path)
		}
	}

	// Print results
	if len(conflicts) > 0 {
		fmt.Println("Conflicts (both source and workspace modified):")
		for _, f := range conflicts {
			fmt.Printf("  ! %s\n", f)
		}
		fmt.Println()
	}

	if len(modified) > 0 {
		fmt.Println("Modified (ready to collapse):")
		for _, f := range modified {
			fmt.Printf("  ~ %s\n", f)
		}
		fmt.Println()
	}

	if len(stale) > 0 {
		fmt.Println("Stale (source JSON deleted):")
		for _, f := range stale {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println()
	}

	// Summary
	total := len(statuses)
	if len(synced) == total {
		fmt.Println("Workspace is in sync with JSON files.")
	} else {
		fmt.Printf("Summary: %d synced, %d modified, %d conflicts, %d stale\n",
			len(synced), len(modified), len(conflicts), len(stale))

		if len(modified) > 0 {
			fmt.Println("\nRun 'weg workspace collapse' to update JSON files.")
		}
	}

	return nil
}
