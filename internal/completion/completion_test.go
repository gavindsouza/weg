package completion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommonDocTypes(t *testing.T) {
	// Verify the common doctypes list is populated
	if len(CommonDocTypes) == 0 {
		t.Error("CommonDocTypes should not be empty")
	}

	// Verify some expected doctypes are present
	expected := []string{"User", "DocType", "File", "Company"}
	for _, dt := range expected {
		found := false
		for _, common := range CommonDocTypes {
			if common == dt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q to be in CommonDocTypes", dt)
		}
	}
}

func TestCompleteDocTypes(t *testing.T) {
	cmd := &cobra.Command{}

	tests := []struct {
		name       string
		toComplete string
		wantMin    int // minimum expected matches
	}{
		{
			name:       "empty prefix",
			toComplete: "",
			wantMin:    len(CommonDocTypes),
		},
		{
			name:       "U prefix",
			toComplete: "U",
			wantMin:    1, // At least "User"
		},
		{
			name:       "user lowercase",
			toComplete: "user",
			wantMin:    1, // Should match "User" (case insensitive)
		},
		{
			name:       "no match",
			toComplete: "xyz123",
			wantMin:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, directive := CompleteDocTypes(cmd, []string{}, tt.toComplete)

			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("expected NoFileComp directive, got %v", directive)
			}

			if len(results) < tt.wantMin {
				t.Errorf("expected at least %d results, got %d", tt.wantMin, len(results))
			}
		})
	}
}

