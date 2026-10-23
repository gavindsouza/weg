/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/remote"
)

// fakeHistorySite serves what the history pipeline reads from Frappe: the
// System Settings time zone, Version rows (honouring the list filters,
// ordering and paging weg sends), users and the trash (Deleted Document).
type fakeHistorySite struct {
	t           *testing.T
	timeZone    string
	versions    []remote.VersionRecord
	users       map[string]string // email -> full name
	failVersion bool
	// deleted are the site's Deleted Document rows: name, deleted_doctype,
	// deleted_name, owner, creation and (optionally) data.
	deleted []map[string]string
	// deletedStatus, when set, is the status Deleted Document reads answer
	// with (403: the API user may not read the trash).
	deletedStatus int
	// timeZoneStatus, when set, is the status the System Settings read
	// answers with (403: the API user may not read System Settings).
	timeZoneStatus int
	// userTimeZone is what frappe.client.get_time_zone answers: the API
	// user's own "time_zone" default, which shadows the site's.
	userTimeZone string
	// docs are the documents the site has, keyed "<doctype>/<name>"; any
	// other document read answers 404, as for a deleted one.
	docs map[string]map[string]any
	// rows are the Custom Field and Property Setter rows the site has, per
	// doctype, for the "does the site still have rows" check.
	rows         map[string][]map[string]any
	versionCalls atomic.Int32 // Version requests come in parallel
	server       *httptest.Server
}

func newFakeHistorySite(t *testing.T) *fakeHistorySite {
	t.Helper()
	s := &fakeHistorySite{t: t, timeZone: "Europe/Berlin", users: map[string]string{}}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *fakeHistorySite) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := func(v any) { json.NewEncoder(w).Encode(v) }
	q := r.URL.Query()

	switch r.URL.Path {
	case "/api/method/frappe.client.get_single_value":
		if s.timeZoneStatus != 0 {
			w.WriteHeader(s.timeZoneStatus)
			reply(map[string]any{"exc_type": "PermissionError"})
			return
		}
		reply(map[string]any{"message": s.timeZone})

	case "/api/method/frappe.client.get_time_zone":
		reply(map[string]any{"message": map[string]any{"time_zone": s.userTimeZone}})

	case "/api/resource/Version":
		s.versionCalls.Add(1)
		if s.failVersion {
			w.WriteHeader(http.StatusInternalServerError)
			reply(map[string]any{"exc": "boom"})
			return
		}
		var filters [][]any
		if err := json.Unmarshal([]byte(q.Get("filters")), &filters); err != nil {
			s.t.Errorf("Version filters are not a filter list: %v", err)
		}
		var rows []remote.VersionRecord
		for _, v := range s.versions {
			if matchesFilters(v, filters) {
				rows = append(rows, v)
			}
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].Creation != rows[j].Creation {
				return rows[i].Creation < rows[j].Creation
			}
			return rows[i].Name < rows[j].Name
		})
		start, _ := strconv.Atoi(q.Get("limit_start"))
		size, _ := strconv.Atoi(q.Get("limit_page_length"))
		if start > len(rows) {
			start = len(rows)
		}
		end := min(start+size, len(rows))
		reply(map[string]any{"data": rows[start:end]})

	case "/api/resource/User":
		var data []map[string]any
		for email, name := range s.users {
			data = append(data, map[string]any{"name": email, "email": email, "full_name": name})
		}
		reply(map[string]any{"data": data})

	case "/api/resource/Deleted Document":
		if s.deletedStatus != 0 {
			w.WriteHeader(s.deletedStatus)
			reply(map[string]any{"exc_type": "PermissionError"})
			return
		}
		reply(map[string]any{"data": s.deletedRows(q)})

	case "/api/resource/Custom Field", "/api/resource/Property Setter":
		doctype := strings.TrimPrefix(r.URL.Path, "/api/resource/")
		var filters map[string]any
		json.Unmarshal([]byte(q.Get("filters")), &filters)
		var data []map[string]any
		for _, row := range s.rows[doctype] {
			if (filters["dt"] == nil || row["dt"] == filters["dt"]) && (filters["doc_type"] == nil || row["doc_type"] == filters["doc_type"]) {
				data = append(data, row)
			}
		}
		reply(map[string]any{"data": data})

	default:
		rest, ok := strings.CutPrefix(r.URL.Path, "/api/resource/")
		if doctype, name, isDoc := strings.Cut(rest, "/"); ok && isDoc && r.Method == http.MethodGet {
			if doc, found := s.docs[doctype+"/"+name]; found {
				reply(map[string]any{"data": doc})
				return
			}
			w.WriteHeader(http.StatusNotFound)
			reply(map[string]any{"exc_type": "DoesNotExistError"})
			return
		}
		s.t.Errorf("unexpected request %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusNotFound)
	}
}

