package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	s := NewState()

	if s.Version != StateVersion {
		t.Errorf("Version = %v, want %v", s.Version, StateVersion)
	}
	if s.Apps == nil {
		t.Error("Apps map should be initialized")
	}
	if s.Sites == nil {
		t.Error("Sites map should be initialized")
	}
	if len(s.Apps) != 0 {
		t.Error("Apps should be empty")
	}
	if len(s.Sites) != 0 {
		t.Error("Sites should be empty")
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	// Load from non-existent path should return new state
	s, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if s.Version != StateVersion {
		t.Errorf("Version = %v, want %v", s.Version, StateVersion)
	}
	if len(s.Apps) != 0 {
		t.Error("Apps should be empty for new state")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and save state
	original := NewState()
	original.Frappe = FrappeState{Version: "15", Database: "mariadb"}
	original.AddApp(AppState{
		Name:   "frappe",
		URL:    "https://github.com/frappe/frappe",
		Branch: "version-15",
		Commit: "abc123",
	})
	original.AddSite(SiteState{
		Name:        "test.localhost",
		Apps:        []string{"frappe"},
		DefaultSite: true,
	})
	original.ConfigHash = "somehash"

	if err := original.Save(tmpDir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	statePath := filepath.Join(tmpDir, ".weg", StateFileName)
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("State file not created: %v", err)
	}

	// Load and verify
	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version mismatch: got %v, want %v", loaded.Version, original.Version)
	}
	if loaded.ConfigHash != original.ConfigHash {
		t.Errorf("ConfigHash mismatch: got %v, want %v", loaded.ConfigHash, original.ConfigHash)
	}
	if loaded.Frappe.Version != "15" {
		t.Errorf("Frappe.Version mismatch: got %v, want 15", loaded.Frappe.Version)
	}
	if loaded.Frappe.Database != "mariadb" {
		t.Errorf("Frappe.Database mismatch: got %v, want mariadb", loaded.Frappe.Database)
	}

	// Check app
	app, ok := loaded.Apps["frappe"]
	if !ok {
		t.Fatal("frappe app not found in loaded state")
	}
	if app.URL != "https://github.com/frappe/frappe" {
		t.Errorf("App URL mismatch: got %v", app.URL)
	}
	if app.Branch != "version-15" {
		t.Errorf("App Branch mismatch: got %v", app.Branch)
	}

	// Check site
	site, ok := loaded.Sites["test.localhost"]
	if !ok {
		t.Fatal("test.localhost site not found in loaded state")
	}
	if !site.DefaultSite {
		t.Error("Site should be default")
	}
	if len(site.Apps) != 1 || site.Apps[0] != "frappe" {
		t.Errorf("Site apps mismatch: got %v", site.Apps)
	}
}

func TestAppOperations(t *testing.T) {
	s := NewState()

	// Add app
	app := AppState{
		Name:   "erpnext",
		URL:    "https://github.com/frappe/erpnext",
		Branch: "version-15",
	}
	s.AddApp(app)

	if !s.HasApp("erpnext") {
		t.Error("HasApp should return true for added app")
	}
	if s.HasApp("nonexistent") {
		t.Error("HasApp should return false for non-existent app")
	}

	// Verify InstalledAt is set
	stored := s.Apps["erpnext"]
	if stored.InstalledAt.IsZero() {
		t.Error("InstalledAt should be set automatically")
	}

	// Remove app
	s.RemoveApp("erpnext")
	if s.HasApp("erpnext") {
		t.Error("HasApp should return false after removal")
	}
}

func TestSiteOperations(t *testing.T) {
	s := NewState()

	// Add site
	site := SiteState{
		Name: "mysite.localhost",
		Apps: []string{"frappe", "erpnext"},
	}
	s.AddSite(site)

	if !s.HasSite("mysite.localhost") {
		t.Error("HasSite should return true for added site")
	}
	if s.HasSite("nonexistent") {
		t.Error("HasSite should return false for non-existent site")
	}

	// Verify CreatedAt is set
	stored := s.Sites["mysite.localhost"]
	if stored.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set automatically")
	}

	// Remove site
	s.RemoveSite("mysite.localhost")
	if s.HasSite("mysite.localhost") {
		t.Error("HasSite should return false after removal")
	}
}

func TestDefaultSite(t *testing.T) {
	s := NewState()

	// No sites - should return empty
	if got := s.GetDefaultSite(); got != "" {
		t.Errorf("GetDefaultSite should return empty for no sites, got %v", got)
	}

	// Add sites
	s.AddSite(SiteState{Name: "site1.localhost", Apps: []string{"frappe"}})
	s.AddSite(SiteState{Name: "site2.localhost", Apps: []string{"frappe"}})

	// No default set - should return first site (order may vary)
	defaultSite := s.GetDefaultSite()
	if defaultSite != "site1.localhost" && defaultSite != "site2.localhost" {
		t.Errorf("GetDefaultSite should return one of the sites, got %v", defaultSite)
	}

	// Set default
	s.SetDefaultSite("site2.localhost")
	if got := s.GetDefaultSite(); got != "site2.localhost" {
		t.Errorf("GetDefaultSite = %v, want site2.localhost", got)
	}

	// Verify only one default
	defaultCount := 0
	for _, site := range s.Sites {
		if site.DefaultSite {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Errorf("Should have exactly 1 default site, got %v", defaultCount)
	}
}

func TestIsEmpty(t *testing.T) {
	s := NewState()

	if !s.IsEmpty() {
		t.Error("New state should be empty")
	}

	s.AddApp(AppState{Name: "frappe"})
	if s.IsEmpty() {
		t.Error("State with app should not be empty")
	}

	s.RemoveApp("frappe")
	s.AddSite(SiteState{Name: "test.localhost"})
	if s.IsEmpty() {
		t.Error("State with site should not be empty")
	}
}

func TestAppNames(t *testing.T) {
	s := NewState()
	s.AddApp(AppState{Name: "frappe"})
	s.AddApp(AppState{Name: "erpnext"})
	s.AddApp(AppState{Name: "hrms"})

	names := s.AppNames()
	if len(names) != 3 {
		t.Errorf("Expected 3 app names, got %v", len(names))
	}

	// Check all names are present (order may vary)
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"frappe", "erpnext", "hrms"} {
		if !nameSet[expected] {
			t.Errorf("Missing app name: %v", expected)
		}
	}
}

