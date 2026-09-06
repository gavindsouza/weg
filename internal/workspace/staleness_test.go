/*
Copyright © 2025 Gavin <me@gavv.in>

Regression tests for the workspace staleness incident (2026-07-29):

 1. `weg remote sync` rewrote source JSONs, leaving workspace state stale.
    `collapse --dry-run` then reported conflicts for files whose JSON field
    content was byte-identical to the workspace copy (false conflicts).
 2. Two workspace files were genuinely stale (the site had appended an
    `execute()` line that only the JSONs had). `collapse --force` wrote the
    stale workspace content over the newer JSONs, silently dropping the
    `execute()` call.
*/
package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scriptFixture is a server_script entity with its expanded workspace file.
type scriptFixture struct {
	baseDir   string
	jsonRel   string // relative path of the source JSON
	wsRel     string // relative path of the workspace file
	fileState FileState
}

// newScriptFixture creates a source JSON holding jsonCode and a workspace
// file holding wsCode (with the generated header), plus a FileState entry.
// The recorded mtimes are deliberately ancient so that both files look
// modified to the legacy mtime heuristic — the incident conditions.
func newScriptFixture(t *testing.T, baseDir, name, jsonCode, wsCode, baseHash string) scriptFixture {
	t.Helper()

	jsonRel := filepath.Join("custom", "server_script", name+".json")
	wsRel := filepath.Join(WorkspaceDir, "server_scripts", name+".py")

	writeScriptJSON(t, baseDir, jsonRel, name, jsonCode)

	fullWs := filepath.Join(baseDir, wsRel)
	if err := os.MkdirAll(filepath.Dir(fullWs), 0755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	header := generateHeader("python", jsonRel, "script")
	if err := os.WriteFile(fullWs, []byte(header+wsCode), 0644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	ancient := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	return scriptFixture{
		baseDir: baseDir,
		jsonRel: jsonRel,
		wsRel:   wsRel,
		fileState: FileState{
			Source:         jsonRel,
			Field:          "script",
			ExpandedAt:     ancient,
			SourceMtime:    ancient,
			WorkspaceMtime: ancient,
			BaseHash:       baseHash,
		},
	}
}

func writeScriptJSON(t *testing.T, baseDir, jsonRel, name, code string) {
	t.Helper()
	fullJSON := filepath.Join(baseDir, jsonRel)
	if err := os.MkdirAll(filepath.Dir(fullJSON), 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	data, err := json.MarshalIndent(map[string]any{"name": name, "script": code}, "", "  ")
	if err != nil {
		t.Fatalf("marshal source: %v", err)
	}
	if err := os.WriteFile(fullJSON, data, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
}

func saveFixtures(t *testing.T, baseDir string, fixtures ...scriptFixture) {
	t.Helper()
	state := &WorkspaceState{Files: make(map[string]FileState)}
	for _, f := range fixtures {
		state.Files[f.wsRel] = f.fileState
	}
	if err := state.Save(baseDir); err != nil {
		t.Fatalf("save state: %v", err)
	}
}

func readScriptField(t *testing.T, baseDir, jsonRel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(baseDir, jsonRel))
	if err != nil {
		t.Fatalf("read source JSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse source JSON: %v", err)
	}
	code, _ := doc["script"].(string)
	return code
}

func readWorkspaceContent(t *testing.T, baseDir, wsRel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(baseDir, wsRel))
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}
	return stripHeader(string(data))
}

// contains reports whether list has the exact element want.
func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// Incident #2 (data loss): the site gained a trailing execute() line that
// remote sync wrote to the JSON, while the workspace copy stayed at the old
// content and was never edited by the user. collapse — even with --force —
// must NOT write the stale workspace content over the newer JSON; it must
// refresh the workspace file from the JSON instead.
func TestCollapse_StaleWorkspaceDoesNotClobberNewerJSON(t *testing.T) {
	baseCode := "print('scheduler')\n"
	newerJSON := baseCode + "execute()\n" // change synced from the site

	for _, force := range []bool{false, true} {
		name := "no-force"
		if force {
			name = "force"
		}
		t.Run(name, func(t *testing.T) {
			baseDir := t.TempDir()
			fx := newScriptFixture(t, baseDir, "scheduler_to_create_commitments",
				newerJSON, baseCode, HashContent(baseCode))
			saveFixtures(t, baseDir, fx)

			result, err := Collapse(CollapseOptions{BaseDir: baseDir, Force: force})
			if err != nil {
				t.Fatalf("Collapse() error = %v", err)
			}

			// The JSON must keep the site-side change.
			gotJSON := readScriptField(t, baseDir, fx.jsonRel)
			if !strings.Contains(gotJSON, "execute()") {
				t.Errorf("JSON lost the execute() call: %q", gotJSON)
			}
			if gotJSON != newerJSON {
				t.Errorf("JSON field = %q, want untouched %q", gotJSON, newerJSON)
			}

			// The workspace file must be refreshed forward from the JSON.
			if got := readWorkspaceContent(t, baseDir, fx.wsRel); got != newerJSON {
				t.Errorf("workspace content = %q, want refreshed %q", got, newerJSON)
			}
			if !contains(result.Refreshed, fx.wsRel) {
				t.Errorf("Refreshed = %v, want to contain %s", result.Refreshed, fx.wsRel)
			}
			if len(result.Updated) != 0 {
				t.Errorf("Updated = %v, want empty (nothing collapsed backwards)", result.Updated)
			}
			if len(result.Conflicts) != 0 {
				t.Errorf("Conflicts = %v, want empty", result.Conflicts)
			}

			// State must record the new base.
			state, err := LoadState(baseDir)
			if err != nil {
				t.Fatalf("LoadState() error = %v", err)
			}
			if got := state.Files[fx.wsRel].BaseHash; got != HashContent(newerJSON) {
				t.Errorf("BaseHash = %q, want hash of refreshed content", got)
			}
		})
	}
}

// Incident #2 with a legacy state entry (no recorded base hash, as written by
// pre-fix weg): the workspace file's mtime still matches the recorded state
// (user never touched it) while the JSON is newer. collapse --force must
// refresh the workspace file instead of clobbering the JSON.
func TestCollapse_LegacyStateStaleWorkspaceDoesNotClobberNewerJSON(t *testing.T) {
	baseCode := "print('scheduler')\n"
	newerJSON := baseCode + "execute()\n"

	baseDir := t.TempDir()
	fx := newScriptFixture(t, baseDir, "scheduler_to_create_time_cycles",
		newerJSON, baseCode, "" /* legacy: no base hash */)

	// The workspace file is untouched since expansion: pin its mtime to the
	// recorded state. The JSON keeps its fresh (newer) mtime.
	expandedAt := time.Now().Add(-24 * time.Hour)
	fullWs := filepath.Join(baseDir, fx.wsRel)
	if err := os.Chtimes(fullWs, expandedAt, expandedAt); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	fx.fileState.ExpandedAt = expandedAt
	fx.fileState.WorkspaceMtime = expandedAt
	fx.fileState.SourceMtime = expandedAt.Add(-time.Hour) // JSON rewritten since
	saveFixtures(t, baseDir, fx)

	result, err := Collapse(CollapseOptions{BaseDir: baseDir, Force: true})
	if err != nil {
		t.Fatalf("Collapse() error = %v", err)
	}

	if got := readScriptField(t, baseDir, fx.jsonRel); got != newerJSON {
		t.Errorf("JSON field = %q, want untouched %q (execute() must survive)", got, newerJSON)
	}
	if got := readWorkspaceContent(t, baseDir, fx.wsRel); got != newerJSON {
		t.Errorf("workspace content = %q, want refreshed %q", got, newerJSON)
	}
	if !contains(result.Refreshed, fx.wsRel) {
		t.Errorf("Refreshed = %v, want to contain %s", result.Refreshed, fx.wsRel)
	}
	if len(result.Updated) != 0 {
		t.Errorf("Updated = %v, want empty", result.Updated)
	}
}

// Incident #1 (false conflict): after remote sync rewrote the JSONs, mtimes
// and recorded hashes were stale on both sides, but the JSON field content
// was byte-identical to the workspace code. That is not a conflict.
func TestCollapse_IdenticalContentIsNeverAConflict(t *testing.T) {
	code := "frappe.msgprint('report')\n"

	baseDir := t.TempDir()
	// Recorded base hash is stale garbage and mtimes are ancient: both the
	// hash and mtime heuristics are useless. Content equality must win.
	fx := newScriptFixture(t, baseDir, "teamplan", code, code, "stale-garbage-hash")
	saveFixtures(t, baseDir, fx)

	if status, err := GetFileStatus(baseDir, fx.wsRel, fx.fileState); err != nil || status != StatusSynced {
		t.Errorf("GetFileStatus() = %v, %v; want StatusSynced, nil", status, err)
	}

	dry, err := Collapse(CollapseOptions{BaseDir: baseDir, DryRun: true})
	if err != nil {
		t.Fatalf("Collapse(dry-run) error = %v", err)
	}
	if len(dry.Conflicts) != 0 {
		t.Errorf("dry-run Conflicts = %v, want none for identical content", dry.Conflicts)
	}
	if !contains(dry.Unchanged, fx.wsRel) {
		t.Errorf("dry-run Unchanged = %v, want to contain %s", dry.Unchanged, fx.wsRel)
	}

	// A real collapse heals the stale state.
	if _, err := Collapse(CollapseOptions{BaseDir: baseDir}); err != nil {
		t.Fatalf("Collapse() error = %v", err)
	}
	state, err := LoadState(baseDir)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got := state.Files[fx.wsRel].BaseHash; got != HashContent(code) {
		t.Errorf("BaseHash not healed: got %q, want hash of content", got)
	}
	// JSON content must be untouched.
	if got := readScriptField(t, baseDir, fx.jsonRel); got != code {
		t.Errorf("JSON field = %q, want untouched %q", got, code)
	}
}

// A genuine three-way divergence (both sides changed since the last expand)
// is a conflict: blocked without --force, workspace wins with --force.
func TestCollapse_GenuineConflictRequiresForce(t *testing.T) {
	baseCode := "print('v1')\n"
	jsonCode := baseCode + "execute()\n" // site-side change
	wsCode := "print('v2, user edit')\n" // local edit

	baseDir := t.TempDir()
	fx := newScriptFixture(t, baseDir, "conflicted", jsonCode, wsCode, HashContent(baseCode))
	saveFixtures(t, baseDir, fx)

	// Without --force: reported, nothing written.
	result, err := Collapse(CollapseOptions{BaseDir: baseDir})
	if err != nil {
		t.Fatalf("Collapse() error = %v", err)
	}
	if !contains(result.Conflicts, fx.wsRel) {
		t.Errorf("Conflicts = %v, want to contain %s", result.Conflicts, fx.wsRel)
	}
	if got := readScriptField(t, baseDir, fx.jsonRel); got != jsonCode {
		t.Errorf("JSON field = %q, want untouched %q", got, jsonCode)
	}

	// With --force: the workspace edit wins, explicitly.
	result, err = Collapse(CollapseOptions{BaseDir: baseDir, Force: true})
	if err != nil {
		t.Fatalf("Collapse(force) error = %v", err)
	}
	if !contains(result.Updated, fx.wsRel) {
		t.Errorf("Updated = %v, want to contain %s", result.Updated, fx.wsRel)
	}
	if got := readScriptField(t, baseDir, fx.jsonRel); got != wsCode {
		t.Errorf("JSON field = %q, want workspace edit %q", got, wsCode)
	}
}

// Content-aware status detection against the recorded base hash, with mtimes
// deliberately useless.
func TestGetFileStatus_ContentAware(t *testing.T) {
	baseCode := "print('base')\n"

	tests := []struct {
		name     string
		jsonCode string
		wsCode   string
		want     FileStatus
	}{
		{"identical content", baseCode, baseCode, StatusSynced},
		{"only workspace edited", baseCode, baseCode + "user_edit()\n", StatusModified},
		{"only JSON changed", baseCode + "execute()\n", baseCode, StatusSourceModified},
		{"both diverged", baseCode + "execute()\n", baseCode + "user_edit()\n", StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			fx := newScriptFixture(t, baseDir, "status_check", tt.jsonCode, tt.wsCode, HashContent(baseCode))

			got, err := GetFileStatus(baseDir, fx.wsRel, fx.fileState)
			if err != nil {
				t.Fatalf("GetFileStatus() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("GetFileStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

// RefreshFromSource is the post-sync reconciliation: unedited workspace files
// follow the JSON; locally-edited files are never touched and genuine
// divergence is reported.
func TestRefreshFromSource(t *testing.T) {
	baseCode := "print('base')\n"
	syncedJSON := baseCode + "execute()\n"
	editedWs := baseCode + "my_local_edit()\n"

	baseDir := t.TempDir()
	clean := newScriptFixture(t, baseDir, "clean_script", syncedJSON, baseCode, HashContent(baseCode))
	edited := newScriptFixture(t, baseDir, "edited_script", syncedJSON, editedWs, HashContent(baseCode))
	inSync := newScriptFixture(t, baseDir, "in_sync_script", baseCode, baseCode, "stale-hash")
	saveFixtures(t, baseDir, clean, edited, inSync)

	result, err := RefreshFromSource(baseDir)
	if err != nil {
		t.Fatalf("RefreshFromSource() error = %v", err)
	}

	// Unedited file: re-expanded from the synced JSON.
	if !contains(result.Refreshed, clean.wsRel) {
		t.Errorf("Refreshed = %v, want to contain %s", result.Refreshed, clean.wsRel)
	}
	if got := readWorkspaceContent(t, baseDir, clean.wsRel); got != syncedJSON {
		t.Errorf("clean workspace content = %q, want %q", got, syncedJSON)
	}

	// Edited file: reported as conflict, both sides untouched.
	if !contains(result.Conflicts, edited.wsRel) {
		t.Errorf("Conflicts = %v, want to contain %s", result.Conflicts, edited.wsRel)
	}
	if got := readWorkspaceContent(t, baseDir, edited.wsRel); got != editedWs {
		t.Errorf("edited workspace content = %q, want untouched %q", got, editedWs)
	}
	if got := readScriptField(t, baseDir, edited.jsonRel); got != syncedJSON {
		t.Errorf("edited JSON field = %q, want untouched %q", got, syncedJSON)
	}

	// Content-identical file with stale recorded state: healed.
	if !contains(result.Healed, inSync.wsRel) {
		t.Errorf("Healed = %v, want to contain %s", result.Healed, inSync.wsRel)
	}

	// Healed and refreshed base hashes are persisted.
	state, err := LoadState(baseDir)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got := state.Files[clean.wsRel].BaseHash; got != HashContent(syncedJSON) {
		t.Errorf("clean BaseHash = %q, want hash of synced content", got)
	}
	if got := state.Files[inSync.wsRel].BaseHash; got != HashContent(baseCode) {
		t.Errorf("in-sync BaseHash = %q, want healed hash", got)
	}
	// The edited file's state must be untouched so the conflict stays visible.
	if got := state.Files[edited.wsRel].BaseHash; got != HashContent(baseCode) {
		t.Errorf("edited BaseHash = %q, want original base hash", got)
	}
}
