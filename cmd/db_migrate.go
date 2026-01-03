package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var dbMigrateCmd = &cobra.Command{
	Use:   "db-migrate [site]",
	Short: "Run database migrations",
	Long: `Run database migrations for a Frappe site.

This applies schema changes and runs patches for all installed apps.
It's equivalent to 'bench frappe --site <site> migrate'.

Examples:
  weg db-migrate                    # Migrate default site
  weg db-migrate test.localhost     # Migrate specific site
  weg db-migrate --all              # Migrate all sites`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDbMigrate,
}

var (
	migrateAll bool
)

func init() {
	rootCmd.AddCommand(dbMigrateCmd)
	dbMigrateCmd.Flags().BoolVar(&migrateAll, "all", false, "Migrate all sites")
}

func runDbMigrate(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	// Determine which sites to migrate
	var sites []string
	if migrateAll {
		st, err := state.Load(absPath)
		if err == nil {
			sites = st.SiteNames()
		}
		if len(sites) == 0 {
			sitesDir := filepath.Join(benchPath, "sites")
			entries, _ := os.ReadDir(sitesDir)
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "assets" {
					sites = append(sites, e.Name())
				}
			}
		}
	} else if len(args) > 0 {
		sites = []string{args[0]}
	} else {
		st, err := state.Load(absPath)
		if err == nil {
			site := st.GetDefaultSite()
			if site != "" {
				sites = []string{site}
			}
		}
		if len(sites) == 0 {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site := strings.TrimSpace(string(data))
			if site != "" {
				sites = []string{site}
			}
		}
	}

	if len(sites) == 0 {
		return fmt.Errorf("no site specified and no default site found")
	}

	// Run migrations for each site
	for _, site := range sites {
		PrintInfo("Running migrations for %s...", site)

		cmdArgs := []string{"frappe", "--site", site, "migrate"}
		if err := RunBench(cmdArgs); err != nil {
			return fmt.Errorf("migration failed for %s: %w", site, err)
		}

		PrintInfo("Migrations complete for %s", site)
	}

	return nil
}
