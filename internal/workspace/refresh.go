/*
Copyright © 2025 Gavin <me@gavv.in>

Refresh workspace files from their source JSONs.
*/
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RefreshResult contains the results of a workspace refresh
type RefreshResult struct {
	Refreshed []string // Workspace files re-expanded from their (newer) source JSON
	Healed    []string // Files whose recorded state was stale but content already in sync
	Conflicts []string // Files with local edits that also changed on the JSON side (left untouched)
	Errors    []string // Errors encountered
}

// RefreshFromSource reconciles the workspace with the source JSONs after the
// JSONs changed underneath it (e.g. after `weg remote sync` or
// `weg remote pull` rewrote them):
//
//   - Files without local edits whose JSON changed are re-expanded from the
//     JSON.
//   - Files whose content already matches the JSON get their recorded state
//     healed (hashes/mtimes re-stamped) so later collapses see them as clean.
//   - Files with local edits are never touched; if the JSON also changed they
//     are reported as conflicts.
func RefreshFromSource(baseDir string) (*RefreshResult, error) {
	state, err := LoadState(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	result := &RefreshResult{}
	stateDirty := false

	for workspacePath, fileState := range state.Files {
		status, err := GetFileStatus(baseDir, workspacePath, fileState)
		if err != nil {
			continue
		}

		switch status {
		case StatusSynced:
			if healState(baseDir, workspacePath, &fileState) {
				state.Files[workspacePath] = fileState
				stateDirty = true
				result.Healed = append(result.Healed, workspacePath)
			}
		case StatusSourceModified:
			if err := refreshWorkspaceFile(baseDir, workspacePath, &fileState); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", workspacePath, err))
				continue
			}
			state.Files[workspacePath] = fileState
			stateDirty = true
			result.Refreshed = append(result.Refreshed, workspacePath)
		case StatusConflict:
			result.Conflicts = append(result.Conflicts, workspacePath)
		}
		// StatusModified: local edits pending, JSON side unchanged — leave
		// them for the next collapse. StatusStale/StatusNew: leave alone.
	}

	if stateDirty {
		if err := state.Save(baseDir); err != nil {
			return result, fmt.Errorf("failed to save state: %w", err)
		}
	}

	return result, nil
}

// refreshWorkspaceFile rewrites a workspace file from its source JSON field
// (header + current code) and updates the FileState in place.
func refreshWorkspaceFile(baseDir, workspacePath string, fs *FileState) error {
	sourcePath := filepath.Join(baseDir, fs.Source)
	code, ok := readSourceField(sourcePath, fs.Field)
	if !ok {
		return fmt.Errorf("failed to read source %s", fs.Source)
	}

	header := generateHeader(languageForFile(workspacePath, fs), fs.Source, fs.Field)
	fullWorkspacePath := filepath.Join(baseDir, workspacePath)
	if err := os.MkdirAll(filepath.Dir(fullWorkspacePath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(fullWorkspacePath, []byte(header+code), 0644); err != nil {
		return err
	}

	fs.ExpandedAt = time.Now()
	fs.BaseHash = HashContent(code)
	if info, err := os.Stat(sourcePath); err == nil {
		fs.SourceMtime = info.ModTime()
	}
	if info, err := os.Stat(fullWorkspacePath); err == nil {
		fs.WorkspaceMtime = info.ModTime()
	}
	return nil
}

// healState re-stamps a FileState whose recorded hash/mtimes drifted even
// though the content is in sync. Returns true if the state entry changed.
func healState(baseDir, workspacePath string, fs *FileState) bool {
	fullWorkspacePath := filepath.Join(baseDir, workspacePath)
	code, ok := readWorkspaceCode(fullWorkspacePath)
	if !ok {
		return false
	}
	sourceInfo, err := os.Stat(filepath.Join(baseDir, fs.Source))
	if err != nil {
		return false
	}
	workspaceInfo, err := os.Stat(fullWorkspacePath)
	if err != nil {
		return false
	}

	hash := HashContent(code)
	if fs.BaseHash == hash &&
		sourceInfo.ModTime().Equal(fs.SourceMtime) &&
		workspaceInfo.ModTime().Equal(fs.WorkspaceMtime) {
		return false
	}

	fs.BaseHash = hash
	fs.SourceMtime = sourceInfo.ModTime()
	fs.WorkspaceMtime = workspaceInfo.ModTime()
	return true
}

// languageForFile determines the comment language for a workspace file, from
// its code-field definition when available, else from its file extension.
func languageForFile(workspacePath string, fs *FileState) string {
	if cf := GetCodeFieldForEntity(detectEntityType(fs.Source), fs.Field); cf != nil {
		return cf.Language
	}
	switch filepath.Ext(workspacePath) {
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".sql":
		return "sql"
	case ".html":
		return "html"
	case ".css":
		return "css"
	default:
		return "python"
	}
}
