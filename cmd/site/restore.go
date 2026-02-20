package site

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore <backup-file> [site]",
	Short: "Restore a Frappe site from backup",
	Long: `Restore a site's database (and optionally files) from a backup.

The backup file should be a SQL dump (plain or gzipped).
If a site is not specified, uses the default site.

Optionally restore private files using --with-files and specifying
the files backup archive.

Examples:
  weg site restore backup.sql.gz                  # Restore to default site
  weg site restore backup.sql.gz test.localhost   # Restore to specific site
  weg site restore db.sql.gz --files files.tar.gz # Also restore files
  weg site restore backup.sql.gz -f               # Skip confirmation`,
	Args:              cobra.RangeArgs(1, 2),
	RunE:              runRestore,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(1),
}

var (
	restoreFiles        string
	restoreClearCache   bool
	mariadbRootPassword string
	restoreForce        bool
)

func init() {
	SiteCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().StringVar(&restoreFiles, "files", "", "Path to files backup archive")
	restoreCmd.Flags().BoolVar(&restoreClearCache, "clear-cache", true, "Clear cache after restore")
	restoreCmd.Flags().StringVar(&mariadbRootPassword, "mariadb-root-password", "", "MariaDB root password (for recreating database)")
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "Skip confirmation")
}

func runRestore(cmd *cobra.Command, args []string) error {
	backupFile := args[0]

	// Verify backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

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

	// Determine site
	var site string
	if len(args) > 1 {
		site = args[1]
	} else {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
	}

	if site == "" {
		return fmt.Errorf("no site specified and no default site found")
	}

	// Load site config
	siteConfig, err := loadSiteConfig(benchPath, site)
	if err != nil {
		return fmt.Errorf("failed to load site config: %w", err)
	}

	// Check for devbox
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	useDevbox := false
	if _, err := os.Stat(devboxJSON); err == nil {
		useDevbox = true
	}

	if !restoreForce {
		output.Printf("This will overwrite the database for %s", site)
		output.Printf("Database: %s", siteConfig.DBName)
		output.Print("")
		if !prompt.ConfirmDanger("Continue with restore?") {
			return fmt.Errorf("restore cancelled")
		}
	}

	output.Infof("Restoring %s from %s...", site, backupFile)

	// Restore database
	if err := restoreDatabase(benchPath, site, siteConfig, backupFile, useDevbox); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}
	output.Print("  Database restored")

	// Restore files if specified
	if restoreFiles != "" {
		if _, err := os.Stat(restoreFiles); os.IsNotExist(err) {
			output.Warningf("files backup not found: %s", restoreFiles)
		} else {
			if err := restoreFilesBackup(benchPath, site, restoreFiles); err != nil {
				output.Warningf("failed to restore files: %v", err)
			} else {
				output.Print("  Files restored")
			}
		}
	}

	// Clear cache
	if restoreClearCache {
		output.Print("  Clearing cache...")
		executor := api.NewExecutor(benchPath, site, "Administrator")
		script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.clear_cache()
    print(json.dumps({"success": True}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

		if _, err := executor.ExecuteRaw(script); err != nil {
			output.Warningf("failed to clear cache: %v", err)
		}
	}

	output.Successf("Restore completed for %s", site)
	output.Print("")
	output.Info("You may want to run 'weg site password' to set the admin password")

	return nil
}

func restoreDatabase(benchPath, site string, cfg *siteConfig, backupFile string, useDevbox bool) error {
	// Detect if file is gzipped
	isGzipped := strings.HasSuffix(backupFile, ".gz")

	// Open the backup file
	file, err := os.Open(backupFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var reader io.Reader = file
	if isGzipped {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to open gzip: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var cmd *exec.Cmd

	switch cfg.DBType {
	case "postgres":
		args := []string{
			"-h", cfg.DBHost,
			"-p", fmt.Sprintf("%d", cfg.DBPort),
			"-U", cfg.DBName,
			"-d", cfg.DBName,
		}
		if useDevbox {
			cmd = exec.Command("devbox", append([]string{"run", "-c", benchPath, "--", "psql"}, args...)...)
		} else {
			cmd = exec.Command("psql", args...)
		}

	case "sqlite":
		// For SQLite, just overwrite the database file
		dbPath := filepath.Join(benchPath, "sites", site, "private", "frappe.db")

		// Ensure private directory exists
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(dbPath)
		if err != nil {
			return err
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, reader)
		return err

	default: // mariadb/mysql
		args := []string{
			"-h", cfg.DBHost,
			"-P", fmt.Sprintf("%d", cfg.DBPort),
			"-u", cfg.DBName,
		}
		if cfg.DBPassword != "" {
			args = append(args, fmt.Sprintf("-p%s", cfg.DBPassword))
		}
		args = append(args, cfg.DBName)

		if useDevbox {
			cmd = exec.Command("devbox", append([]string{"run", "-c", benchPath, "--", "mysql"}, args...)...)
		} else {
			cmd = exec.Command("mysql", args...)
		}
	}

	cmd.Dir = benchPath
	cmd.Stdin = reader
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func restoreFilesBackup(benchPath, site, filesBackup string) error {
	siteDir := filepath.Join(benchPath, "sites", site)

	// Extract tar.gz
	cmd := exec.Command("tar", "-xzf", filesBackup, "-C", siteDir)
	cmd.Dir = benchPath

	return cmd.Run()
}

// loadSiteConfigForRestore loads site config, handling missing files
func loadSiteConfigForRestore(benchPath, site string) (*siteConfig, error) {
	configPath := filepath.Join(benchPath, "sites", site, "site_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Return defaults if config doesn't exist
		return &siteConfig{
			DBName: strings.ReplaceAll(site, ".", "_"),
			DBType: "mariadb",
			DBHost: "127.0.0.1",
			DBPort: 3306,
		}, nil
	}

	var cfg siteConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.DBType == "" {
		cfg.DBType = "mariadb"
	}
	if cfg.DBHost == "" {
		cfg.DBHost = "127.0.0.1"
	}
	if cfg.DBPort == 0 {
		if cfg.DBType == "postgres" {
			cfg.DBPort = 5432
		} else {
			cfg.DBPort = 3306
		}
	}

	return &cfg, nil
}
