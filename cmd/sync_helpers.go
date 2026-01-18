package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/fsutil"
	"github.com/gavindsouza/weg/internal/state"
)

// confirmSync prompts the user to confirm changes unless AssumeYes is set.
// Returns true if the user confirms, false if cancelled.
func confirmSync() bool {
	if AssumeYes() {
		return true
	}

	fmt.Print("\nApply these changes? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// displayChanges shows the diff of changes that will be applied
func displayChanges(diff *state.Diff) {
	fmt.Printf("\nChanges to apply (%d total):\n\n", diff.TotalChanges())

	if len(diff.AppsToAdd) > 0 {
		fmt.Println("Apps to install:")
		for _, app := range diff.AppsToAdd {
			fmt.Printf("  + %s\n", app)
		}
	}

	if len(diff.AppsToRemove) > 0 {
		fmt.Println("Apps to remove:")
		for _, app := range diff.AppsToRemove {
			fmt.Printf("  - %s\n", app)
		}
	}

	if len(diff.AppsToUpdate) > 0 {
		fmt.Println("Apps to update:")
		for _, update := range diff.AppsToUpdate {
			if update.NewBranch != "" {
				fmt.Printf("  ~ %s: %s -> %s\n", update.Name, update.OldBranch, update.NewBranch)
			}
			if update.NewURL != "" {
				fmt.Printf("  ~ %s: URL changed\n", update.Name)
			}
			if update.DepsChanged {
				fmt.Printf("  ~ %s: dependencies changed (will reinstall)\n", update.Name)
			}
		}
	}

	if len(diff.SitesToAdd) > 0 {
		fmt.Println("Sites to create:")
		for _, site := range diff.SitesToAdd {
			fmt.Printf("  + %s\n", site)
		}
	}

	if len(diff.SitesToRemove) > 0 {
		fmt.Println("Sites to remove:")
		for _, site := range diff.SitesToRemove {
			fmt.Printf("  - %s\n", site)
		}
	}

	if len(diff.SitesToUpdate) > 0 {
		fmt.Println("Sites to update:")
		for _, update := range diff.SitesToUpdate {
			fmt.Printf("  ~ %s\n", update.Name)
			for _, app := range update.AppsToAdd {
				fmt.Printf("      + install %s\n", app)
			}
			for _, app := range update.AppsToRemove {
				fmt.Printf("      - uninstall %s\n", app)
			}
		}
	}

	if diff.FrappeChanged {
		fmt.Println("Frappe settings changed (manual update may be required)")
	}

	if diff.ServicesChanged {
		fmt.Println("Services configuration changed:")
		if diff.NewServices.WebPort != 0 {
			fmt.Printf("  ~ web port: %d\n", diff.NewServices.WebPort)
		}
		if diff.NewServices.SocketPort != 0 {
			fmt.Printf("  ~ socket port: %d\n", diff.NewServices.SocketPort)
		}
		if len(diff.NewServices.Workers) > 0 {
			fmt.Printf("  ~ workers: %v\n", diff.NewServices.Workers)
		}
	}
}

// updateAppsTxt writes the apps.txt file with installed apps
// This file is required by frappe/bench to recognize which apps are installed
// Note: apps.txt goes in sites/ directory (frappe uses sites_path=".")
// App names must use underscores (Python module format)
func updateAppsTxt(benchPath string, st *state.State) error {
	sitesDir := filepath.Join(benchPath, "sites")
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		return fmt.Errorf("failed to create sites directory: %w", err)
	}
	appsTxtPath := filepath.Join(sitesDir, "apps.txt")

	// Get app names in order (frappe first)
	var apps []string
	hasFramework := false
	for name := range st.Apps {
		// Convert to Python module name (hyphens to underscores)
		moduleName := strings.ReplaceAll(name, "-", "_")
		if moduleName == "frappe" {
			hasFramework = true
		} else {
			apps = append(apps, moduleName)
		}
	}

	// Sort and prepend frappe
	var orderedApps []string
	if hasFramework {
		orderedApps = append(orderedApps, "frappe")
	}
	orderedApps = append(orderedApps, apps...)

	// Write apps.txt atomically
	content := strings.Join(orderedApps, "\n") + "\n"
	return fsutil.AtomicWriteString(appsTxtPath, content, 0644)
}

// setupAssets creates symlinks for app assets in sites/assets/
// This is required for Frappe to serve static files (images, css, js, etc.)
func setupAssets(benchPath string) error {
	appsDir := filepath.Join(benchPath, "apps")
	assetsDir := filepath.Join(benchPath, "sites", "assets")

	// Ensure assets directory exists
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Read apps directory
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("failed to read apps directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appName := entry.Name()
		// App's public directory: apps/{app}/{app}/public/
		publicDir := filepath.Join(appsDir, appName, appName, "public")

		if _, err := os.Stat(publicDir); os.IsNotExist(err) {
			// Try without nested directory (some apps have public at root)
			publicDir = filepath.Join(appsDir, appName, "public")
			if _, err := os.Stat(publicDir); os.IsNotExist(err) {
				continue // No public directory
			}
		}

		// Create symlink: sites/assets/{app} -> apps/{app}/{app}/public/
		assetLink := filepath.Join(assetsDir, appName)

		// Remove existing link/directory if it exists
		if info, err := os.Lstat(assetLink); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				// It's a symlink, check if it points to the right place
				target, _ := os.Readlink(assetLink)
				if target == publicDir {
					continue // Already correct
				}
			}
			os.RemoveAll(assetLink)
		}

		// Create relative symlink
		relPath, err := filepath.Rel(assetsDir, publicDir)
		if err != nil {
			relPath = publicDir // Fall back to absolute
		}

		if err := os.Symlink(relPath, assetLink); err != nil {
			PrintVerbose("Warning: failed to create asset symlink for %s: %v", appName, err)
		}
	}

	return nil
}

// ensureCommonSiteConfig creates common_site_config.json from weg.toml config
// If benchConfig is nil, uses defaults
func ensureCommonSiteConfig(benchPath string, benchConfig *config.BenchConfig) error {
	sitesDir := filepath.Join(benchPath, "sites")
	configPath := filepath.Join(sitesDir, "common_site_config.json")

	var cfg map[string]interface{}

	if benchConfig != nil {
		// Generate config from weg.toml settings
		cfg = benchConfig.GenerateCommonSiteConfig(nil)
	} else {
		// Use defaults
		cfg = map[string]interface{}{
			"redis_cache":    "redis://localhost:6379/0",
			"redis_queue":    "redis://localhost:6379/1",
			"redis_socketio": "redis://localhost:6379/2",
			"webserver_port": 8000,
			"socketio_port":  9000,
			"developer_mode": 1,
		}
	}

	// Write config atomically
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	return fsutil.AtomicWrite(configPath, data, 0644)
}
