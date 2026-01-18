/*
Copyright © 2025 Gavin <me@gavv.in>

Collapse workspace files back into JSON.
*/
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CollapseOptions configures the collapse operation
type CollapseOptions struct {
	BaseDir  string // Base directory of the weg clone
	DryRun   bool   // Show what would change without modifying
	Force    bool   // Overwrite even if conflicts
	Validate bool   // Run linters before collapse
	Verbose  bool   // Print detailed output
}

// CollapseResult contains the results of a collapse operation
type CollapseResult struct {
	Updated   []string // JSON files updated
	Unchanged []string // Files with no changes
	Conflicts []string // Files with conflicts (not updated)
	Errors    []string // Errors encountered
}

// Collapse packs workspace files back into JSON
func Collapse(opts CollapseOptions) (*CollapseResult, error) {
	result := &CollapseResult{}

	// Load current state
	state, err := LoadState(opts.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Track which source files we've updated (to handle multiple fields per file)
	updatedSources := make(map[string]map[string]interface{})

	// Process each tracked workspace file
	for workspacePath, fileState := range state.Files {
		fullWorkspacePath := filepath.Join(opts.BaseDir, workspacePath)

		// Check if workspace file exists
		if _, err := os.Stat(fullWorkspacePath); os.IsNotExist(err) {
			// File was deleted from workspace, skip
			continue
		}

		// Check for conflicts
		if !opts.Force {
			status, _ := GetFileStatus(opts.BaseDir, workspacePath, fileState)
			if status == StatusConflict {
				result.Conflicts = append(result.Conflicts, workspacePath)
				continue
			}
		}

		// Read workspace file
		workspaceContent, err := os.ReadFile(fullWorkspacePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", workspacePath, err))
			continue
		}

		// Strip header from content
		code := stripHeader(string(workspaceContent))

		// Load source JSON (from cache or disk)
		sourcePath := filepath.Join(opts.BaseDir, fileState.Source)
		var doc map[string]interface{}

		if cached, exists := updatedSources[fileState.Source]; exists {
			doc = cached
		} else {
			data, err := os.ReadFile(sourcePath)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: source not found: %v", workspacePath, err))
				continue
			}
			if err := json.Unmarshal(data, &doc); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid JSON: %v", workspacePath, err))
				continue
			}
		}

		// Check if code actually changed
		existingCode, _ := doc[fileState.Field].(string)
		if existingCode == code {
			result.Unchanged = append(result.Unchanged, workspacePath)
			continue
		}

		// Update the field
		doc[fileState.Field] = code
		updatedSources[fileState.Source] = doc

		result.Updated = append(result.Updated, workspacePath)
	}

	// Write updated JSON files
	if !opts.DryRun {
		for sourcePath, doc := range updatedSources {
			fullSourcePath := filepath.Join(opts.BaseDir, sourcePath)

			data, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to marshal: %v", sourcePath, err))
				continue
			}

			if err := os.WriteFile(fullSourcePath, data, 0644); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to write: %v", sourcePath, err))
				continue
			}
		}

		// Update state with new mtimes
		now := time.Now()
		for workspacePath, fileState := range state.Files {
			if _, exists := updatedSources[fileState.Source]; exists {
				fullWorkspacePath := filepath.Join(opts.BaseDir, workspacePath)
				fullSourcePath := filepath.Join(opts.BaseDir, fileState.Source)

				sourceInfo, _ := os.Stat(fullSourcePath)
				workspaceInfo, _ := os.Stat(fullWorkspacePath)

				fileState.ExpandedAt = now
				if sourceInfo != nil {
					fileState.SourceMtime = sourceInfo.ModTime()
				}
				if workspaceInfo != nil {
					fileState.WorkspaceMtime = workspaceInfo.ModTime()
				}
				state.Files[workspacePath] = fileState
			}
		}

		if err := state.Save(opts.BaseDir); err != nil {
			return result, fmt.Errorf("failed to save state: %w", err)
		}
	}

	return result, nil
}

// stripHeader removes the auto-generated header from file content
func stripHeader(content string) string {
	// Pattern for line-comment headers (Python, JS, SQL)
	lineCommentPattern := regexp.MustCompile(`(?s)^[#/-]+ -+\n.*?[#/-]+ -+\n\n?`)

	// Pattern for block-comment headers (HTML, CSS)
	blockCommentPattern := regexp.MustCompile(`(?s)^<!--.*?-->\n\n?|^/\*.*?\*/\n\n?`)

	// Try line comment first
	if match := lineCommentPattern.FindString(content); match != "" {
		return strings.TrimPrefix(content, match)
	}

	// Try block comment
	if match := blockCommentPattern.FindString(content); match != "" {
		return strings.TrimPrefix(content, match)
	}

	return content
}

// Status returns the sync status of all workspace files
func Status(baseDir string) (map[string]FileStatus, error) {
	state, err := LoadState(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	statuses := make(map[string]FileStatus)

	for workspacePath, fileState := range state.Files {
		status, err := GetFileStatus(baseDir, workspacePath, fileState)
		if err != nil {
			continue
		}
		statuses[workspacePath] = status
	}

	return statuses, nil
}