func TestCompleteNone(t *testing.T) {
	cmd := &cobra.Command{}

	results, directive := CompleteNone(cmd, []string{}, "anything")

	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCompleteDocTypesForArg(t *testing.T) {
	cmd := &cobra.Command{}

	// Get completion function for arg position 0
	fn := CompleteDocTypesForArg(0)

	// Should complete when at position 0
	results, _ := fn(cmd, []string{}, "U")
	if len(results) == 0 {
		t.Error("expected completions at position 0")
	}

	// Should not complete when at position 1
	results, directive := fn(cmd, []string{"User"}, "")
	if len(results) != 0 {
		t.Error("expected no completions at position 1")
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCompleteSiteNamesForArg(t *testing.T) {
	cmd := &cobra.Command{}

	// Get completion function for arg position 1
	fn := CompleteSiteNamesForArg(1)

	// Should not complete when at position 0
	results, directive := fn(cmd, []string{}, "")
	if len(results) != 0 {
		t.Error("expected no completions at position 0")
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCompleteAppNamesForArg(t *testing.T) {
	cmd := &cobra.Command{}

	// Get completion function for arg position 2
	fn := CompleteAppNamesForArg(2)

	// Should not complete when at position 0
	results, directive := fn(cmd, []string{}, "")
	if len(results) != 0 {
		t.Error("expected no completions at position 0")
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}

	// Should not complete when at position 1
	results, directive = fn(cmd, []string{"arg1"}, "")
	if len(results) != 0 {
		t.Error("expected no completions at position 1")
	}
}

// Helper to create a minimal weg bench directory structure
func setupTestBench(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create bench structure
	dirs := []string{
		"apps/frappe",
		"apps/erpnext",
		"sites/test.localhost",
		"sites/prod.localhost",
		"sites/assets",
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	// Create site_config.json files for sites
	siteConfig := `{"db_name": "test"}`
	for _, site := range []string{"test.localhost", "prod.localhost"} {
		configPath := filepath.Join(tmpDir, "sites", site, "site_config.json")
		if err := os.WriteFile(configPath, []byte(siteConfig), 0644); err != nil {
			t.Fatalf("failed to create site config: %v", err)
		}
	}

	// Create weg.toml to mark as weg bench (this is what DetectContext looks for)
	wegToml := `[frappe]
version = "15"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "weg.toml"), []byte(wegToml), 0644); err != nil {
		t.Fatalf("failed to create weg.toml: %v", err)
	}

	return tmpDir
}

func TestGetSiteNamesWithBench(t *testing.T) {
	benchPath := setupTestBench(t)

	// Change to bench directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(benchPath); err != nil {
		t.Fatalf("failed to change to bench dir: %v", err)
	}

	sites := GetSiteNames()

	// Should find both test sites
	if len(sites) != 2 {
		t.Errorf("expected 2 sites, got %d: %v", len(sites), sites)
	}

	// Verify expected sites are present
	foundTest := false
	foundProd := false
	for _, s := range sites {
		if s == "test.localhost" {
			foundTest = true
		}
		if s == "prod.localhost" {
			foundProd = true
		}
	}

	if !foundTest {
		t.Error("expected to find test.localhost")
	}
	if !foundProd {
		t.Error("expected to find prod.localhost")
	}
}

func TestGetAppNamesWithBench(t *testing.T) {
	benchPath := setupTestBench(t)

	// Change to bench directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(benchPath); err != nil {
		t.Fatalf("failed to change to bench dir: %v", err)
	}

	apps := GetAppNames()

	// Should find both apps
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d: %v", len(apps), apps)
	}

	// Verify expected apps are present
	foundFrappe := false
	foundErpnext := false
	for _, a := range apps {
		if a == "frappe" {
			foundFrappe = true
		}
		if a == "erpnext" {
			foundErpnext = true
		}
	}

	if !foundFrappe {
		t.Error("expected to find frappe")
	}
	if !foundErpnext {
		t.Error("expected to find erpnext")
	}
}

func TestCompleteSiteNamesWithBench(t *testing.T) {
	benchPath := setupTestBench(t)

	// Change to bench directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(benchPath); err != nil {
		t.Fatalf("failed to change to bench dir: %v", err)
	}

	cmd := &cobra.Command{}

	// Complete with "test" prefix
	results, directive := CompleteSiteNames(cmd, []string{}, "test")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for 'test' prefix, got %d: %v", len(results), results)
	}

	if len(results) > 0 && results[0] != "test.localhost" {
		t.Errorf("expected test.localhost, got %s", results[0])
	}

	// Complete with empty prefix
	results, _ = CompleteSiteNames(cmd, []string{}, "")
	if len(results) != 2 {
		t.Errorf("expected 2 results for empty prefix, got %d", len(results))
	}
}

func TestCompleteAppNamesWithBench(t *testing.T) {
	benchPath := setupTestBench(t)

	// Change to bench directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(benchPath); err != nil {
		t.Fatalf("failed to change to bench dir: %v", err)
	}

	cmd := &cobra.Command{}

	// Complete with "fra" prefix
	results, directive := CompleteAppNames(cmd, []string{}, "fra")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for 'fra' prefix, got %d: %v", len(results), results)
	}

	if len(results) > 0 && results[0] != "frappe" {
		t.Errorf("expected frappe, got %s", results[0])
	}
}

func TestGetBenchPathNotInBench(t *testing.T) {
	// When not in a weg-managed project, should return empty string
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	result := GetBenchPath()
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

func TestGetSiteNamesNotInBench(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	result := GetSiteNames()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestGetAppNamesNotInBench(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	result := GetAppNames()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestCompleteSiteNamesNotInBench(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	cmd := &cobra.Command{}
	results, directive := CompleteSiteNames(cmd, []string{}, "")

	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCompleteAppNamesNotInBench(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	cmd := &cobra.Command{}
	results, directive := CompleteAppNames(cmd, []string{}, "")

	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCompleteInstalledAppNamesNotInBench(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	cmd := &cobra.Command{}
	results, directive := CompleteInstalledAppNames(cmd, []string{}, "")

	// Should fall back to GetAppNames which returns nil
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestSiteNamesSkipsHiddenAndAssets(t *testing.T) {
	benchPath := setupTestBench(t)

	// Add hidden directory and other non-site directories
	hiddenDir := filepath.Join(benchPath, "sites", ".hidden")
	os.MkdirAll(hiddenDir, 0755)

	// Change to bench directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(benchPath); err != nil {
		t.Fatalf("failed to change to bench dir: %v", err)
	}

	sites := GetSiteNames()

	// Should only have the two valid sites
	for _, s := range sites {
		if s == ".hidden" || s == "assets" {
			t.Errorf("should not include %s in site list", s)
		}
	}
}

func TestAppNamesSkipsHidden(t *testing.T) {
	benchPath := setupTestBench(t)

	// Add hidden app directory
	hiddenApp := filepath.Join(benchPath, "apps", ".hidden_app")
	os.MkdirAll(hiddenApp, 0755)

	// Change to bench directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(benchPath); err != nil {
		t.Fatalf("failed to change to bench dir: %v", err)
	}

	apps := GetAppNames()

	// Should not include hidden app
	for _, a := range apps {
		if a == ".hidden_app" {
			t.Error("should not include .hidden_app in app list")
		}
	}
}
