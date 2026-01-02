package cache

import (
	"github.com/spf13/cobra"
)

// CacheCmd is the root command for cache management
var CacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage Frappe cache",
	Long: `Commands for managing Frappe cache.

The cache includes:
  - Redis cache (session data, document cache)
  - Local Python bytecode (.pyc files)

Examples:
  weg cache clear              # Clear all cache
  weg cache clear --site test  # Clear cache for specific site`,
}

func init() {
	CacheCmd.AddCommand(clearCmd)
}
