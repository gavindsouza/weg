/*
Copyright © 2025 Gavin <me@gavv.in>

Integration tests for weg CLI workflows.
These tests verify that multiple packages work correctly together.
*/
package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/gavindsouza/weg/internal/workspace"
)

// Note: state package has State, AppState, SiteState types
// config package has DetectContext, Context types

// TestRemoteSiteSetupWorkflow tests the complete workflow of setting up a remote site clone
func TestRemoteSiteSetupWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Step 1: Create site config
	siteConfig := remote.NewSiteConfig("https://test.frappe.cloud", "test-project")
	siteConfig.Site.Frappe.Version = "15.0.0"

	if err := siteConfig.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save site config: %v", err)
	}

	// Step 2: Verify directory is recognized as remote site
	if !remote.IsRemoteSite(tmpDir) {
		t.Error("Directory should be recognized as remote site after config is saved")
	}

	// Step 3: Save credentials
	creds := &remote.Credentials{
		Auth: remote.CredentialAuth{
			APIKey:    "test-key-12345",
			APISecret: "test-secret-67890",
		},
	}
	if err := creds.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Step 4: Load and verify everything works together
	loadedConfig, err := remote.LoadSiteConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load site config: %v", err)
	}

	loadedCreds, err := remote.LoadCredentials(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	// Verify config values
	if loadedConfig.Site.URL != "https://test.frappe.cloud" {
		t.Errorf("Config URL mismatch: got %s", loadedConfig.Site.URL)
	}
	if loadedConfig.Site.Frappe.Version != "15.0.0" {
		t.Errorf("Frappe version mismatch: got %s", loadedConfig.Site.Frappe.Version)
	}

	// Verify credentials
	if loadedCreds.Auth.APIKey != "test-key-12345" {
		t.Errorf("Credentials API key mismatch: got %s", loadedCreds.Auth.APIKey)
	}

	// Step 5: Ensure gitignore is created and protects credentials
	if err := remote.EnsureGitignore(tmpDir); err != nil {
		t.Fatalf("Failed to ensure gitignore: %v", err)
	}

	gitignorePath := filepath.Join(tmpDir, ".weg", ".gitignore")
	gitignoreContent, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("Failed to read gitignore: %v", err)
	}
	if !strings.Contains(string(gitignoreContent), "credentials.toml") {
		t.Error("Gitignore should protect credentials.toml")
	}
}

// TestOutputFormatConsistency tests that output formatting works correctly across different formats
func TestOutputFormatConsistency(t *testing.T) {
	output.SaveForTest(t)

	type TestItem struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Count  int    `json:"count"`
	}

	items := []TestItem{
		{Name: "item1", Status: "active", Count: 10},
		{Name: "item2", Status: "pending", Count: 5},
	}

	// Test JSON format
	t.Run("JSON format", func(t *testing.T) {
		var buf bytes.Buffer
		output.Writer = &buf
		output.CurrentFormat = output.FormatJSON
		output.Level = output.VerbosityNormal

		output.List(items)

		var result []TestItem
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("JSON output should be valid JSON: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("Expected 2 items, got %d", len(result))
		}
	})

	// Test Table format
	t.Run("Table format", func(t *testing.T) {
		var buf bytes.Buffer
		output.Writer = &buf
		output.CurrentFormat = output.FormatTable
		output.Level = output.VerbosityNormal

		output.List(items)

		tableOutput := buf.String()
		// Table format should contain headers and data
		if !strings.Contains(tableOutput, "NAME") {
			t.Error("Table output should contain NAME header")
		}
		if !strings.Contains(tableOutput, "item1") {
			t.Error("Table output should contain item1")
		}
	})

	// Test Plain format
	t.Run("Plain format", func(t *testing.T) {
		var buf bytes.Buffer
		output.Writer = &buf
		output.CurrentFormat = output.FormatPlain
		output.Level = output.VerbosityNormal

		output.List(items)

		plainOutput := buf.String()
		// Plain format should list items simply
		if !strings.Contains(plainOutput, "item1") {
			t.Error("Plain output should contain item1")
		}
	})

	// Test Quiet format
	t.Run("Quiet format", func(t *testing.T) {
		var buf bytes.Buffer
		output.Writer = &buf
		output.CurrentFormat = output.FormatQuiet
		output.Level = output.VerbosityNormal

		output.List(items)

		quietOutput := buf.String()
		lines := strings.Split(strings.TrimSpace(quietOutput), "\n")
		if len(lines) != 2 {
			t.Errorf("Quiet format should output 2 lines (names only), got %d", len(lines))
		}
	})
}

