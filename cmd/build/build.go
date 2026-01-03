package build

import (
	"github.com/spf13/cobra"
)

// BuildCmd is the root command for build/asset operations
var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build assets and frontend",
	Long: `Build frontend assets for Frappe apps.

Examples:
  weg build                    # Build all app assets
  weg build myapp              # Build specific app assets
  weg build --watch            # Watch mode for development
  weg build --production       # Production build with minification`,
}
