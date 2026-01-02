package state

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
)

// sortAppsToAdd ensures frappe is always first in the install order
func sortAppsToAdd(apps []string) {
	sort.Slice(apps, func(i, j int) bool {
		// frappe always comes first
		if apps[i] == "frappe" {
			return true
		}
		if apps[j] == "frappe" {
			return false
		}
		// Otherwise alphabetical
		return apps[i] < apps[j]
	})
}

// Diff represents the differences between desired config and current state
type Diff struct {
	AppsToAdd     []string
	AppsToRemove  []string
	AppsToUpdate  []AppUpdate
	SitesToAdd    []string
	SitesToRemove []string
	SitesToUpdate []SiteUpdate
	ConfigChanged bool
	FrappeChanged bool
}

// AppUpdate represents an app that needs updating
type AppUpdate struct {
	Name        string
	OldBranch   string
	NewBranch   string
	OldURL      string
	NewURL      string
	DepsChanged bool // pyproject.toml changed, needs pip reinstall
}

// SiteUpdate represents a site that needs updating
type SiteUpdate struct {
	Name       string
	AppsToAdd  []string
	AppsToRemove []string
}

// IsEmpty returns true if no changes are needed
func (d *Diff) IsEmpty() bool {
	return len(d.AppsToAdd) == 0 &&
		len(d.AppsToRemove) == 0 &&
		len(d.AppsToUpdate) == 0 &&
		len(d.SitesToAdd) == 0 &&
		len(d.SitesToRemove) == 0 &&
		len(d.SitesToUpdate) == 0 &&
		!d.FrappeChanged
}

// HasChanges returns true if there are any changes
func (d *Diff) HasChanges() bool {
	return !d.IsEmpty()
}

// TotalChanges returns the total number of changes
func (d *Diff) TotalChanges() int {
	count := len(d.AppsToAdd) + len(d.AppsToRemove) + len(d.AppsToUpdate)
	count += len(d.SitesToAdd) + len(d.SitesToRemove) + len(d.SitesToUpdate)
	if d.FrappeChanged {
		count++
	}
	return count
}

// ComputeDiffFromBenchConfig computes the diff between a BenchConfig and current state
// benchPath is used to check pyproject.toml changes for installed apps
func ComputeDiffFromBenchConfig(cfg *config.BenchConfig, state *State, benchPath string) *Diff {
	diff := &Diff{}

	// Get enabled apps from config
	enabledApps := cfg.EnabledApps()

	// Find apps to add (in config but not in state)
	for name := range enabledApps {
		if !state.HasApp(name) {
			diff.AppsToAdd = append(diff.AppsToAdd, name)
		}
	}

	// Find apps to remove (in state but not in config or excluded)
	for name := range state.Apps {
		if _, ok := enabledApps[name]; !ok {
			diff.AppsToRemove = append(diff.AppsToRemove, name)
		}
	}

	// Find apps to update (different branch, URL, or pyproject.toml)
	for name, appCfg := range enabledApps {
		if appState, ok := state.Apps[name]; ok {
			needsUpdate := false
			update := AppUpdate{Name: name}

			if appCfg.Branch != "" && appCfg.Branch != appState.Branch {
				update.OldBranch = appState.Branch
				update.NewBranch = appCfg.Branch
				needsUpdate = true
			}
			if appCfg.URL != "" && appCfg.URL != appState.URL {
				update.OldURL = appState.URL
				update.NewURL = appCfg.URL
				needsUpdate = true
			}

			// Check if pyproject.toml changed (deps need reinstall)
			if benchPath != "" {
				appPath := filepath.Join(benchPath, "apps", name)
				currentHash := ComputePyprojectHash(appPath)
				if currentHash != "" && currentHash != appState.PyprojectHash {
					// Hash differs (or stored hash was empty - first time tracking)
					update.DepsChanged = true
					needsUpdate = true
				}
			}

			if needsUpdate {
				diff.AppsToUpdate = append(diff.AppsToUpdate, update)
			}
		}
	}

	// Build map of config sites
	configSites := make(map[string]config.SiteConfig)
	for _, site := range cfg.Sites {
		configSites[site.Name] = site
	}

	// Find sites to add
	for name := range configSites {
		if !state.HasSite(name) {
			diff.SitesToAdd = append(diff.SitesToAdd, name)
		}
	}

	// Find sites to remove
	for name := range state.Sites {
		if _, ok := configSites[name]; !ok {
			diff.SitesToRemove = append(diff.SitesToRemove, name)
		}
	}

	// Find sites to update (apps changed)
	for name, siteCfg := range configSites {
		if siteState, ok := state.Sites[name]; ok {
			update := computeSiteUpdate(name, siteCfg.Apps, siteState.Apps)
			if update != nil {
				diff.SitesToUpdate = append(diff.SitesToUpdate, *update)
			}
		}
	}

	// Check if Frappe settings changed
	if state.Frappe.Version != "" {
		if cfg.Frappe.Version != state.Frappe.Version ||
			cfg.Frappe.Database != state.Frappe.Database {
			diff.FrappeChanged = true
		}
	}

	// Sort apps to ensure frappe is always installed first
	sortAppsToAdd(diff.AppsToAdd)

	return diff
}