// TestWorkspaceExpandCollapseWorkflow tests the workspace expansion and state tracking workflow
func TestWorkspaceExpandCollapseWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Step 1: Create initial workspace state
	wsState := &workspace.WorkspaceState{
		Files: make(map[string]workspace.FileState),
	}

	// Step 2: Add a tracked file
	workspacePath := filepath.Join(workspace.WorkspaceDir, "server_scripts", "test_script.py")
	wsState.Files[workspacePath] = workspace.FileState{
		Source:         "entities/Server Script/test_script.json",
		Field:          "script",
		ExpandedAt:     time.Now(),
		SourceMtime:    time.Now(),
		WorkspaceMtime: time.Now(),
	}

	// Step 3: Save state
	if err := wsState.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save workspace state: %v", err)
	}

	// Step 4: Load state and verify
	loadedState, err := workspace.LoadState(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load workspace state: %v", err)
	}

	if len(loadedState.Files) != 1 {
		t.Errorf("Expected 1 file in state, got %d", len(loadedState.Files))
	}

	fileState, exists := loadedState.Files[workspacePath]
	if !exists {
		t.Error("Expected file state to exist")
	}
	if fileState.Field != "script" {
		t.Errorf("Expected field 'script', got %s", fileState.Field)
	}
}

// TestStateTrackingWorkflow tests the state tracking functionality
func TestStateTrackingWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create state directory
	stateDir := filepath.Join(tmpDir, ".weg")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state directory: %v", err)
	}

	// Step 1: Load state (should return new empty state for non-existent file)
	initialState, err := state.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load initial state: %v", err)
	}

	if initialState == nil {
		t.Fatal("Expected non-nil state")
	}

	// Step 2: Add apps and sites to state
	initialState.Apps["frappe"] = state.AppState{
		Name:        "frappe",
		URL:         "https://github.com/frappe/frappe",
		Branch:      "version-15",
		InstalledAt: time.Now(),
	}
	initialState.Apps["erpnext"] = state.AppState{
		Name:        "erpnext",
		URL:         "https://github.com/frappe/erpnext",
		Branch:      "version-15",
		InstalledAt: time.Now(),
	}
	initialState.Sites["test.localhost"] = state.SiteState{
		Name:      "test.localhost",
		Apps:      []string{"frappe", "erpnext"},
		CreatedAt: time.Now(),
	}

	// Step 3: Save state
	if err := initialState.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Step 4: Load and verify
	loaded, err := state.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if len(loaded.Apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(loaded.Apps))
	}
	if len(loaded.Sites) != 1 {
		t.Errorf("Expected 1 site, got %d", len(loaded.Sites))
	}

	// Verify app details
	frappeApp, exists := loaded.Apps["frappe"]
	if !exists {
		t.Error("Expected frappe app to exist")
	}
	if frappeApp.Branch != "version-15" {
		t.Errorf("Expected branch 'version-15', got %s", frappeApp.Branch)
	}

	// Verify site details
	site, exists := loaded.Sites["test.localhost"]
	if !exists {
		t.Error("Expected test.localhost site to exist")
	}
	if len(site.Apps) != 2 {
		t.Errorf("Expected 2 apps on site, got %d", len(site.Apps))
	}

	// Step 5: Test helper methods
	if !loaded.HasApp("frappe") {
		t.Error("HasApp should return true for frappe")
	}
	if loaded.HasApp("nonexistent") {
		t.Error("HasApp should return false for nonexistent app")
	}
	if !loaded.HasSite("test.localhost") {
		t.Error("HasSite should return true for test.localhost")
	}
}

// TestConfigDetectionWorkflow tests detecting project context from directory structure
func TestConfigDetectionWorkflow(t *testing.T) {
	// Test weg app detection (with [tool.weg] section in pyproject.toml)
	t.Run("weg app detection", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a pyproject.toml with [tool.weg.compatibility] section
		// HasWegSection checks for specific fields like Compatibility.Frappe
		pyproject := `[project]
name = "my-frappe-app"
version = "0.0.1"

[tool.weg]
[tool.weg.compatibility]
frappe = ["15"]
`
		if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
			t.Fatalf("Failed to create pyproject.toml: %v", err)
		}

		// Detect context
		result, err := config.DetectContext(tmpDir)
		if err != nil {
			t.Fatalf("DetectContext failed: %v", err)
		}
		if result.Context != config.ContextWegApp {
			t.Errorf("Expected ContextWegApp, got %v", result.Context)
		}
	})

	// Test bench detection
	t.Run("bench detection", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create bench structure
		os.MkdirAll(filepath.Join(tmpDir, "apps", "frappe"), 0755)
		os.MkdirAll(filepath.Join(tmpDir, "sites"), 0755)

		result, err := config.DetectContext(tmpDir)
		if err != nil {
			t.Fatalf("DetectContext failed: %v", err)
		}
		if result.Context != config.ContextBench {
			t.Errorf("Expected ContextBench, got %v", result.Context)
		}
	})

	// Test fresh directory (unknown project)
	t.Run("fresh directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := config.DetectContext(tmpDir)
		if err != nil {
			t.Fatalf("DetectContext failed: %v", err)
		}
		if result.Context != config.ContextFresh {
			t.Errorf("Expected ContextFresh, got %v", result.Context)
		}
	})

	// Test frappe app detection (has hooks.py)
	t.Run("frappe app detection", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create hooks.py (indicates a Frappe app)
		if err := os.WriteFile(filepath.Join(tmpDir, "hooks.py"), []byte("# Frappe hooks"), 0644); err != nil {
			t.Fatalf("Failed to create hooks.py: %v", err)
		}

		result, err := config.DetectContext(tmpDir)
		if err != nil {
			t.Fatalf("DetectContext failed: %v", err)
		}
		if result.Context != config.ContextApp {
			t.Errorf("Expected ContextApp, got %v", result.Context)
		}
	})

	// Test weg.toml detection
	t.Run("weg.toml detection", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create weg.toml
		wegToml := `[frappe]
version = "15"
`
		if err := os.WriteFile(filepath.Join(tmpDir, "weg.toml"), []byte(wegToml), 0644); err != nil {
			t.Fatalf("Failed to create weg.toml: %v", err)
		}

		result, err := config.DetectContext(tmpDir)
		if err != nil {
			t.Fatalf("DetectContext failed: %v", err)
		}
		if result.Context != config.ContextWegBench {
			t.Errorf("Expected ContextWegBench, got %v", result.Context)
		}
	})
}

