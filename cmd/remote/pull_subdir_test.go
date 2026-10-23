/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gavindsouza/weg/internal/remote"
)

// With the export in a subdirectory of a larger repository (weg -C catalog),
// a pull starts from the committed export: a Custom Field change keeps the
// file's other rows, a pushed document edited and then deleted on the site
// is replayed at its real path and removed there, and the rest of the
// repository (staged or not) stays out of every commit.
func TestReconstructHistoryPullInSubdirectory(t *testing.T) {
	site := newFakeHistorySite(t)
	site.users = map[string]string{"alice@example.com": "Alice", "carol@example.com": "Carol"}
	lastSync := time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC)
	until := time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC)

	rowX := map[string]any{"name": "Subscription-x", "dt": "Subscription", "fieldname": "x", "hidden": 0}
	rowY := map[string]any{"name": "Subscription-y", "dt": "Subscription", "fieldname": "y", "hidden": 0}
	gone := map[string]any{"doctype": "Server Script", "name": "Gone", "module": "Projects", "script": "a = 1"}
	cfPath := "accounts/custom_field/subscription.json"
	gonePath := "projects/server_script/gone.json"
	base := []remote.Entity{
		{Type: remote.EntityCustomField, Name: "Subscription", Module: "Accounts", FilePath: cfPath,
			Data: map[string]any{"doctype": "Subscription", "custom_fields": []any{rowX, rowY}}},
		{Type: remote.EntityServerScript, Name: "Gone", Module: "Projects", FilePath: gonePath, Data: gone},
	}
	repo := newHistoryRepoIn(t, site, lastSync, base, "catalog")

	// The rest of the repository: one committed file with an unstaged edit,
	// one staged new file.
	os.WriteFile(filepath.Join(repo.root, "README.md"), []byte("app\n"), 0o644)
	gitRun(t, repo.root, "add", "README.md")
	gitRun(t, repo.root, "commit", "-q", "-m", "docs: readme")
	baseSHA := gitRun(t, repo.root, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(repo.dir, ".weg", "last_push_commit"), []byte(baseSHA), 0o644)
	os.WriteFile(filepath.Join(repo.root, "README.md"), []byte("app, edited\n"), 0o644)
	os.WriteFile(filepath.Join(repo.root, "notes.txt"), []byte("mine\n"), 0o644)
	gitRun(t, repo.root, "add", "notes.txt")

	cfNow := map[string]any{"doctype": "Subscription", "custom_fields": []map[string]any{
		copyDoc(rowX, "hidden", 1), rowY}}
	current := []remote.Entity{
		{Type: remote.EntityCustomField, Name: "Subscription", Module: "Accounts", FilePath: cfPath, Data: cfNow},
	}
	site.versions = []remote.VersionRecord{
		{Name: "v1", RefDoctype: "Custom Field", Docname: "Subscription-x", Owner: "carol@example.com",
			Creation: "2026-10-20 10:00:00.000000", Data: `{"changed":[["hidden",0,1]]}`},
		{Name: "v2", RefDoctype: "Server Script", Docname: "Gone", Owner: "alice@example.com",
			Creation: "2026-10-20 11:00:00.000000", Data: `{"changed":[["script","a = 1","a = 2"]]}`},
	}

	if err := repo.pull(until, current); err != nil {
		t.Fatalf("pull: %v", err)
	}

	got := repo.log(baseSHA)
	want := []string{
		"Carol " + cfPath,
		"Alice " + gonePath,
		"Weg .weg/site.toml,modules.txt," + gonePath, // Projects has no entity left
	}
	var summary []string
	for _, c := range got {
		summary = append(summary, c.author+" "+strings.Join(c.files, ","))
	}
	if strings.Join(summary, "|") != strings.Join(want, "|") {
		t.Fatalf("commits = %v, want %v", summary, want)
	}

	// The Custom Field commit holds both rows: replay started from the
	// committed file, not from an empty one.
	if s := repo.show(got[0].sha, cfPath); s != marshalExport(t, cfNow) {
		t.Errorf("custom field commit lost sibling rows:\n%s", s)
	}
	// The deleted document's last edit is committed whole at its real path,
	// and the reconcile commit removes that file.
	if s := repo.show(got[1].sha, gonePath); s != marshalExport(t, copyDoc(gone, "script", "a = 2")) {
		t.Errorf("deleted document's edit:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(repo.dir, gonePath)); !os.IsNotExist(err) {
		t.Errorf("%s still in the working tree after reconcile (err %v)", gonePath, err)
	}

	// Every commit stays inside the export: checked per commit, so a path
	// one commit adds and a later one removes cannot hide in the net diff.
	for _, sha := range strings.Fields(gitRun(t, repo.root, "rev-list", baseSHA+"..HEAD")) {
		files := gitRun(t, repo.root, "diff-tree", "-r", "--no-commit-id", "--name-only", sha+"^", sha)
		for _, f := range strings.Fields(files) {
			if !strings.HasPrefix(f, "catalog/") || strings.HasPrefix(f, "catalog/_/") {
				t.Errorf("commit %.8s committed %s", sha, f)
			}
		}
	}
	// gitRun trims the leading space of " M README.md".
	status := gitRun(t, repo.root, "status", "--porcelain")
	if status != "M README.md\nA  notes.txt" {
		t.Errorf("rest of the repository changed by the pull:\n%s", status)
	}
}

