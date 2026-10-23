/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gavindsouza/weg/internal/remote"
)

var (
	deletionsLastSync = time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC) // 07:00 Berlin
	deletionsUntil    = time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC)
)

func trashRow(name, doctype, docname, owner, creation string) map[string]string {
	return map[string]string{"name": name, "deleted_doctype": doctype, "deleted_name": docname,
		"owner": owner, "creation": creation}
}

func entityAt(et remote.EntityType, name, module, path string, data map[string]any) remote.Entity {
	return remote.Entity{Type: et, Name: name, Module: module, FilePath: path, Data: data}
}

// summarize renders commits as "<author> <files>" for comparison.
func summarize(commits []loggedCommit) []string {
	var out []string
	for _, c := range commits {
		out = append(out, c.author+" "+strings.Join(c.files, ","))
	}
	return out
}

// inHead reports whether path is committed at rev.
func (r *historyRepo) inHead(rev, path string) bool {
	r.t.Helper()
	return gitRun(r.t, r.dir, "ls-tree", "--name-only", rev, "--", path) != ""
}

// Deleted Document rows are replayed as commits by whoever deleted the
// document, in time order with the Versions: a Server Script's file is
// removed, a Custom Field's row is dropped from its DocType file, and the
// file goes once its last row is deleted. A deletion of something the
// export never held is skipped, as are trash rows outside the window and
// of entity types that are not synced. The module's sync flag does not
// matter here: the site says who deleted the document.
func TestPullReplaysDeletions(t *testing.T) {
	for _, sub := range []string{"", "catalog"} {
		t.Run("dir="+sub, func(t *testing.T) {
			site := newFakeHistorySite(t)
			site.users = map[string]string{"alice@example.com": "Alice", "bob@example.com": "Bob", "carol@example.com": "Carol"}

			gone := map[string]any{"doctype": "Server Script", "name": "Gone", "module": "Projects", "script_type": "API", "script": "a = 1"}
			keep := map[string]any{"doctype": "Server Script", "name": "Keep", "module": "Projects", "script_type": "API", "script": "k = 1"}
			rowX := map[string]any{"name": "Subscription-x", "dt": "Subscription", "fieldname": "x"}
			rowY := map[string]any{"name": "Subscription-y", "dt": "Subscription", "fieldname": "y"}
			rowZ := map[string]any{"name": "Sales Invoice-z", "dt": "Sales Invoice", "fieldname": "z"}
			gonePath := "projects/server_script/gone.json"
			keepPath := "projects/server_script/keep.json"
			cfPath := "accounts/custom_field/subscription.json"
			siPath := "accounts/custom_field/sales_invoice.json"
			base := []remote.Entity{
				entityAt(remote.EntityServerScript, "Gone", "Projects", gonePath, gone),
				entityAt(remote.EntityServerScript, "Keep", "Projects", keepPath, keep),
				entityAt(remote.EntityCustomField, "Subscription", "Accounts", cfPath, map[string]any{"doctype": "Subscription", "custom_fields": []any{rowX, rowY}}),
				entityAt(remote.EntityCustomField, "Sales Invoice", "Accounts", siPath, map[string]any{"doctype": "Sales Invoice", "custom_fields": []any{rowZ}}),
			}
			repo := newHistoryRepoIn(t, site, deletionsLastSync, base, sub)
			repo.config.Modules["Projects"] = remote.ModuleInfo{App: "erpnext", Sync: false}
			baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")

			site.deleted = []map[string]string{
				trashRow("d0", "Server Script", "Keep", "bob@example.com", "2026-10-20 06:59:59.000000"), // covered by the last pull
				trashRow("d1", "Server Script", "Gone", "alice@example.com", "2026-10-20 10:00:00.000000"),
				trashRow("d2", "Custom Field", "Subscription-x", "carol@example.com", "2026-10-20 11:00:00.000000"),
				trashRow("d3", "Custom Field", "Sales Invoice-z", "bob@example.com", "2026-10-20 12:00:00.000000"),
				trashRow("d4", "Server Script", "Ghost", "bob@example.com", "2026-10-20 12:30:00.000000"), // never exported
				trashRow("d5", "Notification", "Alert", "bob@example.com", "2026-10-20 12:40:00.000000"),  // not synced
				trashRow("d6", "Server Script", "Keep", "bob@example.com", "2026-10-21 08:00:00.000000"),  // next pull's
			}
			cfNow := map[string]any{"doctype": "Subscription", "custom_fields": []map[string]any{rowY}}
			current := []remote.Entity{
				entityAt(remote.EntityServerScript, "Keep", "Projects", keepPath, keep),
				entityAt(remote.EntityCustomField, "Subscription", "Accounts", cfPath, cfNow),
			}
			if err := repo.pull(deletionsUntil, current); err != nil {
				t.Fatalf("pull: %v", err)
			}

			got := repo.log(baseSHA)
			want := []string{
				"Alice " + gonePath,
				"Carol " + cfPath,
				"Bob " + siPath,
				"Weg .weg/site.toml",
			}
			if strings.Join(summarize(got), "|") != strings.Join(want, "|") {
				t.Fatalf("commits = %v, want %v", summarize(got), want)
			}
			dates := []string{"2026-10-20T10:00:00+02:00", "2026-10-20T11:00:00+02:00", "2026-10-20T12:00:00+02:00"}
			for i, d := range dates {
				if got[i].authorDate != d || got[i].committerDate != d {
					t.Errorf("commit %d dated %s/%s, want %s", i, got[i].authorDate, got[i].committerDate, d)
				}
			}
			if !strings.Contains(got[0].subject, "remove Gone") || !strings.Contains(got[2].subject, "remove Sales Invoice-z") {
				t.Errorf("deletion subjects = %q, %q", got[0].subject, got[2].subject)
			}
			if repo.inHead(got[0].sha, gonePath) || repo.inHead(got[2].sha, siPath) {
				t.Errorf("deletion commits kept the deleted files")
			}
			if s := repo.show(got[1].sha, cfPath); s != marshalExport(t, map[string]any{"doctype": "Subscription", "custom_fields": []any{rowY}}) {
				t.Errorf("row deletion commit:\n%s", s)
			}
			if !repo.inHead("HEAD", keepPath) {
				t.Errorf("%s was removed by a trash row outside the window", keepPath)
			}
			if out := gitRun(t, repo.root, "status", "--porcelain"); out != "" {
				t.Errorf("working tree not clean after pull:\n%s", out)
			}
		})
	}
}

