package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/gavindsouza/weg/cmd/app"
	"github.com/gavindsouza/weg/cmd/site"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/testutil"
)

// captureJSON redirects output to a buffer with JSON format and restores on cleanup.
func captureJSON(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := output.CaptureForTest(t)
	output.CurrentFormat = output.FormatJSON
	return buf
}

// --- Site list behavioral tests ---

func TestSiteList_EmptyBench(t *testing.T) {
	bench := testutil.NewBench(t).Build()
	buf := captureJSON(t)

	oldWd, _ := os.Getwd()
	os.Chdir(bench)
	defer os.Chdir(oldWd)

	// Reset the command for re-execution
	cmd := site.SiteCmd
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&bytes.Buffer{}) // cobra's own output (help text)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("site list failed: %v", err)
	}

	// Empty bench with no sites configured in weg.toml and no site dirs
	// Should print "No sites found" message (not JSON)
	out := buf.String()
	if out == "" {
		return // empty is OK for JSON format with no sites
	}
	// If there's output, it should mention "No sites found" or be valid JSON
	if out[0] == '[' {
		var sites []map[string]any
		if err := json.Unmarshal([]byte(out), &sites); err != nil {
			t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
		}
		if len(sites) != 0 {
			t.Errorf("expected 0 sites, got %d", len(sites))
		}
	}
}

func TestSiteList_WithSites(t *testing.T) {
	bench := testutil.NewBench(t).
		WithApp("frappe").
		WithSite("test.localhost").
		WithSite("prod.localhost").
		Build()

	buf := captureJSON(t)

	oldWd, _ := os.Getwd()
	os.Chdir(bench)
	defer os.Chdir(oldWd)

	cmd := site.SiteCmd
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("site list failed: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("expected output for bench with sites")
	}

	var sites []map[string]any
	if err := json.Unmarshal([]byte(out), &sites); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}

	if len(sites) < 2 {
		t.Errorf("expected at least 2 sites, got %d", len(sites))
	}

	// Verify site names are present
	names := make(map[string]bool)
	for _, s := range sites {
		if name, ok := s["name"].(string); ok {
			names[name] = true
		}
	}
	for _, expected := range []string{"test.localhost", "prod.localhost"} {
		if !names[expected] {
			t.Errorf("site %q not found in output", expected)
		}
	}
}

// --- App list behavioral tests ---

func TestAppList_BenchWithApps(t *testing.T) {
	bench := testutil.NewBench(t).
		WithWegToml(`[frappe]
version = "15"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-15"

[apps.erpnext]
url = "https://github.com/frappe/erpnext"
branch = "version-15"
`).
		WithApp("frappe").
		WithApp("erpnext").
		Build()

	buf := captureJSON(t)

	oldWd, _ := os.Getwd()
	os.Chdir(bench)
	defer os.Chdir(oldWd)

	cmd := app.AppCmd
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("app list failed: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("expected output for bench with apps")
	}

	var apps []map[string]any
	if err := json.Unmarshal([]byte(out), &apps); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}

	if len(apps) < 2 {
		t.Errorf("expected at least 2 apps, got %d", len(apps))
	}

	// Verify app fields
	names := make(map[string]bool)
	for _, a := range apps {
		if name, ok := a["name"].(string); ok {
			names[name] = true
		}
		// Every app should have a status field
		if _, ok := a["status"]; !ok {
			t.Error("app missing status field")
		}
	}
	for _, expected := range []string{"frappe", "erpnext"} {
		if !names[expected] {
			t.Errorf("app %q not found in output", expected)
		}
	}
}

func TestAppList_NotWegProject(t *testing.T) {
	tmpDir := t.TempDir() // empty dir, not a weg project

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cmd := app.AppCmd
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-weg project")
	}
}