func TestSiteNames(t *testing.T) {
	s := NewState()
	s.AddSite(SiteState{Name: "site1.localhost"})
	s.AddSite(SiteState{Name: "site2.localhost"})

	names := s.SiteNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 site names, got %v", len(names))
	}
}

func TestComputeConfigHash(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.toml")

	content := []byte("[frappe]\nversion = \"15\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	hash1, err := ComputeConfigHash(configPath)
	if err != nil {
		t.Fatalf("ComputeConfigHash failed: %v", err)
	}
	if hash1 == "" {
		t.Error("Hash should not be empty")
	}

	// Same content should produce same hash
	hash2, err := ComputeConfigHash(configPath)
	if err != nil {
		t.Fatalf("ComputeConfigHash failed: %v", err)
	}
	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}

	// Different content should produce different hash
	if err := os.WriteFile(configPath, []byte("[frappe]\nversion = \"16\"\n"), 0644); err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}
	hash3, err := ComputeConfigHash(configPath)
	if err != nil {
		t.Fatalf("ComputeConfigHash failed: %v", err)
	}
	if hash1 == hash3 {
		t.Error("Different content should produce different hash")
	}
}

func TestComputePyprojectHash(t *testing.T) {
	tmpDir := t.TempDir()

	// No pyproject.toml - should return empty
	hash := ComputePyprojectHash(tmpDir)
	if hash != "" {
		t.Error("Should return empty for missing pyproject.toml")
	}

	// With pyproject.toml
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte("[project]\nname = \"test\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write pyproject.toml: %v", err)
	}

	hash = ComputePyprojectHash(tmpDir)
	if hash == "" {
		t.Error("Should return hash for existing pyproject.toml")
	}
}

func TestNeedsSync(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := []byte("[test]\nvalue = 1\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	s := NewState()

	// No hash - should need sync
	needsSync, err := s.NeedsSync(configPath)
	if err != nil {
		t.Fatalf("NeedsSync failed: %v", err)
	}
	if !needsSync {
		t.Error("Should need sync when no hash is stored")
	}

	// Compute and store hash
	hash, _ := ComputeConfigHash(configPath)
	s.ConfigHash = hash

	// Same content - should not need sync
	needsSync, err = s.NeedsSync(configPath)
	if err != nil {
		t.Fatalf("NeedsSync failed: %v", err)
	}
	if needsSync {
		t.Error("Should not need sync when hash matches")
	}

	// Change content - should need sync
	if err := os.WriteFile(configPath, []byte("[test]\nvalue = 2\n"), 0644); err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}
	needsSync, err = s.NeedsSync(configPath)
	if err != nil {
		t.Fatalf("NeedsSync failed: %v", err)
	}
	if !needsSync {
		t.Error("Should need sync when config changed")
	}
}

func TestUpdateConfigHash(t *testing.T) {
	s := NewState()

	before := time.Now()
	s.UpdateConfigHash("newhash")
	after := time.Now()

	if s.ConfigHash != "newhash" {
		t.Errorf("ConfigHash = %v, want newhash", s.ConfigHash)
	}
	if s.LastSync.Before(before) || s.LastSync.After(after) {
		t.Error("LastSync should be set to current time")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	// No state file
	if Exists(tmpDir) {
		t.Error("Exists should return false when no state file")
	}

	// Create state file
	s := NewState()
	if err := s.Save(tmpDir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !Exists(tmpDir) {
		t.Error("Exists should return true after saving state")
	}
}

func TestLoadCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	wegDir := filepath.Join(tmpDir, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		t.Fatalf("Failed to create .weg dir: %v", err)
	}

	// Write invalid JSON
	statePath := filepath.Join(wegDir, StateFileName)
	if err := os.WriteFile(statePath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted state: %v", err)
	}

	_, err := Load(tmpDir)
	if err == nil {
		t.Error("Load should fail for corrupted state file")
	}
}

func TestLoadWithNilMaps(t *testing.T) {
	tmpDir := t.TempDir()
	wegDir := filepath.Join(tmpDir, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		t.Fatalf("Failed to create .weg dir: %v", err)
	}

	// Write valid JSON but with null maps
	statePath := filepath.Join(wegDir, StateFileName)
	stateJSON := `{"version": "1", "apps": null, "sites": null}`
	if err := os.WriteFile(statePath, []byte(stateJSON), 0644); err != nil {
		t.Fatalf("Failed to write state: %v", err)
	}

	s, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Maps should be initialized even if null in JSON
	if s.Apps == nil {
		t.Error("Apps should be initialized")
	}
	if s.Sites == nil {
		t.Error("Sites should be initialized")
	}
}