// deletedRows answers a Deleted Document list request: a filter list (the
// replay: deleted_doctype "in", creation bounds; ordered and paged) or a
// filter dict (the custom DocType check: deleted_doctype only).
func (s *fakeHistorySite) deletedRows(q url.Values) []map[string]string {
	var list [][]any
	var dict map[string]any
	if err := json.Unmarshal([]byte(q.Get("filters")), &list); err != nil {
		if err := json.Unmarshal([]byte(q.Get("filters")), &dict); err != nil {
			s.t.Errorf("Deleted Document filters: %v", err)
		}
		var rows []map[string]string
		for _, d := range s.deleted {
			if dict["deleted_doctype"] == nil || d["deleted_doctype"] == dict["deleted_doctype"] {
				rows = append(rows, d)
			}
		}
		return rows
	}
	var rows []map[string]string
	for _, d := range s.deleted {
		ok := true
		for _, f := range list {
			field, op := f[1].(string), f[2].(string)
			got := d[field]
			switch op {
			case "in":
				in := false
				for _, v := range f[3].([]any) {
					in = in || v == got
				}
				ok = ok && in
			case "=":
				ok = ok && got == f[3].(string)
			case ">":
				ok = ok && got > f[3].(string)
			case "<=":
				ok = ok && got <= f[3].(string)
			default:
				s.t.Errorf("unexpected Deleted Document filter %v", f)
			}
		}
		if ok {
			rows = append(rows, d)
		}
	}
	desc := strings.HasPrefix(q.Get("order_by"), "creation desc")
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if desc {
			a, b = b, a
		}
		if a["creation"] != b["creation"] {
			return a["creation"] < b["creation"]
		}
		return a["name"] < b["name"]
	})
	start, _ := strconv.Atoi(q.Get("limit_start"))
	size, _ := strconv.Atoi(q.Get("limit_page_length"))
	if start > len(rows) {
		start = len(rows)
	}
	return rows[start:min(start+size, len(rows))]
}

// matchesFilters applies [doctype, field, op, value] filters the way the
// site would; creation values compare as fixed-width datetime strings.
func matchesFilters(v remote.VersionRecord, filters [][]any) bool {
	for _, f := range filters {
		field, op, value := f[1].(string), f[2].(string), f[3].(string)
		got := map[string]string{"ref_doctype": v.RefDoctype, "creation": v.Creation}[field]
		ok := map[string]bool{"=": got == value, ">": got > value, "<=": got <= value}[op]
		if !ok {
			return false
		}
	}
	return true
}

// historyRepo is a weg remote clone directory with one committed export.
type historyRepo struct {
	t      *testing.T
	root   string // repository root
	dir    string // the export: root, or a subdirectory of it (weg -C)
	site   *fakeHistorySite
	config *remote.SiteConfig
}

func newHistoryRepo(t *testing.T, site *fakeHistorySite, lastSync time.Time, base []remote.Entity) *historyRepo {
	t.Helper()
	return newHistoryRepoIn(t, site, lastSync, base, "")
}

