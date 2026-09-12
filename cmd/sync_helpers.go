package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/fsutil"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/gavindsouza/weg/internal/state"
)

// confirmSync prompts the user to confirm changes unless AssumeYes is set.
// Returns true if the user confirms, false if cancelled.
func confirmSync() bool {
	if AssumeYes() {
		return true
	}

	output.Print("")
	return prompt.Confirm("Apply these changes?")
}

// displayChanges shows the diff of changes that will be applied
func displayChanges(diff *state.Diff) {
	output.Printf("\nChanges to apply (%d total):\n", diff.TotalChanges())

	if len(diff.AppsToAdd) > 0 {
		output.Print("Apps to install:")
		for _, app := range diff.AppsToAdd {
			output.Printf("  + %s", app)
		}
	}

	if len(diff.AppsToRemove) > 0 {
		output.Print("Apps to remove:")
		for _, app := range diff.AppsToRemove {
			output.Printf("  - %s", app)
		}
	}

	if len(diff.AppsToUpdate) > 0 {
		output.Print("Apps to update:")
		for _, update := range diff.AppsToUpdate {
			if update.NewBranch != "" {
				output.Printf("  ~ %s: %s -> %s", update.Name, update.OldBranch, update.NewBranch)
			}
			if update.NewURL != "" {
				output.Printf("  ~ %s: URL changed", update.Name)
			}
			if update.DepsChanged {
				output.Printf("  ~ %s: dependencies changed (will reinstall)", update.Name)
			}
		}
	}

	if len(diff.SitesToAdd) > 0 {
		output.Print("Sites to create:")
		for _, site := range diff.SitesToAdd {
			output.Printf("  + %s", site)
		}
	}

	if len(diff.SitesToRemove) > 0 {
		output.Print("Sites to remove:")
		for _, site := range diff.SitesToRemove {
			output.Printf("  - %s", site)
		}
	}

	if len(diff.SitesToUpdate) > 0 {
		output.Print("Sites to update:")
		for _, update := range diff.SitesToUpdate {
			output.Printf("  ~ %s", update.Name)
			for _, app := range update.AppsToAdd {
				output.Printf("      + install %s", app)
			}
			for _, app := range update.AppsToRemove {
				output.Printf("      - uninstall %s", app)
			}
		}
	}

	if diff.FrappeChanged {
		output.Print("Frappe settings changed (manual update may be required)")
	}

	if diff.ServicesChanged {
		output.Print("Services configuration changed:")
		if diff.NewServices.WebPort != 0 {
			output.Printf("  ~ web port: %d", diff.NewServices.WebPort)
		}
		if diff.NewServices.SocketPort != 0 {
			output.Printf("  ~ socket port: %d", diff.NewServices.SocketPort)
		}
		if len(diff.NewServices.Workers) > 0 {
			output.Printf("  ~ workers: %v", diff.NewServices.Workers)
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
		appName := entry.Name()
		// Stat, not entry.IsDir(): path apps are symlinked into apps/.
		if info, err := os.Stat(filepath.Join(appsDir, appName)); err != nil || !info.IsDir() {
			continue
		}

		// App's public directory: apps/{app}/{app}/public/
		publicDir := filepath.Join(appsDir, appName, appName, "public")

		if _, err := os.Stat(publicDir); os.IsNotExist(err) {
			// Try without nested directory (some apps have public at root)
			publicDir = filepath.Join(appsDir, appName, "public")
			if _, err := os.Stat(publicDir); os.IsNotExist(err) {
				continue // No public directory
			}
		}

		// Create relative symlink: sites/assets/{app} -> apps/{app}/{app}/public/
		assetLink := filepath.Join(assetsDir, appName)
		relPath, err := filepath.Rel(assetsDir, publicDir)
		if err != nil {
			relPath = publicDir // Fall back to absolute
		}
		if target, err := os.Readlink(assetLink); err != nil || target != relPath {
			os.RemoveAll(assetLink)
			if err := os.Symlink(relPath, assetLink); err != nil {
				PrintVerbose("Warning: failed to create asset symlink for %s: %v", appName, err)
			}
		}

		// frappe's build links the app's node_modules into its public dir
		// (/assets/frappe/node_modules/... e.g. frappe-gantt); sync builds
		// nothing, so make the same link here.
		nodeModules := filepath.Join(appsDir, appName, "node_modules")
		publicNodeModules := filepath.Join(publicDir, "node_modules")
		if fsutil.DirExists(nodeModules) {
			if _, err := os.Lstat(publicNodeModules); os.IsNotExist(err) {
				if rel, err := filepath.Rel(publicDir, nodeModules); err == nil {
					if err := os.Symlink(rel, publicNodeModules); err != nil {
						PrintVerbose("Warning: failed to link node_modules for %s: %v", appName, err)
					}
				}
			}
		}
	}

	return nil
}

// ensureCommonSiteConfig creates common_site_config.json from weg.toml config
// If benchConfig is nil, uses defaults
func ensureCommonSiteConfig(benchPath string, benchConfig *config.BenchConfig) error {
	sitesDir := filepath.Join(benchPath, "sites")
	configPath := filepath.Join(sitesDir, "common_site_config.json")

	var cfg map[string]any

	if benchConfig != nil {
		// Generate config from weg.toml settings
		cfg = benchConfig.GenerateCommonSiteConfig(benchPath, nil)
	} else {
		// Use defaults with Unix socket for Redis (no port conflicts)
		redisSocketPath := filepath.Join(benchPath, ".devbox", "virtenv", "redis", "redis.sock")
		cfg = map[string]any{
			"redis_cache":    fmt.Sprintf("unix://%s?db=0", redisSocketPath),
			"redis_queue":    fmt.Sprintf("unix://%s?db=1", redisSocketPath),
			"redis_socketio": fmt.Sprintf("unix://%s?db=2", redisSocketPath),
			"webserver_port": 8000,
			"socketio_port":  9000,
			"developer_mode": 1,
		}
	}

	return mergeConfigJSON(configPath, cfg)
}

// mergeConfigJSON overlays the weg-managed keys onto the existing
// common_site_config.json so keys added by the user or by frappe survive.
// An unparseable existing file is an error rather than something to clobber.
func mergeConfigJSON(configPath string, managed map[string]any) error {
	cfg := map[string]any{}
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(strings.TrimSpace(string(data))) > 0 {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber() // keep large ints exact through the round-trip
		if err := dec.Decode(&cfg); err != nil {
			return fmt.Errorf("failed to parse %s: %w", configPath, err)
		}
	}
	maps.Copy(cfg, managed)

	data, err = json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	// Atomic: a partial write would leave Frappe with truncated JSON. Keep the
	// file's mode: site_config.json holds the database password.
	mode := os.FileMode(0644)
	if fi, err := os.Stat(configPath); err == nil {
		mode = fi.Mode().Perm()
	}
	return fsutil.AtomicWrite(configPath, data, mode)
}
