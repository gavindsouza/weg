package doc

import (
	"github.com/spf13/cobra"
)

// DocCmd is the root command for document operations
var DocCmd = &cobra.Command{
	Use:   "doc",
	Short: "Document operations",
	Long: `Create, read, update, and delete Frappe documents.

Examples:
  weg doc get User Administrator           # Get a document
  weg doc list User --limit 10             # List documents
  weg doc delete User test@test.com        # Delete document
  weg doc rename User old-name new-name    # Rename document
  weg doc export User Administrator        # Export to JSON
  weg doc import user.json                 # Import from JSON
  weg doc field get User Administrator email  # Get field value`,
}

func init() {
	// Subcommands are added in their respective files
}
