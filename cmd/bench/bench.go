package bench

import (
	"github.com/spf13/cobra"
)

var BenchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Manage Frappe benches",
	Long: `Commands for managing traditional Frappe bench environments.

While weg encourages app-centric development (your app is the project root),
these commands support traditional bench workflows for compatibility.

Examples:
  weg bench list              # List all weg-managed benches
  weg bench new <path>        # Create a new traditional bench
  weg bench current           # Show current bench
  weg bench drop <name>       # Remove a bench`,
}

func init() {
	BenchCmd.AddCommand(listCmd)
	BenchCmd.AddCommand(newCmd)
	BenchCmd.AddCommand(currentCmd)
	BenchCmd.AddCommand(dropCmd)
}