// After the replay, the reconcile removes the committed files the site no
// longer exports at all: the old path of a document that moved module, a
// DocType that is no longer custom, a deletion with no trash row. Files of
// an entity type that is not synced, of a module marked sync = false, files
// weg does not lay out, and anything outside the export stay. A snapshot
// pull removes the same files.
func TestPullRemovesWhatTheSiteNoLongerExports(t *testing.T) {
	for _, mode := range []string{"history", "snapshot"} {
		for _, sub := range []string{"", "catalog"} {
			t.Run(mode+"/dir="+sub, func(t *testing.T) {
				site := newFakeHistorySite(t)
				flow := map[string]any{"doctype": "Workflow", "name": "Flow", "document_type": "Teamplan Request"}
				thing := map[string]any{"doctype": "DocType", "name": "Thing", "module": "Custom", "custom": 1}
				orphan := map[string]any{"doctype": "Server Script", "name": "Orphan", "script": "o = 1"}
				alert := map[string]any{"doctype": "Notification", "name": "Alert"}
				old := map[string]any{"doctype": "Client Script", "name": "Old", "dt": "Sales Order"}
				keep := map[string]any{"doctype": "Server Script", "name": "Keep", "script": "k = 1"}
				paths := map[string]string{
					"flow":   "custom/workflow/flow.json",
					"moved":  "workplace/workflow/flow.json",
					"thing":  "custom/doctype/thing/thing.json",
					"orphan": "_/server_script/orphan.json",
					"alert":  "custom/notification/alert.json",
					"old":    "selling/client_script/old.json",
					"keep":   "custom/server_script/keep.json",
				}
				base := []remote.Entity{
					entityAt(remote.EntityWorkflow, "Flow", "Custom", paths["flow"], flow),
					entityAt(remote.EntityDocType, "Thing", "Custom", paths["thing"], thing),
					entityAt(remote.EntityServerScript, "Orphan", "_", paths["orphan"], orphan),
					entityAt(remote.EntityNotification, "Alert", "Custom", paths["alert"], alert),
					entityAt(remote.EntityClientScript, "Old", "Selling", paths["old"], old),
					entityAt(remote.EntityServerScript, "Keep", "Custom", paths["keep"], keep),
				}
				repo := newHistoryRepoIn(t, site, deletionsLastSync, base, sub)
				repo.config.Modules["Selling"] = remote.ModuleInfo{App: "erpnext", Sync: false}
				// Not laid out by weg, and (in a subdirectory) outside the export.
				extra := []string{filepath.Join(repo.dir, "custom", "README.md"), filepath.Join(repo.root, "custom", "workflow", "flow.json")}
				for _, p := range extra {
					os.MkdirAll(filepath.Dir(p), 0o755)
					os.WriteFile(p, []byte("{}"), 0o644)
				}
				gitRun(t, repo.root, "add", "-A")
				gitRun(t, repo.root, "commit", "-q", "-m", "docs: notes")
				baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")

				current := []remote.Entity{
					entityAt(remote.EntityWorkflow, "Flow", "Workplace", paths["moved"], flow),
					entityAt(remote.EntityServerScript, "Keep", "Custom", paths["keep"], keep),
				}
				if mode == "history" {
					if err := repo.pull(deletionsUntil, current); err != nil {
						t.Fatalf("pull: %v", err)
					}
				} else {
					t.Chdir(repo.dir)
					if err := pullSnapshot(repo.config, &remote.FetchResult{Entities: current}, deletionsUntil); err != nil {
						t.Fatalf("snapshot pull: %v", err)
					}
				}

				got := repo.log(baseSHA)
				if len(got) != 1 {
					t.Fatalf("commits = %v, want one", summarize(got))
				}
				changed := strings.Join(got[0].files, ",")
				for _, k := range []string{"flow", "moved", "thing", "orphan"} {
					if !strings.Contains(changed, paths[k]) {
						t.Errorf("%s commit does not touch %s: %s", mode, paths[k], changed)
					}
				}
				for _, k := range []string{"flow", "thing", "orphan"} {
					if repo.inHead("HEAD", paths[k]) {
						t.Errorf("%s is still committed", paths[k])
					}
				}
				for _, k := range []string{"moved", "alert", "old", "keep"} {
					if !repo.inHead("HEAD", paths[k]) {
						t.Errorf("%s is no longer committed", paths[k])
					}
				}
				if !repo.inHead("HEAD", "custom/README.md") {
					t.Errorf("custom/README.md was removed")
				}
				if sub != "" {
					if gitRun(t, repo.root, "ls-tree", "--name-only", "HEAD", "--", "custom/workflow/flow.json") == "" {
						t.Errorf("a file outside the export was removed")
					}
				}
				if out := gitRun(t, repo.root, "status", "--porcelain"); out != "" {
					t.Errorf("working tree not clean after pull:\n%s", out)
				}
			})
		}
	}
}

