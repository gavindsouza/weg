package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/api"
	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var clearWebsiteCacheCmd = &cobra.Command{
	Use:   "clear-website-cache",
	Short: "Clear website page cache",
	Long: `Clear the website page cache for a site.

This clears cached HTML pages, forcing them to be regenerated
on the next request. Useful after making changes to web templates.

Examples:
  weg clear-website-cache
  weg clear-website-cache --site test.localhost`,
	RunE: runClearWebsiteCache,
}

var clearWebsiteCacheSite string

func init() {
	rootCmd.AddCommand(clearWebsiteCacheCmd)
	clearWebsiteCacheCmd.Flags().StringVar(&clearWebsiteCacheSite, "site", "", "Site to clear cache for")
}

func runClearWebsiteCache(cmd *cobra.Command, args []string) error {
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

	// Determine site
	site := clearWebsiteCacheSite
	if site == "" {
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

	executor := api.NewExecutor(benchPath, site, "Administrator")

	script := fmt.Sprintf(`import frappe
import json
import os

os.chdir('%s')
frappe.init(site='%s')
frappe.connect()

try:
    from frappe.website.utils import clear_website_cache
    clear_website_cache()
    frappe.db.commit()
    print(json.dumps({"success": True}))
except Exception as ex:
    frappe.db.rollback()
    print(json.dumps({"success": False, "error": str(ex)}))
finally:
    frappe.destroy()
`, filepath.Join(benchPath, "sites"), site)

	apiResult, err := executor.ExecuteRaw(script)
	if err != nil {
		return fmt.Errorf("failed to clear website cache: %w", err)
	}

	if !apiResult.Success {
		return fmt.Errorf("failed to clear website cache: %s", apiResult.Error)
	}

	PrintInfo("Website cache cleared for %s", site)
	return nil
}
