package build

import (
	"github.com/spf13/cobra"
)

// BuildCmd is the root command for build/asset operations. Bare `weg build`
// runs the assets build directly — it's the 90% case.
var BuildCmd = &cobra.Command{
	Use:   "build [app]",
	Short: "Build assets and frontend",
	Long: `Build frontend assets for Frappe apps.

Running 'weg build' is equivalent to 'weg build assets'.

Examples:
  weg build                    # Build all app assets
  weg build myapp              # Build specific app assets
  weg build --production       # Production build with minification
  weg build watch              # Watch mode for development
  weg build clear              # Clear built assets`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAssets,
}

func init() {
	// Mirror the assets flags so bare `weg build` accepts them too.
	BuildCmd.Flags().StringVarP(&assetsSite, "site", "s", "", "Site to build for")
	BuildCmd.Flags().BoolVar(&assetsHard, "hard", false, "Clean build, remove old bundles")
	BuildCmd.Flags().BoolVar(&assetsProduction, "production", false, "Production mode with minification")
}