// A document deleted and created again inside the window ends as the site
// has it, with each step credited: the deletion to who deleted it, the new
// document to its creator (starting from its state at creation), later
// edits to their authors. This holds for a whole document and for a Custom
// Field row. A document the export never held, deleted and created again,
// starts at its new creation: its earlier life is unknown to the pull.
func TestPullDeletedAndRecreatedInWindow(t *testing.T) {
	site := newFakeHistorySite(t)
	site.users = map[string]string{"alice@example.com": "Alice", "bob@example.com": "Bob",
		"carol@example.com": "Carol", "dave@example.com": "Dave", "erin@example.com": "Erin", "frank@example.com": "Frank"}

	calc := map[string]any{"doctype": "Server Script", "name": "Calc", "script_type": "API", "script": "x = 1",
		"owner": "admin@example.com", "creation": "2026-01-01 09:00:00.000000",
		"modified": "2026-10-01 10:00:00.000000", "modified_by": "admin@example.com"}
	rowX := map[string]any{"name": "Subscription-x", "dt": "Subscription", "fieldname": "x", "hidden": 0,
		"owner": "admin@example.com", "creation": "2026-01-01 09:00:00.000000"}
	rowY := map[string]any{"name": "Subscription-y", "dt": "Subscription", "fieldname": "y"}
	calcPath := "projects/server_script/calc.json"
	freshPath := "projects/server_script/fresh.json"
	cfPath := "accounts/custom_field/subscription.json"
	repo := newHistoryRepo(t, site, deletionsLastSync, []remote.Entity{
		entityAt(remote.EntityServerScript, "Calc", "Projects", calcPath, calc),
		entityAt(remote.EntityCustomField, "Subscription", "Accounts", cfPath, map[string]any{"doctype": "Subscription", "custom_fields": []any{rowX, rowY}}),
	})
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")

	v := func(name, doctype, docname, owner, creation, data string) remote.VersionRecord {
		return remote.VersionRecord{Name: name, RefDoctype: doctype, Docname: docname, Owner: owner, Creation: creation, Data: data}
	}
	site.versions = []remote.VersionRecord{
		v("v1", "Server Script", "Calc", "alice@example.com", "2026-10-20 09:00:00.000000", `{"changed":[["script","x = 1","x = 2"]]}`),
		v("v2", "Server Script", "Fresh", "erin@example.com", "2026-10-20 09:15:00.000000", `{"changed":[["script","f = 0","f = 1"]]}`),
		v("v3", "Server Script", "Calc", "dave@example.com", "2026-10-20 12:00:00.000000", `{"changed":[["script","y = 1","y = 2"]]}`),
	}
	site.deleted = []map[string]string{
		trashRow("d1", "Server Script", "Fresh", "erin@example.com", "2026-10-20 09:30:00.000000"),
		trashRow("d2", "Server Script", "Calc", "bob@example.com", "2026-10-20 10:00:00.000000"),
		trashRow("d3", "Custom Field", "Subscription-x", "bob@example.com", "2026-10-20 13:00:00.000000"),
	}
	calcNow := map[string]any{"doctype": "Server Script", "name": "Calc", "script_type": "API", "script": "y = 2",
		"owner": "carol@example.com", "creation": "2026-10-20 11:00:00.000000",
		"modified": "2026-10-20 12:00:00.000000", "modified_by": "dave@example.com"}
	fresh := map[string]any{"doctype": "Server Script", "name": "Fresh", "script": "f = 9",
		"owner": "frank@example.com", "creation": "2026-10-20 10:30:00.000000"}
	rowXNow := map[string]any{"name": "Subscription-x", "dt": "Subscription", "fieldname": "x", "hidden": 1,
		"owner": "carol@example.com", "creation": "2026-10-20 14:00:00.000000"}
	cfNow := map[string]any{"doctype": "Subscription", "custom_fields": []map[string]any{rowXNow, rowY}}
	current := []remote.Entity{
		entityAt(remote.EntityServerScript, "Calc", "Projects", calcPath, calcNow),
		entityAt(remote.EntityServerScript, "Fresh", "Projects", freshPath, fresh),
		entityAt(remote.EntityCustomField, "Subscription", "Accounts", cfPath, cfNow),
	}
	if err := repo.pull(deletionsUntil, current); err != nil {
		t.Fatalf("pull: %v", err)
	}

	got := repo.log(baseSHA)
	want := []string{
		"Alice " + calcPath,
		"Bob " + calcPath,
		"Frank " + freshPath,
		"Carol " + calcPath,
		"Dave " + calcPath,
		"Bob " + cfPath,
		"Carol " + cfPath,
		"Weg .weg/site.toml",
	}
	if strings.Join(summarize(got), "|") != strings.Join(want, "|") {
		t.Fatalf("commits = %v, want %v", summarize(got), want)
	}
	if repo.inHead(got[1].sha, calcPath) {
		t.Errorf("the deletion commit kept %s", calcPath)
	}
	if !strings.HasPrefix(got[3].subject, "feat(") {
		t.Errorf("re-creation subject = %q, want a feat commit", got[3].subject)
	}
	created := copyDoc(calcNow, "script", "y = 1", "modified", "2026-10-20 11:00:00.000000", "modified_by", "carol@example.com")
	if s := repo.show(got[3].sha, calcPath); s != marshalExport(t, created) {
		t.Errorf("re-creation commit is not the document as created:\n%s", s)
	}
	if s := repo.show(got[4].sha, calcPath); s != marshalExport(t, calcNow) {
		t.Errorf("last commit is not the current document:\n%s", s)
	}
	if s := repo.show(got[2].sha, freshPath); s != marshalExport(t, fresh) {
		t.Errorf("creation commit:\n%s", s)
	}
	if s := repo.show(got[5].sha, cfPath); s != marshalExport(t, map[string]any{"doctype": "Subscription", "custom_fields": []any{rowY}}) {
		t.Errorf("row deletion commit:\n%s", s)
	}
	if s := repo.show(got[6].sha, cfPath); s != marshalExport(t, cfNow) {
		t.Errorf("row re-creation commit:\n%s", s)
	}
}