// TestOutputRedactionIntegration tests that sensitive data is redacted in output
func TestOutputRedactionIntegration(t *testing.T) {
	// Test redaction of bearer tokens
	t.Run("bearer token redaction", func(t *testing.T) {
		input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"
		result := output.RedactString(input)
		if !strings.Contains(result, "***") {
			t.Error("Bearer token should be redacted")
		}
		if strings.Contains(result, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
			t.Error("Bearer token content should not be visible")
		}
	})

	// Test redaction of API key:secret format
	t.Run("api key:secret redaction", func(t *testing.T) {
		input := "Using credentials: abc1234567890123456:secret1234567890123"
		result := output.RedactString(input)
		if !strings.Contains(result, "***") {
			t.Error("API key:secret should be redacted")
		}
	})

	// Test map redaction
	t.Run("map redaction", func(t *testing.T) {
		data := map[string]any{
			"username": "admin",
			"password": "secretpassword123",
			"api_key":  "my-api-key-value",
			"url":      "https://example.com",
		}

		result := output.RedactMap(data)

		// Check that sensitive fields are redacted
		if pwd, ok := result["password"].(string); ok {
			if !strings.Contains(pwd, "***") {
				t.Error("Password should be redacted")
			}
		}
		if key, ok := result["api_key"].(string); ok {
			if !strings.Contains(key, "***") {
				t.Error("API key should be redacted")
			}
		}

		// Check that non-sensitive fields are preserved
		if result["username"] != "admin" {
			t.Error("Username should not be redacted")
		}
		if result["url"] != "https://example.com" {
			t.Error("URL should not be redacted")
		}
	})
}

// TestCodeFieldMapping tests that code field mappings work correctly
func TestCodeFieldMapping(t *testing.T) {
	tests := []struct {
		entityType string
		field      string
		wantExt    string
		wantLang   string
	}{
		{"server_script", "script", ".py", "python"},
		{"client_script", "script", ".js", "javascript"},
		{"report", "report_script", ".py", "python"},
		{"report", "javascript", ".js", "javascript"},
		{"report", "query", ".sql", "sql"},
		{"print_format", "html", ".html", "html"},
		{"print_format", "css", ".css", "css"},
	}

	for _, tt := range tests {
		t.Run(tt.entityType+"/"+tt.field, func(t *testing.T) {
			cf := workspace.GetCodeFieldForEntity(tt.entityType, tt.field)
			if cf == nil {
				t.Fatalf("Expected code field for %s/%s", tt.entityType, tt.field)
			}
			if cf.Extension != tt.wantExt {
				t.Errorf("Extension = %s, want %s", cf.Extension, tt.wantExt)
			}
			if cf.Language != tt.wantLang {
				t.Errorf("Language = %s, want %s", cf.Language, tt.wantLang)
			}
		})
	}
}

// TestOutputVerbosityLevels tests that verbosity levels work correctly together
func TestOutputVerbosityLevels(t *testing.T) {
	output.SaveForTest(t)
	output.CurrentFormat = output.FormatTable

	t.Run("quiet mode suppresses Print", func(t *testing.T) {
		var buf bytes.Buffer
		output.Writer = &buf
		output.Level = output.VerbosityQuiet

		output.Print("test message")

		if buf.String() != "" {
			t.Error("Print should be suppressed in quiet mode")
		}
	})

	t.Run("normal mode shows Print", func(t *testing.T) {
		var buf bytes.Buffer
		output.Writer = &buf
		output.Level = output.VerbosityNormal

		output.Print("test message")

		if !strings.Contains(buf.String(), "test message") {
			t.Error("Print should output in normal mode")
		}
	})

	t.Run("verbose only shows in verbose mode", func(t *testing.T) {
		var buf bytes.Buffer
		output.Writer = &buf
		output.Level = output.VerbosityNormal

		output.Verbose("verbose message")

		if buf.String() != "" {
			t.Error("Verbose should not output in normal mode")
		}

		buf.Reset()
		output.Level = output.VerbosityVerbose
		output.Verbose("verbose message")

		if !strings.Contains(buf.String(), "verbose message") {
			t.Error("Verbose should output in verbose mode")
		}
	})
}
