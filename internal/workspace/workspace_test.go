/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStatusString(t *testing.T) {
	tests := []struct {
		status FileStatus
		want   string
	}{
		{StatusSynced, "synced"},
		{StatusModified, "modified"},
		{StatusConflict, "conflict"},
		{StatusStale, "stale"},
		{StatusNew, "new"},
		{FileStatus(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("FileStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"File Name", "file_name"},
		{"path/to/file", "path_to_file"},
		{"path:value", "path_value"},
		{"path\\windows", "path_windows"},
		{"MixedCase", "mixedcase"},
		{"Multiple  Spaces", "multiple__spaces"},
	}

	for _, tt := range tests {
		got := SanitizeFileName(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetCodeFieldForEntity(t *testing.T) {
	tests := []struct {
		entityType string
		field      string
		wantNil    bool
		wantExt    string
	}{
		{"server_script", "script", false, ".py"},
		{"client_script", "script", false, ".js"},
		{"report", "report_script", false, ".py"},
		{"report", "javascript", false, ".js"},
		{"report", "query", false, ".sql"},
		{"print_format", "html", false, ".html"},
		{"print_format", "css", false, ".css"},
		{"unknown_type", "script", true, ""},
		{"server_script", "unknown_field", true, ""},
	}

	for _, tt := range tests {
		cf := GetCodeFieldForEntity(tt.entityType, tt.field)
		if tt.wantNil && cf != nil {
			t.Errorf("GetCodeFieldForEntity(%q, %q) = %v, want nil", tt.entityType, tt.field, cf)
		}
		if !tt.wantNil {
			if cf == nil {
				t.Errorf("GetCodeFieldForEntity(%q, %q) = nil, want non-nil", tt.entityType, tt.field)
			} else if cf.Extension != tt.wantExt {
				t.Errorf("GetCodeFieldForEntity(%q, %q).Extension = %q, want %q", tt.entityType, tt.field, cf.Extension, tt.wantExt)
			}
		}
	}
}

func TestGetCodeFieldsForEntity(t *testing.T) {
	tests := []struct {
		entityType string
		wantCount  int
	}{
		{"server_script", 1},
		{"client_script", 1},
		{"report", 3},       // report_script, javascript, query
		{"print_format", 2}, // html, css
		{"unknown_type", 0},
	}

	for _, tt := range tests {
		fields := GetCodeFieldsForEntity(tt.entityType)
		if len(fields) != tt.wantCount {
			t.Errorf("GetCodeFieldsForEntity(%q) returned %d fields, want %d", tt.entityType, len(fields), tt.wantCount)
		}
	}
}

func TestLoadState_NonExistent(t *testing.T) {
	// Create a temp directory without a state file
	tmpDir := t.TempDir()

	state, err := LoadState(tmpDir)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if state == nil {
		t.Fatal("LoadState() returned nil state")
	}
	if state.Files == nil {
		t.Error("LoadState() returned state with nil Files map")
	}
	if len(state.Files) != 0 {
		t.Errorf("LoadState() returned state with %d files, want 0", len(state.Files))
	}
}

func TestLoadState_Existing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .weg directory and state file
	wegDir := filepath.Join(tmpDir, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		t.Fatalf("Failed to create .weg dir: %v", err)
	}

	testState := &WorkspaceState{
		Files: map[string]FileState{
			"test/file.py": {
				Source:      "source.json",
				Field:       "script",
				ExpandedAt:  time.Now(),
				SourceMtime: time.Now(),
			},
		},
	}

	data, err := json.MarshalIndent(testState, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test state: %v", err)
	}

	statePath := filepath.Join(wegDir, "workspace_state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	// Load the state
	loaded, err := LoadState(tmpDir)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if len(loaded.Files) != 1 {
		t.Errorf("LoadState() returned %d files, want 1", len(loaded.Files))
	}
	if _, ok := loaded.Files["test/file.py"]; !ok {
		t.Error("LoadState() missing expected file entry")
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .weg directory with invalid JSON
	wegDir := filepath.Join(tmpDir, ".weg")
	if err := os.MkdirAll(wegDir, 0755); err != nil {
		t.Fatalf("Failed to create .weg dir: %v", err)
	}

	statePath := filepath.Join(wegDir, "workspace_state.json")
	if err := os.WriteFile(statePath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	_, err := LoadState(tmpDir)
	if err == nil {
		t.Error("LoadState() expected error for invalid JSON, got nil")
	}
}

func TestWorkspaceStateSave(t *testing.T) {
	tmpDir := t.TempDir()

	state := &WorkspaceState{
		Files: map[string]FileState{
			"test/script.py": {
				Source:      "entities/server_script.json",
				Field:       "script",
				ExpandedAt:  time.Now(),
				SourceMtime: time.Now(),
			},
		},
	}

	if err := state.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file was created
	statePath := filepath.Join(tmpDir, ".weg", "workspace_state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("Save() did not create state file")
	}

	// Verify content is valid JSON
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read saved state: %v", err)
	}

	var loaded WorkspaceState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Errorf("Save() created invalid JSON: %v", err)
	}
	if len(loaded.Files) != 1 {
		t.Errorf("Saved state has %d files, want 1", len(loaded.Files))
	}
}

func TestGetFileStatus_SourceDeleted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace file but no source
	workspaceDir := filepath.Join(tmpDir, WorkspaceDir, "server_scripts")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}
	workspacePath := filepath.Join(WorkspaceDir, "server_scripts", "test.py")
	fullPath := filepath.Join(tmpDir, workspacePath)
	if err := os.WriteFile(fullPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("Failed to create workspace file: %v", err)
	}

	state := FileState{
		Source:      "entities/missing.json",
		Field:       "script",
		ExpandedAt:  time.Now(),
		SourceMtime: time.Now(),
	}

	status, err := GetFileStatus(tmpDir, workspacePath, state)
	if err != nil {
		t.Fatalf("GetFileStatus() error = %v", err)
	}
	if status != StatusStale {
		t.Errorf("GetFileStatus() = %v, want StatusStale", status)
	}
}

func TestGetFileStatus_Synced(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	entitiesDir := filepath.Join(tmpDir, "entities")
	if err := os.MkdirAll(entitiesDir, 0755); err != nil {
		t.Fatalf("Failed to create entities dir: %v", err)
	}
	sourcePath := filepath.Join(entitiesDir, "test.json")
	if err := os.WriteFile(sourcePath, []byte(`{"script": "# test"}`), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create workspace file
	workspaceDir := filepath.Join(tmpDir, WorkspaceDir, "server_scripts")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}
	workspacePath := filepath.Join(WorkspaceDir, "server_scripts", "test.py")
	fullPath := filepath.Join(tmpDir, workspacePath)
	if err := os.WriteFile(fullPath, []byte("# test"), 0644); err != nil {
		t.Fatalf("Failed to create workspace file: %v", err)
	}

	// Get file info for accurate times
	sourceInfo, _ := os.Stat(sourcePath)
	workspaceInfo, _ := os.Stat(fullPath)

	state := FileState{
		Source:         "entities/test.json",
		Field:          "script",
		ExpandedAt:     time.Now(),
		SourceMtime:    sourceInfo.ModTime(),
		WorkspaceMtime: workspaceInfo.ModTime(),
	}

	status, err := GetFileStatus(tmpDir, workspacePath, state)
	if err != nil {
		t.Fatalf("GetFileStatus() error = %v", err)
	}
	if status != StatusSynced {
		t.Errorf("GetFileStatus() = %v, want StatusSynced", status)
	}
}

func TestGetFileStatus_Modified(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	entitiesDir := filepath.Join(tmpDir, "entities")
	if err := os.MkdirAll(entitiesDir, 0755); err != nil {
		t.Fatalf("Failed to create entities dir: %v", err)
	}
	sourcePath := filepath.Join(entitiesDir, "test.json")
	if err := os.WriteFile(sourcePath, []byte(`{"script": "# test"}`), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create workspace file
	workspaceDir := filepath.Join(tmpDir, WorkspaceDir, "server_scripts")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}
	workspacePath := filepath.Join(WorkspaceDir, "server_scripts", "test.py")
	fullPath := filepath.Join(tmpDir, workspacePath)
	if err := os.WriteFile(fullPath, []byte("# test modified"), 0644); err != nil {
		t.Fatalf("Failed to create workspace file: %v", err)
	}

	// Get file info
	sourceInfo, _ := os.Stat(sourcePath)

	// Set workspace mtime in the past
	state := FileState{
		Source:         "entities/test.json",
		Field:          "script",
		ExpandedAt:     time.Now().Add(-time.Hour),
		SourceMtime:    sourceInfo.ModTime(),
		WorkspaceMtime: time.Now().Add(-time.Hour), // Old timestamp
	}

	status, err := GetFileStatus(tmpDir, workspacePath, state)
	if err != nil {
		t.Fatalf("GetFileStatus() error = %v", err)
	}
	if status != StatusModified {
		t.Errorf("GetFileStatus() = %v, want StatusModified", status)
	}
}

func TestCodeFieldConstants(t *testing.T) {
	// Verify WorkspaceDir constant
	if WorkspaceDir != "weg_workspace" {
		t.Errorf("WorkspaceDir = %q, want %q", WorkspaceDir, "weg_workspace")
	}

	// Verify StateFile constant
	if StateFile != ".weg/workspace_state.json" {
		t.Errorf("StateFile = %q, want %q", StateFile, ".weg/workspace_state.json")
	}

	// Verify CodeFields is not empty
	if len(CodeFields) == 0 {
		t.Error("CodeFields is empty")
	}
}
