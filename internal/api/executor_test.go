package api

import (
	"encoding/json"
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
