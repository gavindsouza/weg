package cmd

import (
	"github.com/gavindsouza/weg/cmd/db"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

// 'weg migrate' is a hidden alias for 'weg db migrate' - to Frappe users,
// "migrate" means database migrations. The old structure-conversion behavior
// of 'weg migrate' lives on as 'weg convert'.
func init() {
	migrateAlias := db.NewMigrateAlias()
	dbMigrateRunE := migrateAlias.RunE
	migrateAlias.RunE = func(cmd *cobra.Command, args []string) error {
		// Backward compatibility: 'weg migrate app|bench' used to convert
		// the project structure. Forward those invocations to 'weg convert'.
		if len(args) == 1 && (args[0] == "app" || args[0] == "bench") {
			output.Warningf("'weg migrate %s' is now 'weg convert %s'. Forwarding...", args[0], args[0])
			return runConvert(cmd, args)
		}
		return dbMigrateRunE(cmd, args)
	}
	rootCmd.AddCommand(migrateAlias)
}
