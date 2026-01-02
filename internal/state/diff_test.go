package state

import (
	"testing"

	"github.com/gavindsouza/weg/internal/config"
)

func TestDiffIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		diff  Diff
		empty bool
	}{
		{
			name:  "empty diff",
			diff:  Diff{},
			empty: true,
		},
		{
			name:  "apps to add",
			diff:  Diff{AppsToAdd: []string{"erpnext"}},
			empty: false,
		},
		{
			name:  "apps to remove",
			diff:  Diff{AppsToRemove: []string{"erpnext"}},
			empty: false,
		},
		{
			name:  "apps to update",
			diff:  Diff{AppsToUpdate: []AppUpdate{{Name: "erpnext"}}},
			empty: false,
		},
		{
			name:  "sites to add",
			diff:  Diff{SitesToAdd: []string{"test.localhost"}},
			empty: false,
		},
		{
			name:  "sites to remove",
			diff:  Diff{SitesToRemove: []string{"test.localhost"}},
			empty: false,
		},
		{
			name:  "sites to update",
			diff:  Diff{SitesToUpdate: []SiteUpdate{{Name: "test.localhost"}}},
			empty: false,
		},
		{
			name:  "frappe changed",
			diff:  Diff{FrappeChanged: true},
			empty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.IsEmpty(); got != tt.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.empty)
			}
			if got := tt.diff.HasChanges(); got == tt.empty {
				t.Errorf("HasChanges() = %v, want %v", got, !tt.empty)
			}
		})
	}
}

func TestDiffTotalChanges(t *testing.T) {
	diff := Diff{
		AppsToAdd:     []string{"a", "b"},
		AppsToRemove:  []string{"c"},
		AppsToUpdate:  []AppUpdate{{Name: "d"}},
		SitesToAdd:    []string{"e"},
		SitesToRemove: []string{"f"},
		SitesToUpdate: []SiteUpdate{{Name: "g"}},
		FrappeChanged: true,
	}

	// 2 + 1 + 1 + 1 + 1 + 1 + 1 = 8
	if got := diff.TotalChanges(); got != 8 {
		t.Errorf("TotalChanges() = %v, want 8", got)
	}
}

func TestSortAppsToAdd(t *testing.T) {
	apps := []string{"erpnext", "hrms", "frappe", "payments"}
	sortAppsToAdd(apps)

	if apps[0] != "frappe" {
		t.Errorf("First app should be frappe, got %v", apps[0])
	}

	// Rest should be alphabetical
	for i := 1; i < len(apps)-1; i++ {
		if apps[i] > apps[i+1] {
			t.Errorf("Apps not sorted: %v > %v", apps[i], apps[i+1])
		}
	}
}

