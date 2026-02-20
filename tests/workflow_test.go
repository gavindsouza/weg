package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/gavindsouza/weg/internal/testutil"
)

// TestDetectProjectContext_WalkUp verifies that DetectProjectContext
// finds the bench root when called from a subdirectory.
func TestDetectProjectContext_WalkUp(t *testing.T) {
	bench := testutil.NewBench(t).
		WithApp("frappe").
		Build()

	// Create a deeply nested subdirectory
	subdir := filepath.Join(bench, "apps", "frappe", "frappe", "core")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// DetectProjectContext from subdirectory should find the bench root
	result, err := config.DetectProjectContext(subdir)
	if err != nil {
		t.Fatalf("DetectProjectContext failed: %v", err)
	}

	if result.Context != config.ContextWegBench {
		t.Errorf("expected ContextWegBench, got %v", result.Context)
	}
	if result.Path != bench {
		t.Errorf("expected path %s, got %s", bench, result.Path)
	}
}

// TestDetectProjectContext_DirectDetection verifies that DetectProjectContext
// detects the context directly when called from the project root.
func TestDetectProjectContext_DirectDetection(t *testing.T) {
	bench := testutil.NewBench(t).Build()

	result, err := config.DetectProjectContext(bench)
	if err != nil {
		t.Fatalf("DetectProjectContext failed: %v", err)
	}

	if result.Context != config.ContextWegBench {
		t.Errorf("expected ContextWegBench, got %v", result.Context)
	}
}

// TestDetectProjectContext_FreshDir verifies that a fresh directory
// returns ContextFresh even after walk-up.
func TestDetectProjectContext_FreshDir(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := config.DetectProjectContext(tmpDir)
	if err != nil {
		t.Fatalf("DetectProjectContext failed: %v", err)
	}

	if result.Context != config.ContextFresh {
		t.Errorf("expected ContextFresh, got %v", result.Context)
	}
}

// TestSyncDiff_NewBench verifies that a fresh bench with apps in config
// produces the right diff (apps to add, sites to add).
func TestSyncDiff_NewBench(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[[sites]]
name = "test.localhost"
apps = ["frappe", "erpnext"]
`).
		Build()

	benchConfig, err := config.ParseWegToml(bench)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	st := state.NewState()
	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, bench)

	// Should need to add both apps
	if len(diff.AppsToAdd) != 2 {
		t.Errorf("expected 2 apps to add, got %d: %v", len(diff.AppsToAdd), diff.AppsToAdd)
	}

	// frappe should be first (install order)
	if len(diff.AppsToAdd) > 0 && diff.AppsToAdd[0] != "frappe" {
		t.Errorf("expected frappe first in install order, got %s", diff.AppsToAdd[0])
	}

	// Should need to add the site
	if len(diff.SitesToAdd) != 1 {
		t.Errorf("expected 1 site to add, got %d", len(diff.SitesToAdd))
	}
	if len(diff.SitesToAdd) > 0 && diff.SitesToAdd[0] != "test.localhost" {
		t.Errorf("expected site test.localhost, got %s", diff.SitesToAdd[0])
	}

	if diff.IsEmpty() {
		t.Error("diff should not be empty for fresh bench")
	}
}

// TestSyncDiff_UpToDate verifies that no diff is produced when state
// matches config.
func TestSyncDiff_UpToDate(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[[sites]]
name = "test.localhost"
apps = ["frappe"]
`).
		Build()

	benchConfig, err := config.ParseWegToml(bench)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// Create state that matches config
	st := state.NewState()
	st.Apps["frappe"] = state.AppState{
		Name:   "frappe",
		URL:    "https://github.com/frappe/frappe",
		Branch: "version-15",
	}
	st.Sites["test.localhost"] = state.SiteState{
		Name: "test.localhost",
		Apps: []string{"frappe"},
	}
	st.Frappe.Version = "15"
	st.Frappe.Database = "mariadb"
	st.Services.WebPort = 8000
	st.Services.SocketPort = 9000
	st.Services.Workers = map[string]int{"all": 1}

	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, bench)

	if !diff.IsEmpty() {
		t.Errorf("expected empty diff when state matches config; got %d changes (apps+:%v apps-:%v sites+:%v sites-:%v)",
			diff.TotalChanges(), diff.AppsToAdd, diff.AppsToRemove, diff.SitesToAdd, diff.SitesToRemove)
	}
}

