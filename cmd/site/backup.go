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
	"time"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup [site]",
	Short: "Backup a Frappe site",
	Long: `Create a backup of a Frappe site's database and optionally files.

By default, creates a gzipped SQL dump of the database.
Use --with-files to also backup private files.

Backups are stored in .weg/backups/ by default, with timestamp naming:
  {site}_{datetime}.sql.gz
  {site}_{datetime}_files.tar.gz (if --with-files)

Examples:
  weg site backup                      # Backup default site
  weg site backup test.localhost       # Backup specific site
  weg site backup --with-files         # Include private files
  weg site backup --output /path/      # Custom backup location
  weg site backup --all                # Backup all sites`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runBackup,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

var (
	backupWithFiles bool
	backupOutput    string
	backupAll       bool
)

func init() {
	SiteCmd.AddCommand(backupCmd)
	backupCmd.Flags().BoolVar(&backupWithFiles, "with-files", false, "Include private files in backup")
	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "Output directory for backup files")
	backupCmd.Flags().BoolVar(&backupAll, "all", false, "Backup all sites")
}

func runBackup(cmd *cobra.Command, args []string) error {
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

	// Load state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	// Determine which sites to backup
	var sites []string
	if backupAll {
		sites = st.SiteNames()
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
		site := st.GetDefaultSite()
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
		if site == "" {
			return fmt.Errorf("no site specified and no default site found")
		}
		sites = []string{site}
	}

	// Determine backup directory
	backupDir := backupOutput
	if backupDir == "" {
		backupDir = filepath.Join(benchPath, "backups")
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Check for devbox
	devboxJSON := filepath.Join(benchPath, "devbox.json")
	useDevbox := false
	if _, err := os.Stat(devboxJSON); err == nil {
		useDevbox = true
	}

	// Backup each site
	for _, site := range sites {
		output.Infof("Backing up %s...", site)

		siteConfig, err := loadSiteConfig(benchPath, site)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load site config for %s: %v\n", site, err)
			continue
		}

		timestamp := time.Now().Format("20060102_150405")
		baseFilename := fmt.Sprintf("%s_%s", strings.ReplaceAll(site, ".", "_"), timestamp)

		// Backup database
		dbFile := filepath.Join(backupDir, baseFilename+".sql.gz")
		if err := backupDatabase(benchPath, site, siteConfig, dbFile, useDevbox); err != nil {
			return fmt.Errorf("failed to backup database for %s: %w", site, err)
		}
		fmt.Printf("  Database: %s\n", dbFile)

		// Backup files if requested
		if backupWithFiles {
			filesBackup := filepath.Join(backupDir, baseFilename+"_files.tar.gz")
			if err := backupFiles(benchPath, site, filesBackup); err != nil {
				fmt.Printf("Warning: failed to backup files for %s: %v\n", site, err)
			} else {
				fmt.Printf("  Files: %s\n", filesBackup)
			}
		}
	}

	if len(sites) == 1 {
		fmt.Printf("Backup completed for %s\n", sites[0])
	} else {
		fmt.Printf("Backup completed for %d sites\n", len(sites))
	}

	return nil
}

type siteConfig struct {
	DBName     string `json:"db_name"`
	DBType     string `json:"db_type"`
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBPassword string `json:"db_password"`
}

func loadSiteConfig(benchPath, site string) (*siteConfig, error) {
	configPath := filepath.Join(benchPath, "sites", site, "site_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
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

func backupDatabase(benchPath, site string, cfg *siteConfig, outputPath string, useDevbox bool) error {
	// Create the output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	var cmd *exec.Cmd

	switch cfg.DBType {
	case "postgres":
		args := []string{
			"-h", cfg.DBHost,
			"-p", fmt.Sprintf("%d", cfg.DBPort),
			"-U", cfg.DBName,
			cfg.DBName,
		}
		if useDevbox {
			cmd = exec.Command("devbox", append([]string{"run", "-c", benchPath, "--", "pg_dump"}, args...)...)
		} else {
			cmd = exec.Command("pg_dump", args...)
		}

	case "sqlite":
		// SQLite backup is simpler - just copy the file
		dbPath := filepath.Join(benchPath, "sites", site, "private", "frappe.db")
		srcFile, err := os.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open sqlite database: %w", err)
		}
		defer srcFile.Close()

		_, err = io.Copy(gzWriter, srcFile)
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
			cmd = exec.Command("devbox", append([]string{"run", "-c", benchPath, "--", "mysqldump"}, args...)...)
		} else {
			cmd = exec.Command("mysqldump", args...)
		}
	}

	cmd.Dir = benchPath
	cmd.Stdout = gzWriter
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func backupFiles(benchPath, site, outputPath string) error {
	privateDir := filepath.Join(benchPath, "sites", site, "private")
	if _, err := os.Stat(privateDir); os.IsNotExist(err) {
		return fmt.Errorf("private directory not found")
	}

	// Create tar.gz of private files
	cmd := exec.Command("tar", "-czf", outputPath, "-C", filepath.Join(benchPath, "sites", site), "private")
	cmd.Dir = benchPath

	return cmd.Run()
}