func TestComputeDiffFromBenchConfig(t *testing.T) {
	t.Run("apps to add", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Apps: map[string]config.AppSettings{
				"frappe":  {URL: "https://github.com/frappe/frappe"},
				"erpnext": {URL: "https://github.com/frappe/erpnext"},
			},
		}
		state := NewState()
		state.AddApp(AppState{Name: "frappe"})

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		if len(diff.AppsToAdd) != 1 || diff.AppsToAdd[0] != "erpnext" {
			t.Errorf("AppsToAdd = %v, want [erpnext]", diff.AppsToAdd)
		}
	})

	t.Run("apps to remove", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Apps: map[string]config.AppSettings{
				"frappe": {URL: "https://github.com/frappe/frappe"},
			},
		}
		state := NewState()
		state.AddApp(AppState{Name: "frappe"})
		state.AddApp(AppState{Name: "erpnext"})

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		if len(diff.AppsToRemove) != 1 || diff.AppsToRemove[0] != "erpnext" {
			t.Errorf("AppsToRemove = %v, want [erpnext]", diff.AppsToRemove)
		}
	})

	t.Run("apps to update - branch change", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Apps: map[string]config.AppSettings{
				"frappe": {URL: "https://github.com/frappe/frappe", Branch: "version-16"},
			},
		}
		state := NewState()
		state.AddApp(AppState{Name: "frappe", Branch: "version-15"})

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		if len(diff.AppsToUpdate) != 1 {
			t.Fatalf("AppsToUpdate length = %v, want 1", len(diff.AppsToUpdate))
		}
		if diff.AppsToUpdate[0].OldBranch != "version-15" {
			t.Errorf("OldBranch = %v, want version-15", diff.AppsToUpdate[0].OldBranch)
		}
		if diff.AppsToUpdate[0].NewBranch != "version-16" {
			t.Errorf("NewBranch = %v, want version-16", diff.AppsToUpdate[0].NewBranch)
		}
	})

	t.Run("excluded apps not added", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Apps: map[string]config.AppSettings{
				"frappe":  {URL: "https://github.com/frappe/frappe"},
				"erpnext": {URL: "https://github.com/frappe/erpnext", Excluded: true},
			},
		}
		state := NewState()

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		// erpnext should not be in AppsToAdd since it's excluded
		for _, app := range diff.AppsToAdd {
			if app == "erpnext" {
				t.Error("Excluded app should not be in AppsToAdd")
			}
		}
	})

	t.Run("sites to add", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Sites: []config.SiteConfig{
				{Name: "site1.localhost"},
				{Name: "site2.localhost"},
			},
		}
		state := NewState()
		state.AddSite(SiteState{Name: "site1.localhost"})

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		if len(diff.SitesToAdd) != 1 || diff.SitesToAdd[0] != "site2.localhost" {
			t.Errorf("SitesToAdd = %v, want [site2.localhost]", diff.SitesToAdd)
		}
	})

	t.Run("sites to remove", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Sites: []config.SiteConfig{
				{Name: "site1.localhost"},
			},
		}
		state := NewState()
		state.AddSite(SiteState{Name: "site1.localhost"})
		state.AddSite(SiteState{Name: "site2.localhost"})

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		if len(diff.SitesToRemove) != 1 || diff.SitesToRemove[0] != "site2.localhost" {
			t.Errorf("SitesToRemove = %v, want [site2.localhost]", diff.SitesToRemove)
		}
	})

	t.Run("frappe changed", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Frappe: config.FrappeSettings{Version: "16", Database: "mariadb"},
		}
		state := NewState()
		state.Frappe = FrappeState{Version: "15", Database: "mariadb"}

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		if !diff.FrappeChanged {
			t.Error("FrappeChanged should be true when version changes")
		}
	})

	t.Run("frappe not changed for new state", func(t *testing.T) {
		cfg := &config.BenchConfig{
			Frappe: config.FrappeSettings{Version: "16", Database: "mariadb"},
		}
		state := NewState() // Empty state, no Frappe.Version set

		diff := ComputeDiffFromBenchConfig(cfg, state, "")

		if diff.FrappeChanged {
			t.Error("FrappeChanged should be false for new state")
		}
	})
}

func TestComputeSiteUpdate(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		update := computeSiteUpdate("test", []string{"frappe", "erpnext"}, []string{"frappe", "erpnext"})
		if update != nil {
			t.Error("Should return nil when no changes needed")
		}
	})

	t.Run("apps to add", func(t *testing.T) {
		update := computeSiteUpdate("test", []string{"frappe", "erpnext"}, []string{"frappe"})
		if update == nil {
			t.Fatal("Should return update when apps need to be added")
		}
		if len(update.AppsToAdd) != 1 || update.AppsToAdd[0] != "erpnext" {
			t.Errorf("AppsToAdd = %v, want [erpnext]", update.AppsToAdd)
		}
	})

	t.Run("apps to remove", func(t *testing.T) {
		update := computeSiteUpdate("test", []string{"frappe"}, []string{"frappe", "erpnext"})
		if update == nil {
			t.Fatal("Should return update when apps need to be removed")
		}
		if len(update.AppsToRemove) != 1 || update.AppsToRemove[0] != "erpnext" {
			t.Errorf("AppsToRemove = %v, want [erpnext]", update.AppsToRemove)
		}
	})
}

func TestToModuleName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-app", "my_app"},
		{"myapp", "myapp"},
		{"my-custom-app", "my_custom_app"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := toModuleName(tt.input); got != tt.expected {
				t.Errorf("toModuleName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
