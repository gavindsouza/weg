/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package workspace

import (
	"github.com/spf13/cobra"
)

// WorkspaceCmd is the parent command for workspace operations
var WorkspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage expanded code workspace",
	Long: `Manage the weg_workspace/ directory for editing code with proper syntax highlighting.

The workspace extracts code from JSON files into properly-typed source files:
  - Server Scripts (.py)
  - Client Scripts (.js)
  - Reports (.py, .js, .sql)
  - Print Formats (.html, .css)

Commands:
  expand   - Extract code from JSON files into workspace
  collapse - Pack workspace changes back into JSON
  status   - Show sync status between workspace and JSON
  init     - Set up workspace with pre-commit hooks

Example workflow:
  weg workspace expand      # Extract scripts to weg_workspace/
  # ... edit files in your IDE with syntax highlighting ...
  weg workspace collapse    # Pack changes back to JSON
  git commit -m "Update scripts"`,
	Aliases: []string{"ws"},
}

func init() {
	WorkspaceCmd.AddCommand(expandCmd)
	WorkspaceCmd.AddCommand(collapseCmd)
	WorkspaceCmd.AddCommand(statusCmd)
	WorkspaceCmd.AddCommand(initCmd)
}
