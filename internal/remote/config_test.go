/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSiteConfig(t *testing.T) {
	config := NewSiteConfig("https://test.frappe.cloud", "test-site")

	if config.Site.URL != "https://test.frappe.cloud" {
		t.Errorf("expected URL https://test.frappe.cloud, got %s", config.Site.URL)
	}

	if config.Site.Name != "test-site" {
		t.Errorf("expected name test-site, got %s", config.Site.Name)
	}

	if config.Site.Auth.Method != "api_key" {
		t.Errorf("expected auth method api_key, got %s", config.Site.Auth.Method)
	}

	// Check default modules
	if _, exists := config.Modules["Custom"]; !exists {
		t.Error("expected Custom module to exist")
	}
	if _, exists := config.Modules["_"]; !exists {
		t.Error("expected _ (catch-all) module to exist")
	}

	// Check default entity sync settings
	if !config.Sync.Entities.DocType {
		t.Error("expected DocType sync to be enabled by default")
	}
	if !config.Sync.Entities.CustomField {
		t.Error("expected CustomField sync to be enabled by default")
	}
	if config.Sync.Entities.Workspace {
		t.Error("expected Workspace sync to be disabled by default")
	}
}

func TestSiteConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and save config
	config := NewSiteConfig("https://test.frappe.cloud", "test-site")
	config.Site.Frappe.Version = "15.0.0"
	config.Sync.LastSync = time.Now()

	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".weg", "site.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load and verify
	loaded, err := LoadSiteConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Site.URL != config.Site.URL {
		t.Errorf("URL mismatch: expected %s, got %s", config.Site.URL, loaded.Site.URL)
	}

	if loaded.Site.Frappe.Version != "15.0.0" {
		t.Errorf("Frappe version mismatch: expected 15.0.0, got %s", loaded.Site.Frappe.Version)
	}
}

func TestCredentialsSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create credentials
	creds := &Credentials{
		Auth: CredentialAuth{
			APIKey:    "test-api-key",
			APISecret: "test-api-secret",
		},
	}

	if err := creds.Save(tmpDir); err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	// Verify file exists with restricted permissions
	credPath := filepath.Join(tmpDir, ".weg", "credentials.toml")
	info, err := os.Stat(credPath)
	if os.IsNotExist(err) {
		t.Fatal("credentials file was not created")
	}

	// Check permissions (should be 0600)
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}

	// Load and verify
	loaded, err := LoadCredentialsForSite(tmpDir, "")
	if err != nil {
		t.Fatalf("failed to load credentials: %v", err)
	}

	if loaded.Auth.APIKey != "test-api-key" {
		t.Errorf("API key mismatch: expected test-api-key, got %s", loaded.Auth.APIKey)
	}

	if loaded.Auth.APISecret != "test-api-secret" {
		t.Errorf("API secret mismatch: expected test-api-secret, got %s", loaded.Auth.APISecret)
	}
}

func TestCredentialsFromEnv(t *testing.T) {
	tmpDir := t.TempDir()

	// Set environment variables
	os.Setenv("WEG_API_KEY", "env-api-key")
	os.Setenv("WEG_API_SECRET", "env-api-secret")
	defer func() {
		os.Unsetenv("WEG_API_KEY")
		os.Unsetenv("WEG_API_SECRET")
	}()

	// Load credentials - should use env vars
	creds, err := LoadCredentialsForSite(tmpDir, "")
	if err != nil {
		t.Fatalf("failed to load credentials from env: %v", err)
	}

	if creds.Auth.APIKey != "env-api-key" {
		t.Errorf("expected env-api-key, got %s", creds.Auth.APIKey)
	}

	if creds.Auth.APISecret != "env-api-secret" {
		t.Errorf("expected env-api-secret, got %s", creds.Auth.APISecret)
	}
}

func TestCredentialsPriority(t *testing.T) {
	tmpDir := t.TempDir()

	// Create local credentials
	localCreds := &Credentials{
		Auth: CredentialAuth{
			APIKey:    "local-key",
			APISecret: "local-secret",
		},
	}
	if err := localCreds.Save(tmpDir); err != nil {
		t.Fatalf("failed to save local credentials: %v", err)
	}

	// Set environment variables (higher priority)
	os.Setenv("WEG_API_KEY", "env-key")
	os.Setenv("WEG_API_SECRET", "env-secret")
	defer func() {
		os.Unsetenv("WEG_API_KEY")
		os.Unsetenv("WEG_API_SECRET")
	}()

	// Load credentials - env should take priority
	creds, err := LoadCredentialsForSite(tmpDir, "")
	if err != nil {
		t.Fatalf("failed to load credentials: %v", err)
	}

	if creds.Auth.APIKey != "env-key" {
		t.Errorf("expected env vars to take priority, got %s", creds.Auth.APIKey)
	}
}

func TestGlobalCredentials(t *testing.T) {
	// Save original XDG_CONFIG_HOME and restore after test
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	// Clear env vars to test global credentials
	os.Unsetenv("WEG_API_KEY")
	os.Unsetenv("WEG_API_SECRET")

	// Save global credentials for a site
	siteHost := "mysite.frappe.cloud"
	auth := &CredentialAuth{
		APIKey:    "global-key",
		APISecret: "global-secret",
	}

	if err := SaveGlobalCredentials(siteHost, auth); err != nil {
		t.Fatalf("failed to save global credentials: %v", err)
	}

	// Verify they exist
	if !HasGlobalCredentials(siteHost) {
		t.Error("expected global credentials to exist")
	}

	// Load and verify
	creds, err := LoadCredentialsForSite("", siteHost)
	if err != nil {
		t.Fatalf("failed to load global credentials: %v", err)
	}

	if creds.Auth.APIKey != "global-key" {
		t.Errorf("expected global-key, got %s", creds.Auth.APIKey)
	}

	// Remove credentials
	if err := RemoveGlobalCredentials(siteHost); err != nil {
		t.Fatalf("failed to remove global credentials: %v", err)
	}

	if HasGlobalCredentials(siteHost) {
		t.Error("expected global credentials to be removed")
	}
}

func TestIsRemoteSite(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially should not be a remote site
	if IsRemoteSite(tmpDir) {
		t.Error("empty directory should not be a remote site")
	}

	// Create config
	config := NewSiteConfig("https://test.frappe.cloud", "test")
	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Now should be a remote site
	if !IsRemoteSite(tmpDir) {
		t.Error("directory with .weg/site.toml should be a remote site")
	}
}

func TestEnsureGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .weg directory
	wegDir := filepath.Join(tmpDir, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		t.Fatalf("failed to create .weg directory: %v", err)
	}

	if err := EnsureGitignore(tmpDir); err != nil {
		t.Fatalf("failed to ensure gitignore: %v", err)
	}

	// Verify .gitignore exists and contains credentials.toml
	gitignorePath := filepath.Join(wegDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	if string(content) == "" {
		t.Error("gitignore should not be empty")
	}

	// Check it contains credentials.toml
	if !contains(string(content), "credentials.toml") {
		t.Error("gitignore should contain credentials.toml")
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://mysite.frappe.cloud", "mysite.frappe.cloud"},
		{"http://localhost:8000", "localhost"},
		{"https://test.example.com/path", "test.example.com"},
		{"mysite.frappe.cloud", "mysite.frappe.cloud"},
	}

	for _, tt := range tests {
		result := ExtractHost(tt.url)
		if result != tt.expected {
			t.Errorf("ExtractHost(%s) = %s, expected %s", tt.url, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