// newHistoryRepoIn is newHistoryRepo with the export in the subdirectory sub
// of the repository, the layout `weg -C sub` works on.
func newHistoryRepoIn(t *testing.T, site *fakeHistorySite, lastSync time.Time, base []remote.Entity, sub string) *historyRepo {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "config", "user.name", "weg-sync[bot]")
	gitRun(t, root, "config", "user.email", "bot@example.com")
	gitRun(t, root, "config", "commit.gpgsign", "false")
	cfg := remote.NewSiteConfig(site.server.URL, "test")
	cfg.Sync.LastSync = lastSync
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
	if err := remote.EnsureGitignore(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeAllEntities(dir, base, filepath.Join(dir, "modules.txt")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "chore(sync): scheduled export")
	return &historyRepo{t: t, root: root, dir: dir, site: site, config: cfg}
}

// pull runs the history pipeline the way `weg remote pull` does.
func (r *historyRepo) pull(until time.Time, current []remote.Entity) error {
	r.t.Helper()
	return r.pullResult(until, &remote.FetchResult{Entities: current})
}

// pullResult is pull with the whole fetch result given.
func (r *historyRepo) pullResult(until time.Time, result *remote.FetchResult) error {
	r.t.Helper()
	client := remote.NewClient(r.site.server.URL, "key", "secret")
	cfg := r.config
	return reconstructHistory(context.Background(), historyRun{
		dir:           r.dir,
		siteURL:       cfg.Site.URL,
		modulesFile:   filepath.Join(r.dir, "modules.txt"),
		fetcher:       remote.NewFetcher(client, cfg),
		result:        result,
		since:         cfg.Sync.LastSync,
		until:         until,
		fromCommitted: true,
		recordSync: func() (func(), error) {
			prev := cfg.Sync.LastSync
			cfg.Sync.LastSync = until
			return func() { cfg.Sync.LastSync = prev; cfg.Save(r.dir) }, cfg.Save(r.dir)
		},
	})
}

type loggedCommit struct {
	sha, author, authorDate, committerDate, subject string
	files                                           []string
}

func (r *historyRepo) log(since string) []loggedCommit {
	r.t.Helper()
	var out []loggedCommit
	for _, sha := range strings.Fields(gitRun(r.t, r.dir, "rev-list", "--reverse", since+"..HEAD")) {
		meta := strings.Split(gitRun(r.t, r.dir, "log", "-1", "--format=%an%x00%aI%x00%cI%x00%s", sha), "\x00")
		files := strings.Fields(gitRun(r.t, r.dir, "diff-tree", "-r", "--relative", "--no-commit-id", "--name-only", sha+"^", sha))
		out = append(out, loggedCommit{sha, meta[0], meta[1], meta[2], meta[3], files})
	}
	return out
}

func (r *historyRepo) show(rev, path string) string {
	r.t.Helper()
	return gitRun(r.t, r.dir, "show", rev+":./"+path)
}

