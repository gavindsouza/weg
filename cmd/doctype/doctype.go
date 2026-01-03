package doctype

import (
	"github.com/spf13/cobra"
)

// DoctypeCmd is the root command for DocType operations
var DoctypeCmd = &cobra.Command{
	Use:   "doctype",
	Short: "DocType operations",
	Long: `Inspect and manage DocType definitions.

Examples:
  weg doctype list                       # List custom doctypes
  weg doctype list myapp                 # List doctypes in app
  weg doctype show User                  # Show DocType structure
  weg doctype reload myapp module MyDoc  # Reload from disk`,
}
