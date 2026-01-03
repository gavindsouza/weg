package fixtures

import (
	"github.com/spf13/cobra"
)

// FixturesCmd is the root command for fixture operations
var FixturesCmd = &cobra.Command{
	Use:   "fixtures",
	Short: "Manage data fixtures",
	Long: `Export and import data fixtures for app development.

Fixtures are JSON files loaded during app installation to provide
default data like roles, workflows, print formats, etc.

Examples:
  weg fixtures export myapp              # Export all fixtures
  weg fixtures export myapp --doctype Role
  weg fixtures import myapp              # Import fixtures
  weg fixtures list myapp                # List fixture files`,
}
