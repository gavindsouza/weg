package remote

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Fixtures shared by the pull-mode history tests. The site runs in
// Europe/Berlin; the previous pull synced at 05:30:35.288Z (07:30:35 Berlin).

var (
	pullSince = time.Date(2026, 10, 20, 5, 30, 35, 288949976, time.UTC)
	pullUntil = time.Date(2026, 10, 21, 5, 17, 30, 0, time.UTC)
)

func berlinWindow(t *testing.T) HistoryWindow {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return HistoryWindow{Since: pullSince, Until: pullUntil, Location: loc}
}

// exported formats data exactly as WriteEntity does.
func exported(t *testing.T, data map[string]any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func clone(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	return deepCopyMap(m)
}

func with(t *testing.T, m map[string]any, kv ...any) map[string]any {
	t.Helper()
	out := clone(t, m)
	for i := 0; i < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

func runHistory(t *testing.T, f *Fetcher, tmpDir string, custom map[string]bool, entities []Entity, opts HistoryOptions) ([]ReconstructedCommit, *HistoryResult) {
	t.Helper()
	var commits []ReconstructedCommit
	res, err := f.StreamHistory(tmpDir, custom, entities, nil, opts, func(c ReconstructedCommit) error {
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamHistory: %v", err)
	}
	return commits, res
}

func assertContent(t *testing.T, label string, got, want map[string]any) {
	t.Helper()
	g, w := exported(t, got), exported(t, want)
	if !bytes.Equal(g, w) {
		t.Errorf("%s content:\n%s\nwant:\n%s", label, g, w)
	}
}

// Bug 1: an incremental pull used to replay each Version onto an empty
// document, so the commit held only the changed fields. Replay now starts
// from the committed file: an earlier change in the window is that file
// with the change applied (byte-identical elsewhere), the last change is the
// document as the site has it now, and Versions the previous pull covered
// are not replayed again.
func TestStreamHistory_PullStartsFromCommittedFile(t *testing.T) {
	f := newTestFetcher(t)
	tmpDir := t.TempDir()
	window := berlinWindow(t)

	base := map[string]any{
		"doctype": "Server Script", "name": "A", "module": "Projects",
		"script_type": "API", "api_method": "a", "allow_guest": 0,
		"disabled": 1, "script": "a1\nreturn 1", "modified": "2026-10-01 10:00:00.000000",
		"owner": "admin@example.com", "creation": "2026-01-01 09:00:00.000000",
		"description": "Uses <b> &   so Go escaping matters", "rate_limit_count": 5,
	}
	current := with(t, base, "disabled", 0, "script", "a3", "modified", "2026-10-20 12:00:01.000000")
	path := "projects/server_script/a.json"
	entities := []Entity{{Type: EntityServerScript, Name: "A", Module: "Projects", Data: current, FilePath: path}}

	writeVersionJSONL(t, tmpDir, "Server Script",
		VersionRecord{Name: "v0", RefDoctype: "Server Script", Docname: "A", Owner: "zed@example.com",
			Creation: "2026-10-20 06:50:00.000000", Data: `{"changed":[["disabled",1,0]]}`},
		VersionRecord{Name: "v1", RefDoctype: "Server Script", Docname: "A", Owner: "alice@example.com",
			Creation: "2026-10-20 09:00:00.000000", Data: `{"changed":[["script","a1\nreturn 1","a2"]]}`},
		VersionRecord{Name: "v2", RefDoctype: "Server Script", Docname: "A", Owner: "bob@example.com",
			Creation: "2026-10-20 12:00:00.000000", Data: `{"changed":[["script","a2","a3"],["disabled",1,0]]}`},
	)

	commits, res := runHistory(t, f, tmpDir, nil, entities, HistoryOptions{
		Window:   window,
		Baseline: map[string][]byte{path: exported(t, base)},
	})

	if res.OutsideWindow != 1 {
		t.Errorf("OutsideWindow = %d, want 1 (v0 is before last_sync in site time)", res.OutsideWindow)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	for i, c := range commits {
		if c.FilePath != path {
			t.Errorf("commit %d path = %s, want %s", i, c.FilePath, path)
		}
	}

	// Earlier change: the committed file plus the change, nothing else.
	assertContent(t, "v1", commits[0].Content, with(t, base, "script", "a2"))
	wantBytes := bytes.Replace(exported(t, base), []byte(`"a1\nreturn 1"`), []byte(`"a2"`), 1)
	if got := exported(t, commits[0].Content); !bytes.Equal(got, wantBytes) {
		t.Errorf("v1 is not byte-identical to the committed file outside the change:\n%s", got)
	}
	// Last change: the document as it is now.
	assertContent(t, "v2", commits[1].Content, current)

	if got := commits[0].When.Format(time.RFC3339); got != "2026-10-20T09:00:00+02:00" {
		t.Errorf("v1 date = %s, want site time with +02:00", got)
	}
	if !strings.Contains(commits[1].Author, "<bob@example.com>") {
		t.Errorf("v2 author = %s", commits[1].Author)
	}
}

// Bug 2: Custom Field / Property Setter Versions are named after the row, but
// the export holds one file per target DocType. A change must update its row
// in that file (never a phantom _/custom_field/<row>.json), and Versions of
// rows outside the export (system generated) are dropped.
func TestStreamHistory_RowVersionsUpdateDocTypeFile(t *testing.T) {
	tests := []struct {
		name     string
		baseline bool
	}{
		{"pull", true},
		{"clone", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestFetcher(t)
			tmpDir := t.TempDir()
			window := berlinWindow(t)
			if !tt.baseline {
				window = HistoryWindow{Location: window.Location}
			}

			rowX := map[string]any{"name": "Subscription-x", "dt": "Subscription", "fieldname": "x", "hidden": 0, "modified": "2026-07-15 15:37:46.000000"}
			rowY := map[string]any{"name": "Subscription-y", "dt": "Subscription", "fieldname": "y", "hidden": 0, "modified": "2026-07-15 15:37:47.000000"}
			ps := map[string]any{"name": "Purchase Invoice-due_date-hidden", "doc_type": "Purchase Invoice", "field_name": "due_date", "property": "hidden", "value": "0"}

			baseCF := map[string]any{"doctype": "Subscription", "custom_fields": []any{rowX, rowY}}
			basePS := map[string]any{"doctype": "Purchase Invoice", "property_setters": []any{ps}}
			// Current state in the shape fetchCustomFields/fetchPropertySetters build.
			curCF := map[string]any{"doctype": "Subscription", "custom_fields": []map[string]any{
				with(t, rowX, "hidden", 1, "modified", "2026-10-20 11:00:00.000000"), rowY}}
			curPS := map[string]any{"doctype": "Purchase Invoice", "property_setters": []map[string]any{with(t, ps, "value", "1")}}
			cfPath, psPath := "accounts/custom_field/subscription.json", "accounts/property_setter/purchase_invoice.json"
			entities := []Entity{
				{Type: EntityCustomField, Name: "Subscription", Module: "Accounts", Data: curCF, FilePath: cfPath},
				{Type: EntityPropertySetter, Name: "Purchase Invoice", Module: "Accounts", Data: curPS, FilePath: psPath},
			}

			writeVersionJSONL(t, tmpDir, "Custom Field",
				VersionRecord{Name: "c1", RefDoctype: "Custom Field", Docname: "Subscription-x", Owner: "carol@example.com",
					Creation: "2026-10-20 11:00:00.000000", Data: `{"changed":[["hidden",0,1]]}`},
				VersionRecord{Name: "c2", RefDoctype: "Custom Field", Docname: "Sales Invoice-erpnext_system_field", Owner: "admin@example.com",
					Creation: "2026-10-20 11:30:00.000000", Data: `{"changed":[["insert_after","a","b"]]}`},
			)
			writeVersionJSONL(t, tmpDir, "Property Setter",
				VersionRecord{Name: "p1", RefDoctype: "Property Setter", Docname: "Purchase Invoice-due_date-hidden", Owner: "carol@example.com",
					Creation: "2026-10-20 11:05:00.000000", Data: `{"changed":[["value","0","1"]]}`},
			)

			opts := HistoryOptions{Window: window}
			if tt.baseline {
				opts.Baseline = map[string][]byte{cfPath: exported(t, baseCF), psPath: exported(t, basePS)}
			}
			commits, res := runHistory(t, f, tmpDir, nil, entities, opts)

			if len(commits) != 2 {
				t.Fatalf("got %d commits, want 2: %+v", len(commits), commits)
			}
			for p := range res.SeenPaths {
				if strings.HasPrefix(p, "_/") {
					t.Errorf("phantom file %s", p)
				}
			}
			if commits[0].FilePath != cfPath || commits[1].FilePath != psPath {
				t.Errorf("paths = %s, %s", commits[0].FilePath, commits[1].FilePath)
			}
			if tt.baseline {
				assertContent(t, "custom field", commits[0].Content, curCF)
			} else {
				// A clone has no committed file: the row starts the file.
				assertContent(t, "custom field", commits[0].Content,
					map[string]any{"doctype": "Subscription", "custom_fields": []any{curCF["custom_fields"].([]map[string]any)[0]}})
			}
			assertContent(t, "property setter", commits[1].Content, curPS)
			if !strings.Contains(commits[0].Message, "Subscription") {
				t.Errorf("custom field message %q lacks its DocType scope", commits[0].Message)
			}
			if res.Unexported != 1 || len(res.Dropped) != 0 {
				t.Errorf("Unexported = %d, Dropped = %v; want the system field counted as unexported", res.Unexported, res.Dropped)
			}
		})
	}
}

// Bug 5: Frappe writes no Version on insert, so a document created in the
// window first appeared in a later editor's commit. It now gets a creation
// commit by its owner at its creation time, holding the whole document as
// it was before the window's later changes. A new Custom Field row is added
// to its DocType file the same way, in the export's row order.
func TestStreamHistory_CreatedInWindow(t *testing.T) {
	f := newTestFetcher(t)
	tmpDir := t.TempDir()
	window := berlinWindow(t)

	cs := map[string]any{
		"doctype": "Client Script", "name": "New CS", "dt": "Sales Order", "enabled": 1,
		"script": "v2", "owner": "dave@example.com", "creation": "2026-10-20 10:00:00.000000",
		"modified": "2026-10-20 11:00:00.000000", "modified_by": "erin@example.com",
	}
	rowA := map[string]any{"name": "Subscription-a", "dt": "Subscription", "fieldname": "a", "creation": "2025-01-01 00:00:00.000000"}
	rowZ := map[string]any{"name": "Subscription-m", "dt": "Subscription", "fieldname": "m", "label": "M",
		"owner": "erin@example.com", "creation": "2026-10-20 10:30:00.000000"}
	rowB := map[string]any{"name": "Subscription-z", "dt": "Subscription", "fieldname": "z", "creation": "2025-01-01 00:00:00.000000"}
	baseCF := map[string]any{"doctype": "Subscription", "custom_fields": []any{rowA, rowB}}
	curCF := map[string]any{"doctype": "Subscription", "custom_fields": []map[string]any{rowA, rowZ, rowB}}
	csPath, cfPath := "selling/client_script/new_cs.json", "accounts/custom_field/subscription.json"
	entities := []Entity{
		{Type: EntityClientScript, Name: "New CS", Module: "Selling", Data: cs, FilePath: csPath},
		{Type: EntityCustomField, Name: "Subscription", Module: "Accounts", Data: curCF, FilePath: cfPath},
	}
	writeVersionJSONL(t, tmpDir, "Client Script",
		VersionRecord{Name: "e1", RefDoctype: "Client Script", Docname: "New CS", Owner: "erin@example.com",
			Creation: "2026-10-20 11:00:00.000000", Data: `{"changed":[["script","v1","v2"]]}`},
	)

	commits, _ := runHistory(t, f, tmpDir, nil, entities, HistoryOptions{
		Window:   window,
		Baseline: map[string][]byte{cfPath: exported(t, baseCF)},
	})
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3: %+v", len(commits), commits)
	}

	created := commits[0]
	if created.FilePath != csPath || !strings.Contains(created.Author, "<dave@example.com>") ||
		!strings.HasPrefix(created.Message, "feat(") || created.VersionName != "" {
		t.Errorf("creation commit = %+v", created)
	}
	if got := created.When.Format(time.RFC3339); got != "2026-10-20T10:00:00+02:00" {
		t.Errorf("creation date = %s", got)
	}
	// As saved on insert: modified is the creation time, modified_by the owner.
	assertContent(t, "created", created.Content, with(t, cs, "script", "v1",
		"modified", "2026-10-20 10:00:00.000000", "modified_by", "dave@example.com"))

	row := commits[1]
	if row.FilePath != cfPath || !strings.Contains(row.Author, "<erin@example.com>") {
		t.Errorf("row creation commit = %+v", row)
	}
	assertContent(t, "row created", row.Content, curCF)

	assertContent(t, "edit", commits[2].Content, cs)
	if !strings.Contains(commits[2].Author, "<erin@example.com>") {
		t.Errorf("edit author = %s", commits[2].Author)
	}
}

// On a pull, a Version for a document that is neither committed nor exported
// now (created and deleted inside the window, or never exported, such as a
// standard Report) is dropped instead of committed to a file under _/.
// A document deleted in the window still gets its change, applied to its
// committed file at its committed path; the reconcile commit deletes it.
func TestStreamHistory_PullDropsUnexportedAndKeepsDeleted(t *testing.T) {
	f := newTestFetcher(t)
	tmpDir := t.TempDir()

	gone := map[string]any{"doctype": "Server Script", "name": "Gone", "script": "g1", "disabled": 0}
	gonePath := "projects/server_script/gone.json"
	writeVersionJSONL(t, tmpDir, "Server Script",
		VersionRecord{Name: "s1", RefDoctype: "Server Script", Docname: "Transient", Owner: "a@example.com",
			Creation: "2026-10-20 10:00:00", Data: `{"changed":[["script","","x"]]}`},
		VersionRecord{Name: "s2", RefDoctype: "Server Script", Docname: "Gone", Owner: "a@example.com",
			Creation: "2026-10-20 10:05:00", Data: `{"changed":[["disabled",0,1]]}`},
	)
	writeVersionJSONL(t, tmpDir, "Report",
		VersionRecord{Name: "r1", RefDoctype: "Report", Docname: "Standard Report", Owner: "a@example.com",
			Creation: "2026-10-20 10:10:00", Data: `{"changed":[["disabled",0,1]]}`},
	)

	commits, res := runHistory(t, f, tmpDir, nil, nil, HistoryOptions{
		Window:   berlinWindow(t),
		Baseline: map[string][]byte{gonePath: exported(t, gone)},
	})
	if len(commits) != 1 || commits[0].FilePath != gonePath {
		t.Fatalf("commits = %+v, want only the change to Gone", commits)
	}
	assertContent(t, "deleted doc", commits[0].Content, with(t, gone, "disabled", float64(1)))
	if res.Unexported != 2 {
		t.Errorf("Unexported = %d, want Transient and Standard Report", res.Unexported)
	}
}

// Child-table changes (row_changed, added, removed) are applied on the
// replayed document, not ignored: an earlier change in the window shows the
// edited row next to the untouched ones.
func TestStreamHistory_ChildTableChanges(t *testing.T) {
	f := newTestFetcher(t)
	tmpDir := t.TempDir()

	f1 := map[string]any{"name": "f1", "fieldname": "a", "label": "A", "idx": 1}
	f2 := map[string]any{"name": "f2", "fieldname": "b", "label": "B", "idx": 2}
	f3 := map[string]any{"name": "f3", "fieldname": "c", "label": "C", "idx": 3}
	base := map[string]any{"doctype": "DocType", "name": "Gadget", "custom": 1, "fields": []any{f1, f2}}
	current := map[string]any{"doctype": "DocType", "name": "Gadget", "custom": 1,
		"fields": []any{with(t, f1, "label", "AA"), f3}}
	path := "custom/doctype/gadget/gadget.json"

	writeVersionJSONL(t, tmpDir, "DocType",
		VersionRecord{Name: "d1", RefDoctype: "DocType", Docname: "Gadget", Owner: "a@example.com",
			Creation: "2026-10-20 10:00:00", Data: `{"row_changed":[["fields",0,"f1",[["label","A","AA"]]]]}`},
		VersionRecord{Name: "d2", RefDoctype: "DocType", Docname: "Gadget", Owner: "a@example.com",
			Creation: "2026-10-20 10:01:00", Data: `{"added":[["fields",{"name":"f3","fieldname":"c","label":"C","idx":3}]]}`},
		VersionRecord{Name: "d3", RefDoctype: "DocType", Docname: "Gadget", Owner: "a@example.com",
			Creation: "2026-10-20 10:02:00", Data: `{"removed":[["fields",{"name":"f2","fieldname":"b","label":"B","idx":2}]]}`},
	)
	entities := []Entity{{Type: EntityDocType, Name: "Gadget", Module: "Custom", Data: current, FilePath: path}}
	commits, _ := runHistory(t, f, tmpDir, map[string]bool{"Gadget": true}, entities, HistoryOptions{
		Window:   berlinWindow(t),
		Baseline: map[string][]byte{path: exported(t, base)},
	})
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3", len(commits))
	}
	assertContent(t, "row_changed", commits[0].Content,
		with(t, base, "fields", []any{with(t, f1, "label", "AA"), f2}))
	assertContent(t, "added", commits[1].Content,
		with(t, base, "fields", []any{with(t, f1, "label", "AA"), f2, f3}))
	assertContent(t, "removed (last)", commits[2].Content, current)
}

// A document exported now but missing from the committed export, and created
// before the window (for example a Report that stopped being standard), is
// replayed from its current state with the window's changes undone, so its
// first commit already holds the whole document.
func TestStreamHistory_UncommittedDocRebuiltBackwards(t *testing.T) {
	f := newTestFetcher(t)
	tmpDir := t.TempDir()

	current := map[string]any{"doctype": "Server Script", "name": "Old", "script": "s3", "disabled": 0,
		"creation": "2025-01-01 00:00:00.000000"}
	path := "projects/server_script/old.json"
	writeVersionJSONL(t, tmpDir, "Server Script",
		VersionRecord{Name: "o1", RefDoctype: "Server Script", Docname: "Old", Owner: "a@example.com",
			Creation: "2026-10-20 10:00:00", Data: `{"changed":[["script","s1","s2"]]}`},
		VersionRecord{Name: "o2", RefDoctype: "Server Script", Docname: "Old", Owner: "b@example.com",
			Creation: "2026-10-20 11:00:00", Data: `{"changed":[["script","s2","s3"]]}`},
	)
	entities := []Entity{{Type: EntityServerScript, Name: "Old", Module: "Projects", Data: current, FilePath: path}}
	commits, _ := runHistory(t, f, tmpDir, nil, entities, HistoryOptions{
		Window:   berlinWindow(t),
		Baseline: map[string][]byte{},
	})
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2 (no creation commit: created before the window)", len(commits))
	}
	assertContent(t, "o1", commits[0].Content, with(t, current, "script", "s2"))
	assertContent(t, "o2", commits[1].Content, current)
}

// entityTypeOfPath recognises only the export layout, so unrelated JSON
// files in the repository never seed a replay.
func TestEntityTypeOfPath(t *testing.T) {
	tests := []struct {
		path string
		want EntityType
		ok   bool
	}{
		{"projects/server_script/a.json", EntityServerScript, true},
		{"accounts/custom_field/subscription.json", EntityCustomField, true},
		{"custom/doctype/gadget/gadget.json", EntityDocType, true},
		{"selling/report/sales/sales.json", EntityReport, true},
		{"custom/doctype/gadget/other.json", "", false},
		{"custom/doctype/gadget.json", "", false},
		{".weg/server_script/a.json", "", false},
		{"projects/server_script/a.py", "", false},
		{"projects/unknown_type/a.json", "", false},
		{"apps.json", "", false},
	}
	for _, tt := range tests {
		got, ok := entityTypeOfPath(tt.path)
		if got != tt.want || ok != tt.ok {
			t.Errorf("entityTypeOfPath(%q) = %q, %v; want %q, %v", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}
