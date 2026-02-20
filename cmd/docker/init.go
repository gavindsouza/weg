package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/container"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate docker-compose.yml",
	Long: `Generate a docker-compose.yml file for the current project.

Creates a Docker Compose configuration with:
- Web server (Gunicorn)
- Background workers
- Scheduler
- Socket.io server
- Database (MariaDB/PostgreSQL)
- Redis instances

Examples:
  weg docker init                    # Generate for development
  weg docker init --mode prod        # Generate for production
  weg docker init --no-db            # Without database service`,
	RunE: runInit,
}

var (
	initMode    string
	initNoDb    bool
	initNoRedis bool
	initWebPort int
	initDbPort  int
)

func init() {
	initCmd.Flags().StringVar(&initMode, "mode", "dev", "Mode: dev or prod")
	initCmd.Flags().BoolVar(&initNoDb, "no-db", false, "Don't include database service")
	initCmd.Flags().BoolVar(&initNoRedis, "no-redis", false, "Don't include Redis services")
	initCmd.Flags().IntVar(&initWebPort, "web-port", 8000, "Web server port")
	initCmd.Flags().IntVar(&initDbPort, "db-port", 3306, "Database port")
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	result, err := config.DetectProjectContext(cwd)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	var appName string
	var database string

	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
		appName = filepath.Base(cwd)
		// Try to get database from weg.toml
		if cfg, err := config.ParseWegToml(filepath.Join(cwd, "weg.toml")); err == nil {
			database = cfg.Frappe.Database
		}
	case config.ContextWegApp:
		benchPath = filepath.Join(cwd, ".weg")
		appName = filepath.Base(cwd)
		// Try to get database from .weg/weg.toml
		if cfg, err := config.ParseWegToml(filepath.Join(benchPath, "weg.toml")); err == nil {
			database = cfg.Frappe.Database
		}
	default:
		return wegerrors.Usage("not in a weg-managed project")
	}

	if database == "" {
		database = "mariadb"
	}

	opts := container.ComposeOptions{
		BenchPath:    benchPath,
		ProjectName:  appName,
		AppName:      appName,
		Mode:         initMode,
		WebPort:      initWebPort,
		DBPort:       initDbPort,
		Database:     database,
		IncludeDB:    !initNoDb,
		IncludeRedis: !initNoRedis,
		Volumes:      true,
	}

	compose := container.GenerateDockerCompose(opts)

	// Write to project root (not .weg)
	composePath := filepath.Join(cwd, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(compose), 0644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	output.Print("Generated docker-compose.yml")
	output.Printf("  Mode: %s", initMode)
	output.Printf("  Web port: %d", initWebPort)
	if !initNoDb {
		output.Printf("  Database: %s (port %d)", database, initDbPort)
	}
	output.Print("")
	output.Print("Next steps:")
	output.Print("  weg docker up       # Start containers")
	output.Print("  weg docker logs     # View logs")

	return nil
}