// Paths come from the committed export, never from a trash row's name: a
// Server Script named "../evil" is removed at the path it was exported to,
// a deletion of a name that was never exported touches nothing, and a file
// outside the export with the name's would-be path stays.
func TestPullDeletionStaysInsideExport(t *testing.T) {
	site := newFakeHistorySite(t)
	evil := map[string]any{"doctype": "Server Script", "name": "../evil", "script": "e = 1"}
	evilPath := "projects/server_script/___evil.json"
	repo := newHistoryRepoIn(t, site, deletionsLastSync,
		[]remote.Entity{entityAt(remote.EntityServerScript, "../evil", "Projects", evilPath, evil)}, "catalog")
	outside := filepath.Join(repo.root, "evil.json")
	os.WriteFile(outside, []byte("{}"), 0o644)
	gitRun(t, repo.root, "add", "evil.json")
	gitRun(t, repo.root, "commit", "-q", "-m", "docs: outside")
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")

	site.deleted = []map[string]string{
		trashRow("d1", "Server Script", "../evil", "alice@example.com", "2026-10-20 10:00:00.000000"),
		trashRow("d2", "Server Script", "../../evil", "alice@example.com", "2026-10-20 10:01:00.000000"),
		trashRow("d3", "Server Script", "../../../evil.json", "alice@example.com", "2026-10-20 10:02:00.000000"),
	}
	if err := repo.pull(deletionsUntil, nil); err != nil {
		t.Fatalf("pull: %v", err)
	}
	got := repo.log(baseSHA)
	if len(got) != 2 || strings.Join(got[0].files, ",") != evilPath || got[1].author != "Weg" || repo.inHead("HEAD", evilPath) {
		t.Fatalf("commits = %v, want the removal of %s, then the reconcile", summarize(got), evilPath)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside the export: %v", err)
	}
	if gitRun(t, repo.root, "ls-tree", "--name-only", "HEAD", "--", "evil.json") == "" {
		t.Fatalf("file outside the export was removed from HEAD")
	}
}

