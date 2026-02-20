/*
Copyright © 2025 Gavin <me@gavv.in>

Expand code from JSON files into workspace.
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

// ExpandOptions configures the expand operation
type ExpandOptions struct {
	BaseDir    string // Base directory of the weg clone
	EntityType string // Filter by entity type (empty = all)
	Clean      bool   // Remove stale files first
	Force      bool   // Overwrite even if conflicts
	Verbose    bool   // Print detailed output
}

// ExpandResult contains the results of an expand operation
type ExpandResult struct {
	Expanded  []string // Files expanded
	Skipped   []string // Files skipped (no code)
	Conflicts []string // Files with conflicts (not overwritten)
	Errors    []string // Errors encountered
}

// Expand extracts code from JSON files into the workspace
func Expand(opts ExpandOptions) (*ExpandResult, error) {
	result := &ExpandResult{}

	// Load current state
	state, err := LoadState(opts.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Create workspace directory
	workspaceDir := filepath.Join(opts.BaseDir, WorkspaceDir)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Find all JSON entity files
	entityFiles, err := findEntityFiles(opts.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find entity files: %w", err)
	}

	// Process each entity file
	for _, entityFile := range entityFiles {
		entityType := detectEntityType(entityFile)
		if entityType == "" {
			continue
		}

		// Filter by entity type if specified
		if opts.EntityType != "" && entityType != opts.EntityType {
			continue
		}

		// Get code fields for this entity type
		codeFields := GetCodeFieldsForEntity(entityType)
		if len(codeFields) == 0 {
			continue
		}

		// Read the JSON file
		jsonPath := filepath.Join(opts.BaseDir, entityFile)
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entityFile, err))
			continue
		}

		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid JSON: %v", entityFile, err))
			continue
		}

		// Get entity name for the output filename
		entityName := getEntityName(doc, entityFile)

		// Extract each code field
		for _, cf := range codeFields {
			code, ok := doc[cf.Field].(string)
			if !ok || strings.TrimSpace(code) == "" {
				continue
			}

			// Build workspace path
			workspacePath := buildWorkspacePath(cf, entityName, entityFile)
			fullWorkspacePath := filepath.Join(opts.BaseDir, workspacePath)

			// Check for conflicts
			if existingState, exists := state.Files[workspacePath]; exists && !opts.Force {
				status, _ := GetFileStatus(opts.BaseDir, workspacePath, existingState)
				if status == StatusConflict {
					result.Conflicts = append(result.Conflicts, workspacePath)
					continue
				}
			}

			// Create directory
			if err := os.MkdirAll(filepath.Dir(fullWorkspacePath), 0755); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to create directory: %v", workspacePath, err))
				continue
			}

			// Generate header and write file
			header := generateHeader(cf.Language, entityFile, cf.Field)
			content := header + code

			if err := os.WriteFile(fullWorkspacePath, []byte(content), 0644); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to write: %v", workspacePath, err))
				continue
			}

			// Update state
			sourceInfo, _ := os.Stat(jsonPath)
			workspaceInfo, _ := os.Stat(fullWorkspacePath)

			state.Files[workspacePath] = FileState{
				Source:         entityFile,
				Field:          cf.Field,
				ExpandedAt:     time.Now(),
				SourceMtime:    sourceInfo.ModTime(),
				WorkspaceMtime: workspaceInfo.ModTime(),
			}

			result.Expanded = append(result.Expanded, workspacePath)
		}
	}

	// Save state
	if err := state.Save(opts.BaseDir); err != nil {
		return result, fmt.Errorf("failed to save state: %w", err)
	}

	return result, nil
}

// findEntityFiles finds all JSON entity files in the clone
func findEntityFiles(baseDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and workspace
		name := info.Name()
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || name == WorkspaceDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Only JSON files
		if !strings.HasSuffix(name, ".json") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// detectEntityType determines the entity type from the file path
func detectEntityType(path string) string {
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		switch part {
		case "server_script":
			return "server_script"
		case "client_script":
			return "client_script"
		case "report":
			return "report"
		case "print_format":
			return "print_format"
		}
	}
	return ""
}

// getEntityName extracts the entity name from the document or path
func getEntityName(doc map[string]any, path string) string {
	// Try to get name from document
	if name, ok := doc["name"].(string); ok && name != "" {
		return name
	}

	// Fall back to filename without extension
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".json")
}

// buildWorkspacePath constructs the workspace file path
func buildWorkspacePath(cf CodeField, entityName, sourcePath string) string {
	sanitizedName := SanitizeFileName(entityName)

	// For reports with multiple files, use a subdirectory
	if cf.EntityType == "report" {
		return filepath.Join(WorkspaceDir, cf.SubDir, sanitizedName, sanitizedName+cf.Extension)
	}

	// For print formats with multiple files, use a subdirectory
	if cf.EntityType == "print_format" {
		return filepath.Join(WorkspaceDir, cf.SubDir, sanitizedName, sanitizedName+cf.Extension)
	}

	// Simple case: single file
	return filepath.Join(WorkspaceDir, cf.SubDir, sanitizedName+cf.Extension)
}

// generateHeader creates a header comment for the expanded file
func generateHeader(language, sourcePath, field string) string {
	var commentStart, commentEnd, lineComment string

	switch language {
	case "python":
		lineComment = "#"
	case "javascript":
		lineComment = "//"
	case "sql":
		lineComment = "--"
	case "html":
		commentStart = "<!--"
		commentEnd = "-->"
	case "css":
		commentStart = "/*"
		commentEnd = "*/"
	default:
		lineComment = "#"
	}

	if commentStart != "" {
		// Block comment style
		return fmt.Sprintf(`%s
  AUTO-GENERATED by weg workspace expand
  Source: %s
  Field: %s

  Edit this file, then run: weg workspace collapse
  Do NOT edit the source JSON directly.
%s

`, commentStart, sourcePath, field, commentEnd)
	}

	// Line comment style
	line := strings.Repeat("-", 60)
	return fmt.Sprintf(`%s %s
%s AUTO-GENERATED by weg workspace expand
%s Source: %s
%s Field: %s
%s
%s Edit this file, then run: weg workspace collapse
%s Do NOT edit the source JSON directly.
%s %s

`, lineComment, line,
		lineComment,
		lineComment, sourcePath,
		lineComment, field,
		lineComment,
		lineComment,
		lineComment,
		lineComment, line)
}
