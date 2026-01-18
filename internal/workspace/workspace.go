/*
Copyright © 2025 Gavin <me@gavv.in>

Workspace management for expanded code editing.
*/
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// WorkspaceDir is the directory name for expanded code
	WorkspaceDir = "weg_workspace"

	// StateFile is the workspace state tracking file
	StateFile = ".weg/workspace_state.json"
)

// CodeField represents a field in a JSON entity that contains code
type CodeField struct {
	EntityType string // e.g., "server_script", "client_script"
	Field      string // e.g., "script", "report_script"
	Extension  string // e.g., ".py", ".js"
	Language   string // e.g., "python", "javascript"
	SubDir     string // e.g., "server_scripts", "client_scripts"
}

// CodeFields defines all extractable code fields
var CodeFields = []CodeField{
	{EntityType: "server_script", Field: "script", Extension: ".py", Language: "python", SubDir: "server_scripts"},
	{EntityType: "client_script", Field: "script", Extension: ".js", Language: "javascript", SubDir: "client_scripts"},
	{EntityType: "report", Field: "report_script", Extension: ".py", Language: "python", SubDir: "reports"},
	{EntityType: "report", Field: "javascript", Extension: ".js", Language: "javascript", SubDir: "reports"},
	{EntityType: "report", Field: "query", Extension: ".sql", Language: "sql", SubDir: "reports"},
	{EntityType: "print_format", Field: "html", Extension: ".html", Language: "html", SubDir: "print_formats"},
	{EntityType: "print_format", Field: "css", Extension: ".css", Language: "css", SubDir: "print_formats"},
}

// FileState tracks the state of an expanded file
type FileState struct {
	Source         string    `json:"source"`          // Path to source JSON file
	Field          string    `json:"field"`           // Field name in JSON
	ExpandedAt     time.Time `json:"expanded_at"`     // When the file was expanded
	SourceMtime    time.Time `json:"source_mtime"`    // Source file mtime at expansion
	WorkspaceMtime time.Time `json:"workspace_mtime"` // Workspace file mtime at expansion
}

// WorkspaceState tracks all expanded files
type WorkspaceState struct {
	Files map[string]FileState `json:"files"` // workspace path -> state
}

// LoadState loads the workspace state from disk
func LoadState(baseDir string) (*WorkspaceState, error) {
	statePath := filepath.Join(baseDir, StateFile)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &WorkspaceState{Files: make(map[string]FileState)}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state WorkspaceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	if state.Files == nil {
		state.Files = make(map[string]FileState)
	}

	return &state, nil
}

// Save saves the workspace state to disk
func (s *WorkspaceState) Save(baseDir string) error {
	statePath := filepath.Join(baseDir, StateFile)

	// Ensure .weg directory exists
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return fmt.Errorf("failed to create .weg directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// FileStatus represents the sync status of a workspace file
type FileStatus int

const (
	StatusSynced   FileStatus = iota // No changes
	StatusModified                   // Workspace file modified
	StatusConflict                   // Both source and workspace modified
	StatusStale                      // Source JSON deleted
	StatusNew                        // New file in workspace (no source)
)

func (s FileStatus) String() string {
	switch s {
	case StatusSynced:
		return "synced"
	case StatusModified:
		return "modified"
	case StatusConflict:
		return "conflict"
	case StatusStale:
		return "stale"
	case StatusNew:
		return "new"
	default:
		return "unknown"
	}
}

// GetFileStatus determines the sync status of a workspace file
func GetFileStatus(baseDir, workspacePath string, state FileState) (FileStatus, error) {
	sourcePath := filepath.Join(baseDir, state.Source)
	fullWorkspacePath := filepath.Join(baseDir, workspacePath)

	// Check if source exists
	sourceInfo, sourceErr := os.Stat(sourcePath)
	if os.IsNotExist(sourceErr) {
		return StatusStale, nil
	}
	if sourceErr != nil {
		return StatusSynced, sourceErr
	}

	// Check if workspace file exists
	workspaceInfo, workspaceErr := os.Stat(fullWorkspacePath)
	if os.IsNotExist(workspaceErr) {
		return StatusNew, nil
	}
	if workspaceErr != nil {
		return StatusSynced, workspaceErr
	}

	// Check for modifications
	sourceModified := sourceInfo.ModTime().After(state.SourceMtime.Add(time.Second))
	workspaceModified := workspaceInfo.ModTime().After(state.WorkspaceMtime.Add(time.Second))

	if sourceModified && workspaceModified {
		return StatusConflict, nil
	}
	if workspaceModified {
		return StatusModified, nil
	}

	return StatusSynced, nil
}

// GetCodeFieldForEntity returns the code field definition for an entity type and field
func GetCodeFieldForEntity(entityType, field string) *CodeField {
	for _, cf := range CodeFields {
		if cf.EntityType == entityType && cf.Field == field {
			return &cf
		}
	}
	return nil
}

// GetCodeFieldsForEntity returns all code fields for an entity type
func GetCodeFieldsForEntity(entityType string) []CodeField {
	var fields []CodeField
	for _, cf := range CodeFields {
		if cf.EntityType == entityType {
			fields = append(fields, cf)
		}
	}
	return fields
}

// SanitizeFileName converts an entity name to a valid filename
func SanitizeFileName(name string) string {
	// Replace problematic characters
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ToLower(name)
	return name
}
