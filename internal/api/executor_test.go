package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name         string
		benchPath    string
		site         string
		user         string
		expectedUser string
	}{
		{
			name:         "default user",
			benchPath:    "/path/to/bench",
			site:         "test.localhost",
			user:         "",
			expectedUser: "Administrator",
		},
		{
			name:         "custom user",
			benchPath:    "/path/to/bench",
			site:         "test.localhost",
			user:         "Guest",
			expectedUser: "Guest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewExecutor(tt.benchPath, tt.site, tt.user)

			if exec.BenchPath != tt.benchPath {
				t.Errorf("BenchPath = %v, want %v", exec.BenchPath, tt.benchPath)
			}
			if exec.Site != tt.site {
				t.Errorf("Site = %v, want %v", exec.Site, tt.site)
			}
			if exec.User != tt.expectedUser {
				t.Errorf("User = %v, want %v", exec.User, tt.expectedUser)
			}
		})
	}
}

func TestResultParsing(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		wantErr  bool
		success  bool
		hasData  bool
		hasError bool
	}{
		{
			name:    "success with data",
			jsonStr: `{"success": true, "data": {"name": "test"}}`,
			success: true,
			hasData: true,
		},
		{
			name:     "failure with error",
			jsonStr:  `{"success": false, "error": "something went wrong"}`,
			success:  false,
			hasError: true,
		},
		{
			name:    "success with null data",
			jsonStr: `{"success": true, "data": null}`,
			success: true,
			hasData: false,
		},
		{
			name:    "invalid json",
			jsonStr: `not valid json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Result
			err := json.Unmarshal([]byte(tt.jsonStr), &result)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Success != tt.success {
				t.Errorf("Success = %v, want %v", result.Success, tt.success)
			}

			if tt.hasData && result.Data == nil {
				t.Error("expected Data to be set")
			}

			if tt.hasError && result.Error == "" {
				t.Error("expected Error to be set")
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name:    "simple map",
			data:    map[string]string{"key": "value"},
			wantErr: false,
		},
		{
			name:    "nested structure",
			data:    map[string]interface{}{"user": map[string]string{"name": "test"}},
			wantErr: false,
		},
		{
			name:    "slice",
			data:    []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name:    "nil",
			data:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatJSON(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == "" && tt.data != nil {
				t.Error("expected non-empty result")
			}
		})
	}
}

func TestResultFields(t *testing.T) {
	// Test that Result struct fields are correctly typed
	result := Result{
		Success: true,
		Data:    map[string]interface{}{"count": 42},
		Error:   "",
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Error("expected Data to be a map")
	}

	count, ok := data["count"]
	if !ok {
		t.Error("expected count field in data")
	}

	if count != 42 {
		t.Errorf("expected count to be 42, got %v", count)
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Test creating a new directory
	newDir := tmpDir + "/test/nested/dir"
	ensureDir(newDir)

	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, not file")
	}

	// Test that it doesn't fail on existing directory
	ensureDir(newDir) // Should not panic or error
}

func TestNewExecutorCreatesLogDirs(t *testing.T) {
	tmpDir := t.TempDir()
	site := "test.localhost"

	// Create the executor - it should create log directories
	_ = NewExecutor(tmpDir, site, "Administrator")

	// Check that logs directory was created
	logsDir := tmpDir + "/logs"
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		t.Error("logs directory should have been created")
	}

	// Check site logs directory
	siteLogsDir := tmpDir + "/sites/" + site + "/logs"
	if _, err := os.Stat(siteLogsDir); os.IsNotExist(err) {
		t.Error("site logs directory should have been created")
	}
}

func TestNewExecutorAppCentricMode(t *testing.T) {
	tmpDir := t.TempDir()
	// Simulate app-centric mode by having bench path end in .weg
	wegDir := tmpDir + "/project/.weg"
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		t.Fatalf("failed to create .weg dir: %v", err)
	}

	_ = NewExecutor(wegDir, "test.localhost", "")

	// Check parent logs directory was created
	parentLogs := tmpDir + "/project/logs"
	if _, err := os.Stat(parentLogs); os.IsNotExist(err) {
		t.Error("parent logs directory should have been created for app-centric mode")
	}
}

func TestFormatJSONOutput(t *testing.T) {
	data := map[string]interface{}{
		"name":  "Test",
		"count": 42,
	}

	result, err := FormatJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's properly indented
	if !strings.Contains(result, "  ") {
		t.Error("expected indented output")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}

	if parsed["name"] != "Test" {
		t.Errorf("expected name=Test, got %v", parsed["name"])
	}
}

func TestResultWithTraceback(t *testing.T) {
	result := Result{
		Success:   false,
		Error:     "Division by zero",
		Traceback: "File \"test.py\", line 1\n  1/0\nZeroDivisionError",
	}

	if result.Success {
		t.Error("expected failure")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
	if result.Traceback == "" {
		t.Error("expected traceback")
	}
}

func TestResultArrayData(t *testing.T) {
	jsonStr := `{"success": true, "data": [{"name": "doc1"}, {"name": "doc2"}]}`
	var result Result
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	data, ok := result.Data.([]interface{})
	if !ok {
		t.Error("expected data to be an array")
	}

	if len(data) != 2 {
		t.Errorf("expected 2 items, got %d", len(data))
	}
}

func TestExecutorVerboseFlag(t *testing.T) {
	exec := NewExecutor("/tmp/bench", "site.localhost", "Admin")
	exec.Verbose = true

	if !exec.Verbose {
		t.Error("expected Verbose to be true")
	}
}