// A replayed removal is checked like a write: a path that leaves the export
// or enters a hidden directory is refused before anything is removed.
func TestApplyReplayedCommitRefusesUnsafeRemoval(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "catalog")
	os.MkdirAll(filepath.Join(dir, "projects", "server_script"), 0o755)
	gitRun(t, root, "init", "-q")
	os.WriteFile(filepath.Join(root, "evil.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "projects", "server_script", "a.json"), []byte("{}"), 0o644)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")

	for _, p := range []string{"../evil.json", ".git/config", "/etc/passwd", "projects/../../evil.json"} {
		touched := map[string]bool{}
		c := remote.ReconstructedCommit{Removed: []string{"projects/server_script/a.json", p}, Author: "A <a@example.com>", Message: "chore: remove"}
		if ok, err := applyReplayedCommit(dir, c, touched); err == nil || ok || !strings.Contains(err.Error(), "refusing") {
			t.Errorf("removing %q: committed=%v err=%v, want a refusal", p, ok, err)
		}
		if len(touched) != 0 {
			t.Errorf("removing %q touched %v", p, touched)
		}
	}
	for _, p := range []string{filepath.Join(root, "evil.json"), filepath.Join(dir, "projects", "server_script", "a.json")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if out := gitRun(t, root, "status", "--porcelain"); out != "" {
		t.Errorf("index changed:\n%s", out)
	}
}

// Without read access to Deleted Document the pull still converges: the
// deletion lands in the reconcile commit instead of a commit by its author.
func TestPullWithoutTrashAccess(t *testing.T) {
	site := newFakeHistorySite(t)
	site.deletedStatus = http.StatusForbidden
	gone := map[string]any{"doctype": "Server Script", "name": "Gone", "script": "a = 1"}
	keep := map[string]any{"doctype": "Server Script", "name": "Keep", "script": "k = 1"}
	gonePath, keepPath := "custom/server_script/gone.json", "custom/server_script/keep.json"
	repo := newHistoryRepo(t, site, deletionsLastSync, []remote.Entity{
		entityAt(remote.EntityServerScript, "Gone", "Custom", gonePath, gone),
		entityAt(remote.EntityServerScript, "Keep", "Custom", keepPath, keep),
	})
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")
	if err := repo.pull(deletionsUntil, []remote.Entity{entityAt(remote.EntityServerScript, "Keep", "Custom", keepPath, keep)}); err != nil {
		t.Fatalf("pull: %v", err)
	}
	got := repo.log(baseSHA)
	if s := strings.Join(summarize(got), "|"); s != "Weg .weg/site.toml,"+gonePath {
		t.Fatalf("commits = %s", s)
	}
}

// The push baseline follows a pull that started from it on a clean export,
// so the next push does not send the pull's removals back as deletions;
// with unpushed local commits, or uncommitted changes the pull's commit
// took in, it stays.
func TestAdvancePushBaseline(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	os.MkdirAll(filepath.Join(dir, ".weg"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.json"), []byte("{}"), 0o644)
	gitRun(t, dir, "add", "a.json")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	start := gitRun(t, dir, "rev-parse", "HEAD")
	gitRun(t, dir, "rm", "-q", "a.json")
	gitRun(t, dir, "commit", "-q", "-m", "chore(sync): reconcile")
	head := gitRun(t, dir, "rev-parse", "HEAD")
	baseline := filepath.Join(dir, ".weg", "last_push_commit")
	read := func() string { b, _ := os.ReadFile(baseline); return string(b) }

	os.WriteFile(baseline, []byte(start), 0o644)
	advancePushBaseline(dir, start, false)
	if read() != start {
		t.Errorf("baseline moved over uncommitted changes the pull committed")
	}
	advancePushBaseline(dir, start, true)
	if read() != head {
		t.Errorf("baseline = %s, want %s", read(), head)
	}

	other := "0000000000000000000000000000000000000000"
	os.WriteFile(baseline, []byte(other), 0o644)
	advancePushBaseline(dir, start, true)
	if read() != other {
		t.Errorf("baseline moved over unpushed commits: %s", read())
	}
}

// A committed file the site does not export but that has local work not
// yet pushed (committed since the push baseline, or edited) is kept: it may
// be a document made locally for the next push.
func TestPullKeepsUnpushedLocalFiles(t *testing.T) {
	site := newFakeHistorySite(t)
	keep := map[string]any{"doctype": "Server Script", "name": "Keep", "script": "k = 1"}
	gone := map[string]any{"doctype": "Server Script", "name": "Gone", "script": "g = 1"}
	edited := map[string]any{"doctype": "Server Script", "name": "Edited", "script": "e = 1"}
	keepPath, gonePath := "custom/server_script/keep.json", "custom/server_script/gone.json"
	editedPath, newPath := "custom/server_script/edited.json", "custom/server_script/new.json"
	repo := newHistoryRepo(t, site, deletionsLastSync, []remote.Entity{
		entityAt(remote.EntityServerScript, "Keep", "Custom", keepPath, keep),
		entityAt(remote.EntityServerScript, "Gone", "Custom", gonePath, gone),
		entityAt(remote.EntityServerScript, "Edited", "Custom", editedPath, edited),
	})
	os.WriteFile(filepath.Join(repo.dir, ".weg", "last_push_commit"), []byte(gitRun(t, repo.dir, "rev-parse", "HEAD")), 0o644)
	// Made locally, committed, not pushed yet.
	os.WriteFile(filepath.Join(repo.dir, newPath), []byte(`{"doctype": "Server Script", "name": "New"}`), 0o644)
	gitRun(t, repo.dir, "add", newPath)
	gitRun(t, repo.dir, "commit", "-q", "-m", "feat: new script")
	// Edited locally, not committed.
	os.WriteFile(filepath.Join(repo.dir, editedPath), []byte(`{"doctype": "Server Script", "name": "Edited", "script": "e = 2"}`), 0o644)

	if err := repo.pull(deletionsUntil, []remote.Entity{entityAt(remote.EntityServerScript, "Keep", "Custom", keepPath, keep)}); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if repo.inHead("HEAD", gonePath) {
		t.Errorf("%s is still committed", gonePath)
	}
	if !repo.inHead("HEAD", newPath) {
		t.Errorf("the unpushed %s was removed", newPath)
	}
	if b, err := os.ReadFile(filepath.Join(repo.dir, editedPath)); err != nil || !strings.Contains(string(b), "e = 2") {
		t.Errorf("the local edit of %s was lost: %s, %v", editedPath, b, err)
	}
}

// A file that moved to another path still holds its document, so push
// does not delete that document on the site; a file whose document is gone
// from the export is still a deletion.
func TestFindDeletedEntitiesSkipsMovedDocuments(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644)
	}
	write("custom/workflow/flow.json", `{"name": "Flow"}`)
	write("custom/server_script/gone.json", `{"name": "Gone"}`)
	write("custom/custom_field/lead.json", `{"doctype": "Lead", "custom_fields": []}`)
	write("custom/client_script/local.json", `{"name": "Local"}`)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	os.MkdirAll(filepath.Join(dir, ".weg"), 0o755)
	os.WriteFile(filepath.Join(dir, ".weg", "last_push_commit"), []byte(gitRun(t, dir, "rev-parse", "HEAD")), 0o644)
	gitRun(t, dir, "rm", "-q", "custom/workflow/flow.json", "custom/server_script/gone.json", "custom/custom_field/lead.json")
	write("workplace/workflow/flow.json", `{"name": "Flow"}`)
	write("workplace/custom_field/lead.json", `{"doctype": "Lead", "custom_fields": []}`)
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "chore(sync): reconcile")
	// Deleted in the working tree only: HEAD still has it.
	os.Remove(filepath.Join(dir, "custom", "client_script", "local.json"))

	got, err := findDeletedEntities(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].name != "Gone" {
		t.Fatalf("deleted = %+v, want only Gone", got)
	}
	got, err = findDeletedEntities(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range got {
		names = append(names, e.name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "Gone,Local" {
		t.Fatalf("deleted with uncommitted = %v, want Gone,Local", names)
	}
}

// When the push baseline is not a commit of the repository, local work
// cannot be told apart, so the pull removes nothing rather than guess.
func TestPullKeepsEverythingWithUnknownPushBaseline(t *testing.T) {
	site := newFakeHistorySite(t)
	keep := map[string]any{"doctype": "Server Script", "name": "Keep", "script": "k = 1"}
	gone := map[string]any{"doctype": "Server Script", "name": "Gone", "script": "g = 1"}
	keepPath, gonePath := "custom/server_script/keep.json", "custom/server_script/gone.json"
	repo := newHistoryRepo(t, site, deletionsLastSync, []remote.Entity{
		entityAt(remote.EntityServerScript, "Keep", "Custom", keepPath, keep),
		entityAt(remote.EntityServerScript, "Gone", "Custom", gonePath, gone),
	})
	os.WriteFile(filepath.Join(repo.dir, ".weg", "last_push_commit"), []byte("1111111111111111111111111111111111111111"), 0o644)
	if err := repo.pull(deletionsUntil, []remote.Entity{entityAt(remote.EntityServerScript, "Keep", "Custom", keepPath, keep)}); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if !repo.inHead("HEAD", gonePath) {
		t.Errorf("%s was removed although local work is unknown", gonePath)
	}
}
