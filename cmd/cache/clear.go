package cache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var (
	clearSite string
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear Frappe cache",
	Long: `Clear the Frappe cache including Redis and local bytecode files.

This is commonly used when:
  - Troubleshooting display issues
  - After restoring from backup
  - After making changes to doctypes
  - When forms or lists are not reflecting changes

Examples:
  weg cache clear              # Clear cache for default site
  weg cache clear --site test  # Clear cache for specific site
  weg cache clear --all        # Clear cache for all sites`,
	RunE: runClear,
}

var clearAll bool

func init() {
	clearCmd.Flags().StringVar(&clearSite, "site", "", "Site to clear cache for")
	clearCmd.Flags().BoolVar(&clearAll, "all", false, "Clear cache for all sites")
}

func runClear(cmd *cobra.Command, args []string) error {
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

	// Load state to get sites
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	// Determine which sites to clear
	var sites []string
	if clearAll {
		sites = st.SiteNames()
		if len(sites) == 0 {
			// Try to get sites from sites directory
			sitesDir := filepath.Join(benchPath, "sites")
			entries, _ := os.ReadDir(sitesDir)
			for _, e := range entries {
				if e.IsDir() && e.Name() != "assets" && e.Name() != "common_site_config.json" {
					sites = append(sites, e.Name())
				}
			}
		}
	} else if clearSite != "" {
		sites = []string{clearSite}
	} else {
		// Use default site
		site := st.GetDefaultSite()
		if site == "" {
			// Try currentsite.txt
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, err := os.ReadFile(currentSitePath)
			if err == nil {
				site = string(data)
			}
		}
		if site == "" {
			return fmt.Errorf("no site specified and no default site found. Use --site or --all")
		}
		sites = []string{site}
	}

	if len(sites) == 0 {
		return fmt.Errorf("no sites found to clear cache")
	}

	// Clear cache for each site
	for _, site := range sites {
		output.Infof("Clearing cache for %s...\n", site)

		executor := api.NewExecutor(benchPath, site, "Administrator")

		// Call frappe.clear_cache()
		script := `import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    frappe.clear_cache()
    print(json.dumps({"success": True, "data": "cache cleared"}))
except Exception as ex:
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`
		sitesDir := filepath.Join(benchPath, "sites")
		formattedScript := fmt.Sprintf(script, sitesDir, site)

		result, err := executor.ExecuteRaw(formattedScript)
		if err != nil {
			return fmt.Errorf("failed to clear cache for %s: %w", site, err)
		}

		if !result.Success {
			return fmt.Errorf("failed to clear cache for %s: %s", site, result.Error)
		}
	}

	// Also clear local Python bytecode
	appsDir := filepath.Join(benchPath, "apps")
	if _, err := os.Stat(appsDir); err == nil {
		fmt.Println("Clearing Python bytecode...")
		clearPycache(appsDir)
	}

	if len(sites) == 1 {
		fmt.Printf("Cache cleared for %s\n", sites[0])
	} else {
		fmt.Printf("Cache cleared for %d sites\n", len(sites))
	}
	return nil
}

// clearPycache removes __pycache__ directories and .pyc files
func clearPycache(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && info.Name() == "__pycache__" {
			os.RemoveAll(path)
			return filepath.SkipDir
		}

		if !info.IsDir() && filepath.Ext(path) == ".pyc" {
			os.Remove(path)
		}

		return nil
	})
}
