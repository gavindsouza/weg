package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all weg-managed benches",
	Long: `List all benches managed by weg.

This scans common locations for weg.toml files and .weg directories.

Examples:
  weg bench list
  weg bench list --all`,
	RunE: runList,
}

var listAll bool

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "Include hidden .weg directories")
}

type benchInfo struct {
	Name     string
	Path     string
	Version  string
	Database string
	IsHidden bool
}

func runList(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Common locations to search
	searchPaths := []string{
		homeDir,
		filepath.Join(homeDir, "frappe"),
		filepath.Join(homeDir, "projects"),
		filepath.Join(homeDir, "code"),
		filepath.Join(homeDir, "dev"),
		filepath.Join(homeDir, "Desktop"),
		filepath.Join(homeDir, "Desktop", "projects"),
	}

	// Also check current directory and parent
	cwd, _ := os.Getwd()
	searchPaths = append(searchPaths, cwd, filepath.Dir(cwd))

	// Deduplicate
	seen := make(map[string]bool)
	var uniquePaths []string
	for _, p := range searchPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if !seen[absPath] {
			seen[absPath] = true
			uniquePaths = append(uniquePaths, absPath)
		}
	}

	var benches []benchInfo

	for _, searchPath := range uniquePaths {
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		// Check if this directory itself is a bench
		if info := checkBench(searchPath); info != nil {
			benches = append(benches, *info)
		}

		// Check immediate subdirectories (don't recurse too deep)
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			subPath := filepath.Join(searchPath, entry.Name())

			// Check for weg.toml
			if info := checkBench(subPath); info != nil {
				benches = append(benches, *info)
			}

			// Check for .weg directory (hidden bench)
			if listAll {
				wegPath := filepath.Join(subPath, ".weg")
				if info := checkBench(wegPath); info != nil {
					info.IsHidden = true
					benches = append(benches, *info)
				}
			}
		}
	}

	// Deduplicate benches
	seenBenches := make(map[string]bool)
	var uniqueBenches []benchInfo
	for _, b := range benches {
		if !seenBenches[b.Path] {
			seenBenches[b.Path] = true
			uniqueBenches = append(uniqueBenches, b)
		}
	}

	if len(uniqueBenches) == 0 {
		fmt.Println("No weg-managed benches found.")
		fmt.Println("\nCreate one with:")
		fmt.Println("  weg create <name>     # Create traditional bench")
		fmt.Println("  weg init              # Initialize in current directory")
		return nil
	}

	// Print table
	fmt.Println("NAME                 PATH                                      VERSION  DATABASE")
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")

	for _, b := range uniqueBenches {
		name := b.Name
		if b.IsHidden {
			name = fmt.Sprintf("%s (.weg)", name)
		}

		// Truncate path if too long
		path := b.Path
		if len(path) > 40 {
			path = "..." + path[len(path)-37:]
		}

		fmt.Printf("%-20s %-41s %-8s %s\n", name, path, b.Version, b.Database)
	}

	return nil
}

func checkBench(path string) *benchInfo {
	// Check for weg.toml
	wegTomlPath := filepath.Join(path, "weg.toml")
	if _, err := os.Stat(wegTomlPath); err == nil {
		cfg, err := config.ParseWegToml(wegTomlPath)
		if err == nil {
			return &benchInfo{
				Name:     cfg.Bench.Name,
				Path:     path,
				Version:  cfg.Frappe.Version,
				Database: cfg.Frappe.Database,
			}
		}
	}

	// Check for traditional bench structure (apps/ + sites/)
	appsDir := filepath.Join(path, "apps")
	sitesDir := filepath.Join(path, "sites")

	if _, err := os.Stat(appsDir); err == nil {
		if _, err := os.Stat(sitesDir); err == nil {
			// It's a bench, try to determine version
			name := filepath.Base(path)
			version := detectFrappeVersion(path)

			return &benchInfo{
				Name:     name,
				Path:     path,
				Version:  version,
				Database: "unknown",
			}
		}
	}

	return nil
}

func detectFrappeVersion(benchPath string) string {
	// Try to detect Frappe version from the bench
	// Check apps/frappe/frappe/__init__.py for version
	initPath := filepath.Join(benchPath, "apps", "frappe", "frappe", "__init__.py")
	if data, err := os.ReadFile(initPath); err == nil {
		content := string(data)
		// Look for __version__ = "X.Y.Z"
		if idx := strings.Index(content, "__version__"); idx >= 0 {
			line := content[idx:]
			if endIdx := strings.Index(line, "\n"); endIdx > 0 {
				line = line[:endIdx]
			}
			// Extract version
			if strings.Contains(line, "\"") {
				parts := strings.Split(line, "\"")
				if len(parts) >= 2 {
					ver := parts[1]
					// Return major version
					if dotIdx := strings.Index(ver, "."); dotIdx > 0 {
						return ver[:dotIdx]
					}
					return ver
				}
			}
		}
	}

	// Check for version in git branch name
	gitHead := filepath.Join(benchPath, "apps", "frappe", ".git", "HEAD")
	if data, err := os.ReadFile(gitHead); err == nil {
		content := strings.TrimSpace(string(data))
		if strings.Contains(content, "version-") {
			parts := strings.Split(content, "version-")
			if len(parts) >= 2 {
				ver := parts[1]
				if slashIdx := strings.Index(ver, "/"); slashIdx > 0 {
					ver = ver[:slashIdx]
				}
				return ver
			}
		}
	}

	return "?"
}