func marshalExport(t *testing.T, data map[string]any) string {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func copyDoc(m map[string]any, kv ...any) map[string]any {
	b, _ := json.Marshal(m)
	var out map[string]any
	json.Unmarshal(b, &out)
	for i := 0; i < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// End to end against a fake site in Europe/Berlin: a pull replays whole
// documents with their authors and true offsets, puts Custom Field and
// Property Setter changes into their DocType files, credits a new document
// to its creator, skips what the previous pull covered, leaves the reconcile
// commit with nothing but last_sync, and a second pull replays nothing again.
func TestReconstructHistoryPull(t *testing.T) {
	site := newFakeHistorySite(t)
	site.users = map[string]string{"alice@example.com": "Alice", "bob@example.com": "Bob", "carol@example.com": "Carol"}
	lastSync := time.Date(2026, 10, 20, 5, 30, 35, 288949976, time.UTC) // 07:30:35 Berlin
	until := time.Date(2026, 10, 21, 5, 17, 30, 0, time.UTC)

	script := map[string]any{"doctype": "Server Script", "name": "Calc", "module": "Projects",
		"script_type": "API", "api_method": "calc", "disabled": 0, "script": "x = 1\ny = 2\n",
		"modified": "2026-10-01 10:00:00.000000", "owner": "admin@example.com", "creation": "2026-01-01 09:00:00.000000"}
	rowX := map[string]any{"name": "Subscription-x", "dt": "Subscription", "fieldname": "x", "hidden": 0}
	rowY := map[string]any{"name": "Subscription-y", "dt": "Subscription", "fieldname": "y", "hidden": 0}
	ps := map[string]any{"name": "Project-status-hidden", "doc_type": "Project", "field_name": "status", "property": "hidden", "value": "0"}
	cs := map[string]any{"doctype": "Client Script", "name": "Greeter", "dt": "Project", "enabled": 1, "script": "hello()",
		"owner": "carol@example.com", "creation": "2026-10-20 13:00:00.000000"}

	scriptPath := "projects/server_script/calc.json"
	cfPath := "accounts/custom_field/subscription.json"
	psPath := "projects/property_setter/project.json"
	csPath := "projects/client_script/greeter.json"
	ent := func(et remote.EntityType, name, module, path string, data map[string]any) remote.Entity {
		return remote.Entity{Type: et, Name: name, Module: module, Data: data, FilePath: path}
	}
	base := []remote.Entity{
		ent(remote.EntityServerScript, "Calc", "Projects", scriptPath, script),
		ent(remote.EntityCustomField, "Subscription", "Accounts", cfPath, map[string]any{"doctype": "Subscription", "custom_fields": []any{rowX, rowY}}),
		ent(remote.EntityPropertySetter, "Project", "Projects", psPath, map[string]any{"doctype": "Project", "property_setters": []any{ps}}),
	}
	repo := newHistoryRepo(t, site, lastSync, base)
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")

	scriptNow := copyDoc(script, "script", "x = 3\ny = 2\n", "modified", "2026-10-20 12:00:00.500000")
	// Rows in the shape the fetcher builds them, with the site's bumped modified.
	cfNow := map[string]any{"doctype": "Subscription", "custom_fields": []map[string]any{
		copyDoc(rowX, "hidden", 1, "modified", "2026-10-20 10:00:00.100000"), rowY}}
	psNow := map[string]any{"doctype": "Project", "property_setters": []map[string]any{
		copyDoc(ps, "value", "1", "modified", "2026-10-20 11:00:00.100000")}}
	current := []remote.Entity{
		ent(remote.EntityServerScript, "Calc", "Projects", scriptPath, scriptNow),
		ent(remote.EntityCustomField, "Subscription", "Accounts", cfPath, cfNow),
		ent(remote.EntityPropertySetter, "Project", "Projects", psPath, psNow),
		ent(remote.EntityClientScript, "Greeter", "Projects", csPath, cs),
	}
	v := func(name, doctype, docname, owner, creation, data string) remote.VersionRecord {
		return remote.VersionRecord{Name: name, RefDoctype: doctype, Docname: docname, Owner: owner, Creation: creation, Data: data}
	}
	site.versions = []remote.VersionRecord{
		// Covered by the previous pull (07:30:35.288 Berlin): never replayed.
		v("v00", "Server Script", "Calc", "bob@example.com", "2026-10-20 07:30:35.100000", `{"changed":[["script","x = 0\ny = 2\n","x = 1\ny = 2\n"]]}`),
		v("v01", "Server Script", "Calc", "alice@example.com", "2026-10-20 09:00:00.000000", `{"changed":[["script","x = 1\ny = 2\n","x = 2\ny = 2\n"]]}`),
		v("v02", "Custom Field", "Subscription-x", "carol@example.com", "2026-10-20 10:00:00.000000", `{"changed":[["hidden",0,1]]}`),
		v("v03", "Custom Field", "Sales Invoice-system_field", "admin@example.com", "2026-10-20 10:30:00.000000", `{"changed":[["insert_after","a","b"]]}`),
		v("v04", "Property Setter", "Project-status-hidden", "alice@example.com", "2026-10-20 11:00:00.000000", `{"changed":[["value","0","1"]]}`),
		v("v05", "Server Script", "Calc", "bob@example.com", "2026-10-20 12:00:00.000000", `{"changed":[["script","x = 2\ny = 2\n","x = 3\ny = 2\n"]]}`),
		// After this pull's sync time: left for the next pull.
		v("v06", "Server Script", "Calc", "bob@example.com", "2026-10-21 07:17:30.000001", `{"changed":[["disabled",0,1]]}`),
	}

	if err := repo.pull(until, current); err != nil {
		t.Fatalf("pull: %v", err)
	}

	got := repo.log(baseSHA)
	want := []struct {
		author, date, subjectPrefix string
		files                       []string
	}{
		{"Alice", "2026-10-20T09:00:00+02:00", "", []string{scriptPath}},
		{"Carol", "2026-10-20T10:00:00+02:00", "chore(Subscription)", []string{cfPath}},
		{"Alice", "2026-10-20T11:00:00+02:00", "chore(Project)", []string{psPath}},
		{"Bob", "2026-10-20T12:00:00+02:00", "", []string{scriptPath}},
		{"Carol", "2026-10-20T13:00:00+02:00", "feat(", []string{csPath}},
		{"Weg", "", "chore(sync): reconcile", []string{".weg/site.toml"}},
	}
	if len(got) != len(want) {
		for _, c := range got {
			t.Logf("%s %s %s %v", c.author, c.authorDate, c.subject, c.files)
		}
		t.Fatalf("got %d commits, want %d", len(got), len(want))
	}
	for i, w := range want {
		c := got[i]
		if c.author != w.author || (w.date != "" && (c.authorDate != w.date || c.committerDate != w.date)) ||
			!strings.HasPrefix(c.subject, w.subjectPrefix) || strings.Join(c.files, ",") != strings.Join(w.files, ",") {
			t.Errorf("commit %d = %+v, want %+v", i, c, w)
		}
		for _, f := range c.files {
			if strings.HasPrefix(f, "_/") {
				t.Errorf("commit %d touches phantom file %s", i, f)
			}
		}
	}

	// Each replayed commit holds whole documents.
	if s := repo.show(got[0].sha, scriptPath); s != marshalExport(t, copyDoc(script, "script", "x = 2\ny = 2\n")) {
		t.Errorf("first script commit is not the whole document:\n%s", s)
	}
	if s := repo.show(got[3].sha, scriptPath); s != marshalExport(t, scriptNow) {
		t.Errorf("last script commit is not the current document:\n%s", s)
	}
	if s := repo.show(got[1].sha, cfPath); s != marshalExport(t, cfNow) {
		t.Errorf("custom field commit:\n%s", s)
	}
	if s := repo.show(got[2].sha, psPath); s != marshalExport(t, psNow) {
		t.Errorf("property setter commit:\n%s", s)
	}
	if s := repo.show(got[4].sha, csPath); s != marshalExport(t, cs) {
		t.Errorf("creation commit:\n%s", s)
	}
	if out := gitRun(t, repo.dir, "status", "--porcelain"); out != "" {
		t.Errorf("working tree not clean after pull:\n%s", out)
	}

	// Second pull, nothing new on the site except v06 (now inside the window).
	afterFirst := gitRun(t, repo.dir, "rev-parse", "HEAD")
	scriptLater := copyDoc(scriptNow, "disabled", 1)
	current[0].Data = scriptLater
	if err := repo.pull(until.Add(24*time.Hour), current); err != nil {
		t.Fatalf("second pull: %v", err)
	}
	second := repo.log(afterFirst)
	if len(second) != 2 || second[0].author != "Bob" || second[0].authorDate != "2026-10-21T07:17:30+02:00" ||
		second[1].author != "Weg" || strings.Join(second[1].files, ",") != ".weg/site.toml" {
		for _, c := range second {
			t.Logf("%s %s %s %v", c.author, c.authorDate, c.subject, c.files)
		}
		t.Fatalf("second pull should replay only v06, then record last_sync")
	}
}

// A failed Version fetch still leaves a usable snapshot commit, but the run
// returns an error (non-zero exit) so automation sees the missing history.
func TestReconstructHistoryPullFetchFailure(t *testing.T) {
	site := newFakeHistorySite(t)
	site.failVersion = true
	script := map[string]any{"doctype": "Server Script", "name": "Calc", "script": "x = 1"}
	path := "projects/server_script/calc.json"
	repo := newHistoryRepo(t, site, time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC),
		[]remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: script, FilePath: path}})
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")

	now := []remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects",
		Data: copyDoc(script, "script", "x = 2"), FilePath: path}}
	err := repo.pull(time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC), now)
	if err == nil || wegerrors.ExitCode(err) == 0 {
		t.Fatalf("pull error = %v, want a non-zero exit", err)
	}
	log := repo.log(baseSHA)
	if len(log) != 1 || !strings.HasPrefix(log[0].subject, "chore(sync): snapshot pull") {
		t.Fatalf("want one snapshot commit, got %+v", log)
	}
	if s := repo.show("HEAD", path); s != marshalExport(t, now[0].Data) {
		t.Errorf("snapshot does not hold the current state:\n%s", s)
	}
}

