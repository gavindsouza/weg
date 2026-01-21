package bench

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <path>",
	Short: "Create a new traditional bench",
	Long: `Create a new Frappe bench in the traditional layout.

This creates a bench with the standard apps/ and sites/ structure,
managed by weg.

Examples:
  weg bench new ~/frappe-bench
  weg bench new ./my-bench --version 15
  weg bench new /path/to/bench --version 15 --database postgres`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

var (
	newVersion  string
	newDatabase string
)

func init() {
	newCmd.Flags().StringVar(&newVersion, "version", "", "Frappe version (14, 15, 16)")
	newCmd.Flags().StringVar(&newDatabase, "database", "", "Database (mariadb, postgres, sqlite)")
}

func runNew(cmd *cobra.Command, args []string) error {
	benchPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	benchName := filepath.Base(benchPath)

	// Check if path exists
	if _, err := os.Stat(benchPath); err == nil {
		// Check if it's empty
		entries, err := os.ReadDir(benchPath)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("directory %s is not empty", benchPath)
		}
	}

	// Get version
	version := newVersion
	if version == "" {
		version, err = promptForVersion()
		if err != nil {
			return err
		}
	}

	// Validate version
	if version != "14" && version != "15" && version != "16" {
		return fmt.Errorf("invalid version: %s (must be 14, 15, or 16)", version)
	}

	// Get database
	database := newDatabase
	if database == "" {
		database, err = promptForDatabase(version)
		if err != nil {
			return err
		}
	}

	// Validate database
	if !tools.IsDatabaseSupported(version, database) {
		return fmt.Errorf("database %s is not supported for Frappe %s", database, version)
	}

	output.Infof("Creating bench at %s...\n", benchPath)
	fmt.Printf("  Frappe version: %s\n", version)
	fmt.Printf("  Database: %s\n", database)
	fmt.Println()

	// Create directory structure
	if err := os.MkdirAll(benchPath, 0755); err != nil {
		return fmt.Errorf("failed to create bench directory: %w", err)
	}

	dirs := []string{
		"apps",
		"sites",
		"logs",
		"config",
		".weg",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(benchPath, dir), 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}

	// Create weg.toml
	siteName := fmt.Sprintf("%s.localhost", strings.ReplaceAll(benchName, "_", "-"))
	wegToml := fmt.Sprintf(`# Weg configuration for bench: %s

[bench]
name = "%s"

[frappe]
version = "%s"
database = "%s"

[apps]
# Add apps here, e.g.:
# erpnext = { url = "https://github.com/frappe/erpnext", branch = "version-%s" }

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-%s"

[[sites]]
name = "%s"
default = true
`, benchName, benchName, version, database, version, version, siteName)

	wegTomlPath := filepath.Join(benchPath, "weg.toml")
	if err := os.WriteFile(wegTomlPath, []byte(wegToml), 0644); err != nil {
		return fmt.Errorf("failed to write weg.toml: %w", err)
	}

	// Create common_site_config.json
	commonConfig := `{
  "frappe_user": "frappe",
  "webserver_port": 8000,
  "socketio_port": 9000,
  "file_watcher_port": 6787
}
`
	commonConfigPath := filepath.Join(benchPath, "sites", "common_site_config.json")
	if err := os.WriteFile(commonConfigPath, []byte(commonConfig), 0644); err != nil {
		return fmt.Errorf("failed to write common_site_config.json: %w", err)
	}

	// Create .gitignore
	gitignore := `# Weg
.weg/state.json
logs/
*.pyc
__pycache__/
*.egg-info/
.eggs/
dist/
build/
env/
venv/
.venv/
node_modules/
*.log
`
	gitignorePath := filepath.Join(benchPath, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignore), 0644); err != nil {
		fmt.Printf("Warning: failed to write .gitignore: %v\n", err)
	}

	fmt.Println("Bench created successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", benchPath)
	fmt.Println("  weg sync        # Install Frappe and create site")
	fmt.Println("  weg start       # Start development server")

	return nil
}

func promptForVersion() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Frappe version (14/15/16) [15]: ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		return "15", nil
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "15", nil
	}
	return answer, nil
}

func promptForDatabase(version string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	defaultDB := "mariadb"
	options := "mariadb/postgres"
	if version == "16" {
		options = "mariadb/postgres/sqlite"
	}

	fmt.Printf("Database (%s) [%s]: ", options, defaultDB)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return defaultDB, nil
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return defaultDB, nil
	}
	return answer, nil
}

// Verify config package is imported
var _ = config.ContextFresh