// ComputeDiffFromAppConfig computes the diff for app-centric configs
func ComputeDiffFromAppConfig(cfg *config.AppConfig, appName string, state *State) *Diff {
	diff := &Diff{}

	// For app-centric, we mainly care about dependencies
	for _, dep := range cfg.Dependencies.Apps {
		if !state.HasApp(dep.Name) {
			diff.AppsToAdd = append(diff.AppsToAdd, dep.Name)
		}
	}

	// Check if the main app is installed
	if !state.HasApp(appName) {
		diff.AppsToAdd = append(diff.AppsToAdd, appName)
	}

	// Check if frappe is installed
	if !state.HasApp("frappe") {
		diff.AppsToAdd = append(diff.AppsToAdd, "frappe")
	}

	// Build set of desired apps
	desiredApps := make(map[string]bool)
	desiredApps["frappe"] = true
	desiredApps[appName] = true
	for _, dep := range cfg.Dependencies.Apps {
		desiredApps[dep.Name] = true
	}

	// Find apps to remove (in state but not desired)
	for name := range state.Apps {
		if !desiredApps[name] {
			diff.AppsToRemove = append(diff.AppsToRemove, name)
		}
	}

	// Handle default site for app-centric projects
	defaultSiteName := toModuleName(appName) + ".localhost"
	if !state.HasSite(defaultSiteName) {
		diff.SitesToAdd = append(diff.SitesToAdd, defaultSiteName)
	} else {
		// Check if apps need to be installed on the site
		siteState := state.Sites[defaultSiteName]
		var desiredSiteApps []string
		for app := range desiredApps {
			desiredSiteApps = append(desiredSiteApps, app)
		}
		if update := computeSiteUpdate(defaultSiteName, desiredSiteApps, siteState.Apps); update != nil {
			diff.SitesToUpdate = append(diff.SitesToUpdate, *update)
		}
	}

	// Sort apps to ensure frappe is always installed first
	sortAppsToAdd(diff.AppsToAdd)

	return diff
}

// toModuleName converts an app name to its Python module name
func toModuleName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// computeSiteUpdate checks if a site needs app updates
func computeSiteUpdate(name string, configApps, stateApps []string) *SiteUpdate {
	configSet := make(map[string]bool)
	stateSet := make(map[string]bool)

	for _, app := range configApps {
		configSet[app] = true
	}
	for _, app := range stateApps {
		stateSet[app] = true
	}

	update := &SiteUpdate{Name: name}

	// Apps to add to site
	for app := range configSet {
		if !stateSet[app] {
			update.AppsToAdd = append(update.AppsToAdd, app)
		}
	}

	// Apps to remove from site
	for app := range stateSet {
		if !configSet[app] {
			update.AppsToRemove = append(update.AppsToRemove, app)
		}
	}

	if len(update.AppsToAdd) == 0 && len(update.AppsToRemove) == 0 {
		return nil
	}

	return update
}