// When System Settings cannot be read, a pull stops before fetching any
// Version: guessing a zone would place the window hours off (replaying what
// the last pull covered, or skipping changes). frappe.client.get_time_zone is
// readable but answers with the API user's own zone (here Asia/Kolkata while
// the site runs Europe/Berlin), so it must not stand in for the site's.
// Nothing is committed, the working tree is untouched and last_sync stays, so
// the next run covers the window.
func TestReconstructHistoryPullUnknownTimeZone(t *testing.T) {
	site := newFakeHistorySite(t)
	site.timeZoneStatus = http.StatusForbidden
	site.userTimeZone = "Asia/Kolkata"
	script := map[string]any{"doctype": "Server Script", "name": "Calc", "script": "x = 1"}
	path := "projects/server_script/calc.json"
	repo := newHistoryRepo(t, site, time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC),
		[]remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: script, FilePath: path}})
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")
	siteToml, _ := os.ReadFile(filepath.Join(repo.dir, ".weg", "site.toml"))
	site.versions = []remote.VersionRecord{{Name: "v1", RefDoctype: "Server Script", Docname: "Calc",
		Owner: "a@example.com", Creation: "2026-10-20 06:30:00.000000", Data: `{"changed":[["script","x = 1","x = 2"]]}`}}

	now := []remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects",
		Data: copyDoc(script, "script", "x = 2"), FilePath: path}}
	err := repo.pull(time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC), now)
	if err == nil || wegerrors.ExitCode(err) == 0 || !strings.Contains(err.Error(), "time zone") {
		t.Fatalf("pull error = %v, want a non-zero exit naming the time zone", err)
	}
	if n := site.versionCalls.Load(); n != 0 {
		t.Errorf("fetched Versions %d times without knowing the site time zone", n)
	}
	if head := gitRun(t, repo.dir, "rev-parse", "HEAD"); head != baseSHA {
		t.Errorf("HEAD moved to %s despite the failure", head)
	}
	if out := gitRun(t, repo.dir, "status", "--porcelain"); out != "" {
		t.Errorf("working tree changed:\n%s", out)
	}
	if after, _ := os.ReadFile(filepath.Join(repo.dir, ".weg", "site.toml")); string(after) != string(siteToml) {
		t.Errorf("last_sync was written despite the failure")
	}
}