// TestSyncDiff_AppAdded verifies diff detects a newly added app.
func TestSyncDiff_AppAdded(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"
`).
		Build()

	benchConfig, err := config.ParseWegToml(bench)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// State only has frappe
	st := state.NewState()
	st.Apps["frappe"] = state.AppState{
		Name:   "frappe",
		URL:    "https://github.com/frappe/frappe",
		Branch: "version-15",
	}
	st.Frappe.Version = "15"
	st.Frappe.Database = "mariadb"
	st.Services.WebPort = 8000
	st.Services.SocketPort = 9000
	st.Services.Workers = map[string]int{"all": 1}

	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, bench)

	if len(diff.AppsToAdd) != 1 || diff.AppsToAdd[0] != "erpnext" {
		t.Errorf("expected [erpnext] to add, got %v", diff.AppsToAdd)
	}
	if len(diff.AppsToRemove) != 0 {
		t.Errorf("expected no apps to remove, got %v", diff.AppsToRemove)
	}
}

// TestSyncDiff_AppRemoved verifies diff detects a removed app.
func TestSyncDiff_AppRemoved(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"
`).
		Build()

	benchConfig, err := config.ParseWegToml(bench)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// State has both frappe and erpnext
	st := state.NewState()
	st.Apps["frappe"] = state.AppState{Name: "frappe", Branch: "version-15"}
	st.Apps["erpnext"] = state.AppState{Name: "erpnext", Branch: "version-15"}
	st.Frappe.Version = "15"
	st.Frappe.Database = "mariadb"
	st.Services.WebPort = 8000
	st.Services.SocketPort = 9000
	st.Services.Workers = map[string]int{"all": 1}

	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, bench)

	if len(diff.AppsToRemove) != 1 || diff.AppsToRemove[0] != "erpnext" {
		t.Errorf("expected [erpnext] to remove, got %v", diff.AppsToRemove)
	}
}

// TestSyncDiff_BranchUpdate verifies diff detects a branch change.
func TestSyncDiff_BranchUpdate(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "16"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-16"
`).
		Build()

	benchConfig, err := config.ParseWegToml(bench)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	st := state.NewState()
	st.Apps["frappe"] = state.AppState{
		Name:   "frappe",
		URL:    "https://github.com/frappe/frappe",
		Branch: "version-15",
	}
	st.Frappe.Version = "15"
	st.Frappe.Database = "mariadb"
	st.Services.WebPort = 8000
	st.Services.SocketPort = 9000
	st.Services.Workers = map[string]int{"all": 1}

	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, bench)

	if len(diff.AppsToUpdate) != 1 {
		t.Fatalf("expected 1 app to update, got %d", len(diff.AppsToUpdate))
	}
	update := diff.AppsToUpdate[0]
	if update.Name != "frappe" {
		t.Errorf("expected frappe update, got %s", update.Name)
	}
	if update.OldBranch != "version-15" || update.NewBranch != "version-16" {
		t.Errorf("expected branch change 15->16, got %s->%s", update.OldBranch, update.NewBranch)
	}
	if !diff.FrappeChanged {
		t.Error("expected FrappeChanged to be true")
	}
}

// TestConfigHash_NeedsSync verifies that modifying a config file
// causes NeedsSync to return true.
func TestConfigHash_NeedsSync(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"
`).
		Build()

	wegTomlPath := filepath.Join(bench, "weg.toml")

	// Compute initial hash
	hash, err := state.ComputeConfigHash(wegTomlPath)
	if err != nil {
		t.Fatalf("ComputeConfigHash failed: %v", err)
	}

	st := state.NewState()
	st.UpdateConfigHash(hash)

	// Should not need sync with same config
	needsSync, err := st.NeedsSync(wegTomlPath)
	if err != nil {
		t.Fatalf("NeedsSync failed: %v", err)
	}
	if needsSync {
		t.Error("should not need sync when config unchanged")
	}

	// Modify config
	if err := os.WriteFile(wegTomlPath, []byte(`[frappe]
version = "16"
`), 0644); err != nil {
		t.Fatalf("failed to write weg.toml: %v", err)
	}

	// Should need sync now
	needsSync, err = st.NeedsSync(wegTomlPath)
	if err != nil {
		t.Fatalf("NeedsSync failed: %v", err)
	}
	if !needsSync {
		t.Error("should need sync after config change")
	}
}

// TestStatePersistence verifies that state can be saved and loaded
// across the full lifecycle.
func TestStatePersistence(t *testing.T) {
	bench := testutil.NewBench(t).Build()

	// Create state directory
	stateDir := filepath.Join(bench, ".weg")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create .weg: %v", err)
	}

	// Create and save state
	st := state.NewState()
	st.Apps["frappe"] = state.AppState{
		Name:        "frappe",
		URL:         "https://github.com/frappe/frappe",
		Branch:      "version-15",
		InstalledAt: time.Now().Truncate(time.Second),
	}
	st.Sites["test.localhost"] = state.SiteState{
		Name:      "test.localhost",
		Apps:      []string{"frappe"},
		CreatedAt: time.Now().Truncate(time.Second),
	}
	st.Frappe.Version = "15"
	st.Frappe.Database = "mariadb"

	if err := st.Save(bench); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and verify
	loaded, err := state.Load(bench)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(loaded.Apps))
	}
	if !loaded.HasApp("frappe") {
		t.Error("expected frappe app")
	}
	if loaded.Apps["frappe"].Branch != "version-15" {
		t.Errorf("expected branch version-15, got %s", loaded.Apps["frappe"].Branch)
	}
	if len(loaded.Sites) != 1 {
		t.Errorf("expected 1 site, got %d", len(loaded.Sites))
	}
	if !loaded.HasSite("test.localhost") {
		t.Error("expected test.localhost site")
	}
	if loaded.Frappe.Version != "15" {
		t.Errorf("expected frappe version 15, got %s", loaded.Frappe.Version)
	}
}

