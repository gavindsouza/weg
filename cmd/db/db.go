package db

import (
	"github.com/spf13/cobra"
)

// DbCmd is the root command for database operations
var DbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations",
	Long: `Database management commands.

Examples:
  weg db migrate                    # Run migrations
  weg db console                    # Open database shell
  weg db backup                     # Backup database
  weg db restore backup.sql.gz      # Restore from backup
  weg db trim --days 30             # Trim log tables`,
}