// A git failure while replaying stops the pull before the reconcile commit
// and before last_sync is written, instead of being ignored.
func TestReconstructHistoryPullGitFailure(t *testing.T) {
	site := newFakeHistorySite(t)
	script := map[string]any{"doctype": "Server Script", "name": "Calc", "script": "x = 1"}
	path := "projects/server_script/calc.json"
	lastSync := time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC)
	repo := newHistoryRepo(t, site, lastSync,
		[]remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: script, FilePath: path}})
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")
	siteToml, _ := os.ReadFile(filepath.Join(repo.dir, ".weg", "site.toml"))
	site.versions = []remote.VersionRecord{{Name: "v1", RefDoctype: "Server Script", Docname: "Calc",
		Owner: "a@example.com", Creation: "2026-10-20 09:00:00.000000", Data: `{"changed":[["script","x = 1","x = 2"]]}`}}

	// A stale index lock makes every git write fail.
	if err := os.WriteFile(filepath.Join(repo.dir, ".git", "index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	now := []remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects",
		Data: copyDoc(script, "script", "x = 2"), FilePath: path}}
	err := repo.pull(time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC), now)
	if err == nil || !strings.Contains(err.Error(), "index.lock") {
		t.Fatalf("pull error = %v, want the git failure", err)
	}
	os.Remove(filepath.Join(repo.dir, ".git", "index.lock"))
	if head := gitRun(t, repo.dir, "rev-parse", "HEAD"); head != baseSHA {
		t.Errorf("HEAD moved to %s despite the failure", head)
	}
	if after, _ := os.ReadFile(filepath.Join(repo.dir, ".weg", "site.toml")); string(after) != string(siteToml) {
		t.Errorf("last_sync was written despite the failure")
	}
}

