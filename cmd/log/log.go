package log

import (
	"github.com/spf13/cobra"
)

// LogCmd is the root command for log operations
var LogCmd = &cobra.Command{
	Use:   "log",
	Short: "View and manage logs",
	Long: `View, tail, and manage Frappe logs.

Examples:
  weg log tail                  # Tail all logs
  weg log tail web              # Tail web server logs
  weg log tail worker           # Tail background worker logs
  weg log show                  # Show recent log entries
  weg log clear                 # Clear log files`,
}
