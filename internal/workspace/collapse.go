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
	Force    bool   // Resolve genuine conflicts in favor of the workspace edit
	Validate bool   // Run linters before collapse
	Verbose  bool   // Print detailed output
}

// CollapseResult contains the results of a collapse operation
type CollapseResult struct {
	Updated   []string // Workspace files whose edits were packed into their source JSON
	Refreshed []string // Workspace files refreshed FROM a newer JSON (no local edits to collapse)
	Unchanged []string // Files with no changes
	Conflicts []string // Files with conflicts (not updated)
	Errors    []string // Errors encountered
}

// Collapse packs workspace files back into JSON.
//
// Direction rules:
//   - A workspace file with local edits (and an unchanged JSON) is packed into
//     its source JSON. This is the normal collapse direction.
//   - A workspace file WITHOUT local edits whose source JSON changed (e.g.
//     after `weg remote sync`) is refreshed from the JSON instead. Collapsing
//     it would overwrite the newer JSON with stale content, so this happens
//     even with Force.
//   - A genuine conflict (both sides changed since the last expand) is
//     reported. With Force the workspace edit wins and is packed into the
//     JSON, overwriting the JSON-side change.
func Collapse(opts CollapseOptions) (*CollapseResult, error) {
	result := &CollapseResult{}

	// Load current state
	state, err := LoadState(opts.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	stateDirty := false

	// Track which source files we've updated (to handle multiple fields per file)
	updatedSources := make(map[string]map[string]any)
	// Code content collapsed per workspace file, for recording base hashes.
	collapsedCode := make(map[string]string)

	// Process each tracked workspace file
	for workspacePath, fileState := range state.Files {
		fullWorkspacePath := filepath.Join(opts.BaseDir, workspacePath)

		// Check if workspace file exists
		if _, err := os.Stat(fullWorkspacePath); os.IsNotExist(err) {
			// File was deleted from workspace, skip
			continue
		}

		status, _ := GetFileStatus(opts.BaseDir, workspacePath, fileState)
		switch status {
		case StatusConflict:
			if !opts.Force {
				result.Conflicts = append(result.Conflicts, workspacePath)
				continue
			}
			// Force: the workspace edit wins over the JSON-side change.
		case StatusSourceModified:
			// The source JSON changed and the workspace copy has no local
			// edits: collapsing would clobber the newer JSON with stale
			// content. Refresh the workspace file from the JSON instead —
			// deliberately even with Force.
			if opts.DryRun {
				result.Refreshed = append(result.Refreshed, workspacePath)
				continue
			}
			if err := refreshWorkspaceFile(opts.BaseDir, workspacePath, &fileState); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: refresh from JSON failed: %v", workspacePath, err))
				continue
			}
			state.Files[workspacePath] = fileState
			stateDirty = true
			result.Refreshed = append(result.Refreshed, workspacePath)
			continue
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
		var doc map[string]any

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
			// Auto-heal stale state (hashes/mtimes) for content-identical files.
			if !opts.DryRun && healState(opts.BaseDir, workspacePath, &fileState) {
				state.Files[workspacePath] = fileState
				stateDirty = true
			}
			continue
		}

		// Update the field
		doc[fileState.Field] = code
		updatedSources[fileState.Source] = doc
		collapsedCode[workspacePath] = code

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

		// Update state with new mtimes and base hashes
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
				if code, ok := collapsedCode[workspacePath]; ok {
					fileState.BaseHash = HashContent(code)
				}
				state.Files[workspacePath] = fileState
			}
		}

		if len(updatedSources) > 0 {
			stateDirty = true
		}
		if stateDirty {
			if err := state.Save(opts.BaseDir); err != nil {
				return result, fmt.Errorf("failed to save state: %w", err)
			}
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