// TestAppCentricDiff verifies diff computation for app-centric projects
// (pyproject.toml with [tool.weg]).
func TestAppCentricDiff(t *testing.T) {
	tmpDir := t.TempDir()

	pyproject := `[project]
name = "my-app"
version = "0.0.1"

[tool.weg]
[tool.weg.compatibility]
frappe = ["15"]

[tool.weg.dev]
frappe = "15"
database = "mariadb"

[tool.weg.dependencies]
apps = [
  { name = "erpnext", url = "https://github.com/frappe/erpnext", branch = "version-15" },
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatalf("failed to write pyproject.toml: %v", err)
	}

	appConfig, err := config.ParsePyproject(tmpDir)
	if err != nil {
		t.Fatalf("ParsePyproject failed: %v", err)
	}

	st := state.NewState()
	diff := state.ComputeDiffFromAppConfig(appConfig, "my-app", st)

	// Should need frappe, my-app, and erpnext
	if len(diff.AppsToAdd) != 3 {
		t.Errorf("expected 3 apps to add, got %d: %v", len(diff.AppsToAdd), diff.AppsToAdd)
	}

	// frappe should be first
	if len(diff.AppsToAdd) > 0 && diff.AppsToAdd[0] != "frappe" {
		t.Errorf("expected frappe first, got %s", diff.AppsToAdd[0])
	}

	// Should create default site
	if len(diff.SitesToAdd) != 1 {
		t.Errorf("expected 1 site to add, got %d", len(diff.SitesToAdd))
	}
	if len(diff.SitesToAdd) > 0 && diff.SitesToAdd[0] != "my_app.localhost" {
		t.Errorf("expected my_app.localhost, got %s", diff.SitesToAdd[0])
	}
}

// TestSyncDiff_SiteAppsUpdate verifies diff detects when apps need to be
// added to or removed from an existing site.
func TestSyncDiff_SiteAppsUpdate(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"

[[sites]]
name = "test.localhost"
apps = ["frappe", "erpnext"]
`).
		Build()

	benchConfig, err := config.ParseWegToml(bench)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	// State has the site but only with frappe (erpnext not yet installed on site)
	st := state.NewState()
	st.Apps["frappe"] = state.AppState{Name: "frappe", Branch: "version-15"}
	st.Apps["erpnext"] = state.AppState{Name: "erpnext", Branch: "version-15"}
	st.Sites["test.localhost"] = state.SiteState{
		Name: "test.localhost",
		Apps: []string{"frappe"},
	}
	st.Frappe.Version = "15"
	st.Frappe.Database = "mariadb"
	st.Services.WebPort = 8000
	st.Services.SocketPort = 9000
	st.Services.Workers = map[string]int{"all": 1}

	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, bench)

	if len(diff.SitesToUpdate) != 1 {
		t.Fatalf("expected 1 site to update, got %d", len(diff.SitesToUpdate))
	}

	siteUpdate := diff.SitesToUpdate[0]
	if siteUpdate.Name != "test.localhost" {
		t.Errorf("expected test.localhost, got %s", siteUpdate.Name)
	}
	if len(siteUpdate.AppsToAdd) != 1 || siteUpdate.AppsToAdd[0] != "erpnext" {
		t.Errorf("expected [erpnext] to add to site, got %v", siteUpdate.AppsToAdd)
	}
}

// TestSyncDiff_ExcludedApp verifies that excluded apps are not included
// in the diff.
func TestSyncDiff_ExcludedApp(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"
database = "mariadb"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"
excluded = true
`).
		Build()

	benchConfig, err := config.ParseWegToml(bench)
	if err != nil {
		t.Fatalf("ParseWegToml failed: %v", err)
	}

	st := state.NewState()
	diff := state.ComputeDiffFromBenchConfig(benchConfig, st, bench)

	// Only frappe should be added (erpnext is excluded)
	if len(diff.AppsToAdd) != 1 {
		t.Errorf("expected 1 app to add, got %d: %v", len(diff.AppsToAdd), diff.AppsToAdd)
	}
	if len(diff.AppsToAdd) > 0 && diff.AppsToAdd[0] != "frappe" {
		t.Errorf("expected frappe, got %s", diff.AppsToAdd[0])
	}
}