// An entity path that climbs out of the export is refused: the pull fails,
// nothing is written outside the export, and the rollback does not touch the
// file the path points at either.
func TestReconstructHistoryPullRefusesEscapingPath(t *testing.T) {
	site := newFakeHistorySite(t)
	calc := map[string]any{"doctype": "Server Script", "name": "Calc", "script": "x = 1"}
	calcPath := "projects/server_script/calc.json"
	lastSync := time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC)
	repo := newHistoryRepoIn(t, site, lastSync,
		[]remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: calc, FilePath: calcPath}},
		"catalog")
	victim := filepath.Join(repo.root, "victim.json")
	os.WriteFile(victim, []byte("keep\n"), 0o644)
	gitRun(t, repo.root, "add", "victim.json")
	gitRun(t, repo.root, "commit", "-q", "-m", "victim")
	baseSHA := gitRun(t, repo.root, "rev-parse", "HEAD")

	evil := map[string]any{"doctype": "Server Script", "name": "../victim", "script": "pwned"}
	now := []remote.Entity{
		{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: copyDoc(calc, "script", "x = 2"), FilePath: calcPath},
		{Type: remote.EntityServerScript, Name: "../victim", Module: "Projects", Data: evil, FilePath: "../victim.json"},
	}
	site.versions = []remote.VersionRecord{
		{Name: "v1", RefDoctype: "Server Script", Docname: "Calc", Owner: "a@example.com",
			Creation: "2026-10-20 09:00:00.000000", Data: `{"changed":[["script","x = 1","x = 2"]]}`},
		{Name: "v2", RefDoctype: "Server Script", Docname: "../victim", Owner: "a@example.com",
			Creation: "2026-10-20 10:00:00.000000", Data: `{"changed":[["script","x","pwned"]]}`},
	}

	err := repo.pull(time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC), now)
	if err == nil || !strings.Contains(err.Error(), "refusing export path") {
		t.Fatalf("pull error = %v, want the escaping path refused", err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "keep\n" {
		t.Errorf("file outside the export changed: %q", b)
	}
	if head := gitRun(t, repo.root, "rev-parse", "HEAD"); head != baseSHA {
		t.Errorf("HEAD moved to %s despite the refusal", head)
	}
	if out := gitRun(t, repo.root, "status", "--porcelain", "--untracked-files=no"); out != "" {
		t.Errorf("repository not restored after the refusal:\n%s", out)
	}
}

// readCommittedFiles lists the export relative to its own directory when
// it is a subdirectory of the repository, and nothing outside it.
func TestReadCommittedFilesInSubdirectory(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte(s), 0o644)
	}
	write("catalog/projects/server_script/a.json", "{}")
	write("catalog/.weg/cache.json", "{}")
	write("other/projects/server_script/b.json", "{}")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")

	files, err := readCommittedFiles(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files["projects/server_script/a.json"] == nil {
		t.Errorf("files = %v, want only projects/server_script/a.json", files)
	}
}

func TestWriteFileContentRefusesEscapingPath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "catalog")
	for _, p := range []string{"../x.json", "/tmp/x.json", "a/../../x.json", ".git/x.json", ""} {
		if err := writeFileContent(dir, p, map[string]any{"a": 1}); err == nil {
			t.Errorf("writeFileContent(%q) succeeded", p)
		}
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("wrote %v", entries)
	}
}

// restoreExport undoes a failed replay at the repository root and in a
// subdirectory export (weg -C catalog): a file the replay added is removed
// from the index and the working tree, an edited file gets its committed
// content back, and nothing is left staged.
func TestRestoreExportRemovesReplayedFiles(t *testing.T) {
	for _, sub := range []string{"", "catalog"} {
		t.Run("sub="+sub, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, sub)
			write := func(p, s string) {
				os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
				os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644)
			}
			gitRun(t, root, "init", "-q")
			write("m/server_script/old.json", "{}\n")
			gitRun(t, root, "add", "-A")
			gitRun(t, root, "commit", "-q", "-m", "base")
			startHead := gitRun(t, root, "rev-parse", "HEAD")

			// A replayed commit adds a file and edits another.
			write("m/server_script/new.json", "{\"a\": 1}\n")
			write("m/server_script/old.json", "{\"b\": 2}\n")
			gitRun(t, root, "add", "-A")
			gitRun(t, root, "commit", "-q", "-m", "replayed")

			touched := map[string]bool{"m/server_script/new.json": true, "m/server_script/old.json": true}
			if err := restoreExport(dir, startHead, touched); err != nil {
				t.Fatalf("restoreExport: %v", err)
			}
			if head := gitRun(t, root, "rev-parse", "HEAD"); head != startHead {
				t.Errorf("HEAD = %s, want %s", head, startHead)
			}
			if _, err := os.Stat(filepath.Join(dir, "m/server_script/new.json")); !os.IsNotExist(err) {
				t.Errorf("new.json still on disk (err %v)", err)
			}
			if b, _ := os.ReadFile(filepath.Join(dir, "m/server_script/old.json")); string(b) != "{}\n" {
				t.Errorf("old.json = %q, want the committed content", b)
			}
			if out := gitRun(t, root, "status", "--porcelain"); out != "" {
				t.Errorf("repository not clean after restore:\n%s", out)
			}
		})
	}
}
