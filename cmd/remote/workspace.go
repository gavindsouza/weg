/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/workspace"
)

// refreshWorkspaceAfterPull reconciles weg_workspace/ after entity JSONs were
// rewritten from the remote site, so stale workspace copies can never be
// collapsed back over the fresher JSONs. Workspace files without local edits
// are re-expanded from the updated JSONs; files with local edits are left
// untouched (real conflicts are reported). A missing workspace is a no-op.
func refreshWorkspaceAfterPull(dirName string) {
	if _, err := os.Stat(filepath.Join(dirName, workspace.WorkspaceDir)); err != nil {
		return // no workspace in this clone
	}

	res, err := workspace.RefreshFromSource(dirName)
	if err != nil {
		output.Warningf("Workspace refresh failed: %v", err)
		return
	}

	if len(res.Refreshed) > 0 {
		output.Printf("  Refreshed %d workspace files from synced JSONs", len(res.Refreshed))
	}
	for _, f := range res.Conflicts {
		output.Warningf("Workspace conflict (local edits vs synced JSON): %s", f)
	}
	if len(res.Conflicts) > 0 {
		output.Warningf("Resolve with 'weg workspace collapse --force' (keep edits) or 'weg workspace expand --force' (keep JSON)")
	}
	for _, e := range res.Errors {
		output.Warningf("Workspace refresh error: %s", e)
	}
}
