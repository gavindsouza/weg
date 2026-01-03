package user

import (
	"github.com/spf13/cobra"
)

// UserCmd is the root command for user operations
var UserCmd = &cobra.Command{
	Use:   "user",
	Short: "User management",
	Long: `Create and manage Frappe users.

Examples:
  weg user list                          # List all users
  weg user create test@example.com       # Create a new user
  weg user password Administrator        # Set password
  weg user enable test@example.com       # Enable user
  weg user disable test@example.com      # Disable user
  weg user role add test@example.com "System Manager"`,
}
