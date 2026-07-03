package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate [site]",
	Short: "Run database migrations",
	Long: `Run database migrations for a Frappe site.

This applies schema changes and runs patches for all installed apps.

Examples:
  weg db migrate                    # Migrate default site
  weg db migrate test.localhost     # Migrate specific site
  weg db migrate --all              # Migrate all sites`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMigrate,
}

var migrateAll bool

func init() {
	DbCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().BoolVar(&migrateAll, "all", false, "Migrate all sites")
}

// NewMigrateAlias returns a hidden command that behaves like 'weg db migrate',
// for registration as a top-level 'weg migrate' alias.
func NewMigrateAlias() *cobra.Command {
	alias := &cobra.Command{
		Use:    "migrate [site]",
		Short:  "Alias for 'weg db migrate'",
		Hidden: true,
		Long: `Alias for 'weg db migrate'. Runs database migrations for a Frappe site.

To convert project structure between app-centric and bench-centric modes
(the old behavior of 'weg migrate'), see 'weg convert'.

Examples:
  weg migrate                    # Migrate default site
  weg migrate test.localhost     # Migrate specific site
  weg migrate --all              # Migrate all sites`,
		Args: cobra.MaximumNArgs(1),
		RunE: runMigrate,
	}
	alias.Flags().BoolVar(&migrateAll, "all", false, "Migrate all sites")
	return alias
}

func runMigrate(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return wegerrors.NotInProject(absPath)
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
		return wegerrors.Usage("no site specified and no default site found")
	}

	// Run migrations for each site
	for _, site := range sites {
		output.Infof("Running migrations for %s...", site)

		cmdArgs := []string{"frappe", "--site", site, "migrate"}
		if err := runBench(benchPath, cmdArgs); err != nil {
			return wegerrors.Operation("migration", site, err)
		}

		output.Printf("Migrations complete for %s", site)
	}

	return nil
}

// runBench runs a bench command - imported from bench_runner
func runBench(benchPath string, args []string) error {
	// This will be implemented by importing from the main cmd package
	// For now, use exec directly
	return runBenchHelper(benchPath, args)
}