// readCommittedFiles reads the export from HEAD, not the working tree, so
// local edits never leak into replayed site history.
func TestReadCommittedFiles(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	files, err := readCommittedFiles(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("unborn HEAD: %v, %v", files, err)
	}

	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644)
	}
	write("projects/server_script/a.json", "{\n  \"name\": \"A\"\n}")
	write("custom/doctype/g/g.json", "{}")
	write(".weg/site.toml", "x")
	write(".weg/cache.json", "{}")
	write("weg_workspace/a.json", "{}")
	write("notes.txt", "x")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	write("projects/server_script/a.json", "local edit")

	files, err = readCommittedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	if strings.Join(paths, ",") != "custom/doctype/g/g.json,projects/server_script/a.json" {
		t.Errorf("paths = %v", paths)
	}
	if string(files["projects/server_script/a.json"]) != "{\n  \"name\": \"A\"\n}" {
		t.Errorf("content = %q, want the committed one", files["projects/server_script/a.json"])
	}
}

func TestFormatGitDate(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	tests := []struct {
		in   time.Time
		want string
	}{
		{time.Time{}, ""},
		{time.Date(2026, 10, 20, 9, 0, 0, 0, berlin), "2026-10-20T09:00:00+02:00"},
		{time.Date(2026, 11, 2, 9, 0, 0, 0, berlin), "2026-11-02T09:00:00+01:00"},
		{time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC), "2026-11-02T09:00:00Z"},
	}
	for _, tt := range tests {
		if got := formatGitDate(tt.in); got != tt.want {
			t.Errorf("formatGitDate(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// A failure partway through the replay rolls the commits it made back, so
// a rerun starts from the same committed export and replays the window once,
// instead of replaying it again on top of its own partial result.
func TestReconstructHistoryPullRollsBackPartialReplay(t *testing.T) {
	site := newFakeHistorySite(t)
	calc := map[string]any{"doctype": "Server Script", "name": "Calc", "script": "x = 1"}
	calcPath := "projects/server_script/calc.json"
	lastSync := time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC)
	repo := newHistoryRepo(t, site, lastSync,
		[]remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: calc, FilePath: calcPath}})
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")

	newPath := "projects/server_script/fresh.json"
	fresh := map[string]any{"doctype": "Server Script", "name": "Fresh", "script": "y = 1",
		"owner": "b@example.com", "creation": "2026-10-20 10:00:00.000000"}
	now := []remote.Entity{
		{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: copyDoc(calc, "script", "x = 2"), FilePath: calcPath},
		{Type: remote.EntityServerScript, Name: "Fresh", Module: "Projects", Data: fresh, FilePath: newPath},
	}
	site.versions = []remote.VersionRecord{{Name: "v1", RefDoctype: "Server Script", Docname: "Calc",
		Owner: "a@example.com", Creation: "2026-10-20 09:00:00.000000", Data: `{"changed":[["script","x = 1","x = 2"]]}`}}

	// Something in the way of the second commit's file.
	blocker := filepath.Join(repo.dir, newPath)
	if err := os.MkdirAll(filepath.Join(blocker, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	until := time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC)
	if err := repo.pull(until, now); err == nil {
		t.Fatal("pull succeeded despite the blocked path")
	}
	if head := gitRun(t, repo.dir, "rev-parse", "HEAD"); head != baseSHA {
		t.Fatalf("HEAD = %s, want the replay rolled back to %s", head, baseSHA)
	}
	// The export is back as committed: no replayed content left in the
	// working tree or the index, last_sync unchanged.
	if out := gitRun(t, repo.dir, "status", "--porcelain", "--untracked-files=no"); out != "" {
		t.Fatalf("export not restored after the failed pull:\n%s", out)
	}

	os.RemoveAll(blocker)
	repo.config.Sync.LastSync = lastSync
	if err := repo.pull(until, now); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	var subjects []string
	for _, c := range repo.log(baseSHA) {
		subjects = append(subjects, c.author+" "+strings.Join(c.files, ","))
	}
	want := []string{"A " + calcPath, "B " + newPath, "Weg .weg/site.toml"}
	if strings.Join(subjects, "|") != strings.Join(want, "|") {
		t.Fatalf("after rerun: %v, want %v", subjects, want)
	}
}

// A replayed commit holds only its document: changes the user had staged
// are not credited to the site user.
func TestReconstructHistoryPullKeepsStagedWorkOutOfReplay(t *testing.T) {
	site := newFakeHistorySite(t)
	calc := map[string]any{"doctype": "Server Script", "name": "Calc", "script": "x = 1"}
	path := "projects/server_script/calc.json"
	repo := newHistoryRepo(t, site, time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC),
		[]remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects", Data: calc, FilePath: path}})
	baseSHA := gitRun(t, repo.dir, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(repo.dir, "notes.txt"), []byte("mine"), 0o644)
	gitRun(t, repo.dir, "add", "notes.txt")

	site.versions = []remote.VersionRecord{{Name: "v1", RefDoctype: "Server Script", Docname: "Calc",
		Owner: "a@example.com", Creation: "2026-10-20 09:00:00.000000", Data: `{"changed":[["script","x = 1","x = 2"]]}`}}
	now := []remote.Entity{{Type: remote.EntityServerScript, Name: "Calc", Module: "Projects",
		Data: copyDoc(calc, "script", "x = 2"), FilePath: path}}
	if err := repo.pull(time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC), now); err != nil {
		t.Fatal(err)
	}
	log := repo.log(baseSHA)
	if len(log) == 0 || log[0].author != "A" || strings.Join(log[0].files, ",") != path {
		t.Fatalf("replayed commit = %+v, want only %s", log, path)
	}
}

// commitFinal puts the previous sync state back when the final commit fails,
// so the next run covers the same window.
func TestCommitFinalRestoresSyncOnFailure(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, ".git", "index.lock"), nil, 0o644)

	restored := false
	run := historyRun{dir: dir, recordSync: func() (func(), error) {
		return func() { restored = true }, nil
	}}
	if err := commitFinal(run, "", "msg"); err == nil {
		t.Fatal("commitFinal succeeded with a locked index")
	}
	if !restored {
		t.Fatal("sync state was not restored")
	}
}
