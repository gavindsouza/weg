package cloud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	// Test without API key
	c := NewClient("")
	if c.BaseURL != DefaultCloudAPI {
		t.Errorf("expected BaseURL %s, got %s", DefaultCloudAPI, c.BaseURL)
	}
	if c.Token != nil {
		t.Error("expected nil token without API key")
	}
	if c.HTTPClient == nil {
		t.Error("expected HTTPClient to be set")
	}

	// Test with API key
	c = NewClient("test-api-key")
	if c.Token == nil {
		t.Error("expected token to be set with API key")
	}
	if c.Token.AccessToken != "test-api-key" {
		t.Errorf("expected AccessToken 'test-api-key', got %s", c.Token.AccessToken)
	}
}

func TestTokenStruct(t *testing.T) {
	token := Token{
		AccessToken:  "access123",
		RefreshToken: "refresh456",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		Team:         "team789",
	}

	if token.AccessToken != "access123" {
		t.Errorf("unexpected AccessToken: %s", token.AccessToken)
	}
	if token.Team != "team789" {
		t.Errorf("unexpected Team: %s", token.Team)
	}
}

func TestSaveAndLoadCredentials(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp dir so ConfigPaths() uses it for local paths
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create credentials struct
	creds := &CloudCredentials{
		Clouds: map[string]*CloudAuth{
			"test": {Token: "test-api-key-123"},
		},
	}

	// Save credentials (local)
	err := SaveCredentials(creds, false)
	if err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	// Verify file exists
	tokenPath := filepath.Join(tmpDir, ".weg", "credentials.toml")
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Error("credentials file not created")
	}

	// Load credentials
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("failed to load credentials: %v", err)
	}

	if loaded.Clouds["test"] == nil || loaded.Clouds["test"].Token != "test-api-key-123" {
		t.Errorf("expected token 'test-api-key-123', got %+v", loaded.Clouds["test"])
	}
}

func TestLoadCredentialsNotExist(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp dir so ConfigPaths() uses it for local paths
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// LoadCredentials returns empty struct when no files exist, not an error
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(creds.Clouds) != 0 {
		t.Errorf("expected empty clouds map, got %d entries", len(creds.Clouds))
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		n        int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a very long string", 10, "this is..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "..."},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.n)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, result, tt.expected)
		}
	}
}

func TestFormatSiteListEmpty(t *testing.T) {
	result := FormatSiteList(nil)
	if result != "No sites found." {
		t.Errorf("expected 'No sites found.', got %s", result)
	}

	result = FormatSiteList([]Site{})
	if result != "No sites found." {
		t.Errorf("expected 'No sites found.', got %s", result)
	}
}

func TestFormatSiteList(t *testing.T) {
	sites := []Site{
		{Name: "test.frappe.cloud", Status: "Active", Plan: "Basic", Region: "us-east"},
		{Name: "prod.frappe.cloud", Status: "Active", Plan: "Pro", Region: "eu-west"},
	}

	result := FormatSiteList(sites)

	// Check header
	if !strings.Contains(result, "NAME") {
		t.Error("expected header with NAME")
	}
	if !strings.Contains(result, "STATUS") {
		t.Error("expected header with STATUS")
	}

	// Check sites are listed
	if !strings.Contains(result, "test.frappe.cloud") {
		t.Error("expected test.frappe.cloud in output")
	}
	if !strings.Contains(result, "prod.frappe.cloud") {
		t.Error("expected prod.frappe.cloud in output")
	}
}

func TestFormatBenchListEmpty(t *testing.T) {
	result := FormatBenchList(nil)
	if result != "No benches found." {
		t.Errorf("expected 'No benches found.', got %s", result)
	}
}

func TestFormatBenchList(t *testing.T) {
	benches := []Bench{
		{Name: "bench1", Title: "Production Bench", Version: "v15", Status: "Active", Apps: []string{"frappe", "erpnext"}},
	}

	result := FormatBenchList(benches)

	if !strings.Contains(result, "Production Bench") {
		t.Error("expected 'Production Bench' in output")
	}
	if !strings.Contains(result, "v15") {
		t.Error("expected 'v15' in output")
	}
}

func TestSiteStruct(t *testing.T) {
	site := Site{
		Name:      "mysite.frappe.cloud",
		Status:    "Active",
		Plan:      "Pro",
		Region:    "us-east",
		Apps:      []string{"frappe", "erpnext", "hrms"},
		CreatedAt: "2024-01-15",
	}

	if site.Name != "mysite.frappe.cloud" {
		t.Errorf("unexpected Name: %s", site.Name)
	}
	if len(site.Apps) != 3 {
		t.Errorf("expected 3 apps, got %d", len(site.Apps))
	}
}

func TestBenchStruct(t *testing.T) {
	bench := Bench{
		Name:    "bench-abc123",
		Title:   "My Bench",
		Version: "v15.42.0",
		Apps:    []string{"frappe"},
		Status:  "Active",
	}

	if bench.Version != "v15.42.0" {
		t.Errorf("unexpected Version: %s", bench.Version)
	}
}

func TestDeploymentStruct(t *testing.T) {
	deploy := Deployment{
		ID:         "deploy-123",
		Site:       "test.frappe.cloud",
		Status:     "Success",
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "2024-01-15 10:05:00",
		Duration:   "5m",
		Error:      "",
	}

	if deploy.Status != "Success" {
		t.Errorf("unexpected Status: %s", deploy.Status)
	}
}

func TestDeployOptionsStruct(t *testing.T) {
	opts := DeployOptions{
		SiteName: "test.frappe.cloud",
		Apps:     []string{"myapp"},
		Branch:   "main",
		Message:  "Deploying new feature",
	}

	if opts.SiteName != "test.frappe.cloud" {
		t.Errorf("unexpected SiteName: %s", opts.SiteName)
	}
	if opts.Message != "Deploying new feature" {
		t.Errorf("unexpected Message: %s", opts.Message)
	}
}

func TestBenchInfoStruct(t *testing.T) {
	info := BenchInfo{
		Name:          "bench-xyz",
		Status:        "Active",
		FrappeVersion: "15.42.0",
		AppCount:      5,
		SiteCount:     3,
	}

	if info.AppCount != 5 {
		t.Errorf("expected AppCount 5, got %d", info.AppCount)
	}
	if info.SiteCount != 3 {
		t.Errorf("expected SiteCount 3, got %d", info.SiteCount)
	}
}

func TestClientDoRequestNoAuth(t *testing.T) {
	c := NewClient("")

	_, err := c.doRequest("GET", "/test", nil)
	if err == nil {
		t.Error("expected error when not authenticated")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("expected 'not authenticated' error, got: %v", err)
	}
}

func TestUserStruct(t *testing.T) {
	user := User{
		Email: "test@example.com",
		Name:  "Test User",
	}

	if user.Email != "test@example.com" {
		t.Errorf("unexpected Email: %s", user.Email)
	}
}

