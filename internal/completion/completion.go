// Package completion provides shell completion helpers for weg CLI commands.
package completion

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

// GetBenchPath detects the bench path from the current directory.
// Returns the bench path or empty string if not in a weg-managed project.
func GetBenchPath() string {
	absPath, err := filepath.Abs(".")
	if err != nil {
		return ""
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return ""
	}

	switch result.Context {
	case config.ContextWegBench:
		return absPath
	case config.ContextWegApp:
		return filepath.Join(absPath, ".weg")
	default:
		return ""
	}
}

// GetSiteNames returns a list of site names from the sites directory.
func GetSiteNames() []string {
	benchPath := GetBenchPath()
	if benchPath == "" {
		return nil
	}

	sitesDir := filepath.Join(benchPath, "sites")
	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return nil
	}

	var sites []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden directories, assets, and common non-site directories
		if strings.HasPrefix(name, ".") || name == "assets" || name == "common_site_config.json" {
			continue
		}
		// Verify it looks like a site (has site_config.json)
		siteConfigPath := filepath.Join(sitesDir, name, "site_config.json")
		if _, err := os.Stat(siteConfigPath); err == nil {
			sites = append(sites, name)
		}
	}

	return sites
}

// GetAppNames returns a list of app names from the apps directory.
func GetAppNames() []string {
	benchPath := GetBenchPath()
	if benchPath == "" {
		return nil
	}

	appsDir := filepath.Join(benchPath, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil
	}

	var apps []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		apps = append(apps, name)
	}

	return apps
}

// GetInstalledAppNames returns app names from state (installed apps).
func GetInstalledAppNames() []string {
	absPath, err := filepath.Abs(".")
	if err != nil {
		return nil
	}

	st, err := state.Load(absPath)
	if err != nil {
		return nil
	}

	return st.AppNames()
}

// CommonDocTypes returns a list of common Frappe DocTypes for completion.
// These are frequently used doctypes that don't require database access.
var CommonDocTypes = []string{
	// Core
	"User",
	"Role",
	"DocType",
	"Module Def",
	"Report",
	"Page",
	"Print Format",
	"Workflow",
	"Workflow State",
	"Workflow Action",
	// ERPNext common
	"Company",
	"Customer",
	"Supplier",
	"Item",
	"Sales Order",
	"Sales Invoice",
	"Purchase Order",
	"Purchase Invoice",
	"Stock Entry",
	"Journal Entry",
	"Payment Entry",
	"Employee",
	// Frappe Framework
	"File",
	"Comment",
	"Communication",
	"Email Account",
	"Email Template",
	"Notification",
	"Web Page",
	"Blog Post",
	"System Settings",
	"Website Settings",
}

// CompleteSiteNames provides cobra completion for site names.
func CompleteSiteNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	sites := GetSiteNames()
	if sites == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string
	for _, site := range sites {
		if strings.HasPrefix(site, toComplete) {
			matches = append(matches, site)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// CompleteAppNames provides cobra completion for app names from apps directory.
func CompleteAppNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	apps := GetAppNames()
	if apps == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string
	for _, app := range apps {
		if strings.HasPrefix(app, toComplete) {
			matches = append(matches, app)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// CompleteInstalledAppNames provides cobra completion for installed app names.
func CompleteInstalledAppNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	apps := GetInstalledAppNames()
	if apps == nil {
		// Fall back to apps directory
		apps = GetAppNames()
	}
	if apps == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string
	for _, app := range apps {
		if strings.HasPrefix(app, toComplete) {
			matches = append(matches, app)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// CompleteDocTypes provides cobra completion for DocType names.
func CompleteDocTypes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var matches []string
	toCompleteLower := strings.ToLower(toComplete)

	for _, dt := range CommonDocTypes {
		if strings.HasPrefix(strings.ToLower(dt), toCompleteLower) {
			matches = append(matches, dt)
		}
	}

	// Also allow any input (user might have custom doctypes)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// CompleteSiteNamesForArg returns a completion function that completes site names
// only for the specified argument position.
func CompleteSiteNamesForArg(argPos int) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != argPos {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return CompleteSiteNames(cmd, args, toComplete)
	}
}

// CompleteAppNamesForArg returns a completion function that completes app names
// only for the specified argument position.
func CompleteAppNamesForArg(argPos int) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != argPos {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return CompleteAppNames(cmd, args, toComplete)
	}
}

// CompleteDocTypesForArg returns a completion function that completes doctype names
// only for the specified argument position.
func CompleteDocTypesForArg(argPos int) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != argPos {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return CompleteDocTypes(cmd, args, toComplete)
	}
}

// CompleteNone returns no completions (useful for disabling file completion).
func CompleteNone(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
