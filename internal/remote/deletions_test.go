/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Deleted Document rows are filtered like Versions: by the enabled entity
// doctypes and by creation in site time, with microseconds kept.
func TestBuildDeletionFilters(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	w := HistoryWindow{
		Since:    time.Date(2026, 10, 20, 5, 30, 35, 288949000, time.UTC),
		Until:    time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC),
		Location: berlin,
	}
	got := marshalPlain(buildDeletionFilters([]string{"Server Script", "Custom Field"}, w))
	want := `[["Deleted Document","deleted_doctype","in",["Server Script","Custom Field"]],` +
		`["Deleted Document","creation",">","2026-10-20 07:30:35.288949"],` +
		`["Deleted Document","creation","<=","2026-10-21 07:00:00.000000"]]`
	if string(got) != want {
		t.Errorf("filters =\n%s\nwant\n%s", got, want)
	}
	clone := marshalPlain(buildDeletionFilters([]string{"Server Script"}, HistoryWindow{}))
	if string(clone) != `[["Deleted Document","deleted_doctype","in",["Server Script"]]]` {
		t.Errorf("clone filters = %s", clone)
	}
}

// marshalPlain is json.Marshal without HTML escaping, for readable wants.
func marshalPlain(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
	return strings.TrimSpace(b.String())
}

// FetchDeletions pages through the trash and returns the rows as deletion
// records, oldest first.
func TestFetchDeletionsPages(t *testing.T) {
	const total = 230
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/resource/Deleted Document" {
			t.Errorf("unexpected request %s", r.URL)
		}
		requests++
		q := r.URL.Query()
		if q.Get("order_by") != "creation asc, name asc" {
			t.Errorf("order_by = %q", q.Get("order_by"))
		}
		start, _ := strconv.Atoi(q.Get("limit_start"))
		size, _ := strconv.Atoi(q.Get("limit_page_length"))
		var rows []map[string]string
		for i := start; i < total && i < start+size; i++ {
			rows = append(rows, map[string]string{"name": fmt.Sprintf("d%03d", i), "deleted_doctype": "Server Script",
				"deleted_name": fmt.Sprintf("S%d", i), "owner": "a@example.com",
				"creation": fmt.Sprintf("2026-10-20 10:%02d:%02d.000000", i/60, i%60)})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": rows})
	}))
	defer srv.Close()

	cfg := NewSiteConfig(srv.URL, "test")
	f := NewFetcher(NewClient(srv.URL, "k", "s"), cfg)
	recs, err := f.FetchDeletions(HistoryWindow{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != total || requests != 3 {
		t.Fatalf("got %d records in %d requests", len(recs), requests)
	}
	if r := recs[0]; !r.Deleted || r.RefDoctype != "Server Script" || r.Docname != "S0" || r.Owner != "a@example.com" {
		t.Errorf("first record = %+v", r)
	}
	for i := 1; i < len(recs); i++ {
		if lessVersion(recs[i], recs[i-1]) {
			t.Fatalf("records out of order at %d", i)
		}
	}
}

// Only files weg lays out, of synced entity types and modules, that the
// fetch no longer produces are stale.
func TestStaleExportFiles(t *testing.T) {
	cfg := NewSiteConfig("https://example.com", "test")
	cfg.Modules["Selling"] = ModuleInfo{App: "erpnext", Sync: false}
	cfg.Modules["Workplace"] = ModuleInfo{App: "operations", Sync: true}
	committed := []string{
		"custom/workflow/flow.json",               // moved to Workplace: stale
		"workplace/workflow/flow.json",            // exported now
		"custom/doctype/thing/thing.json",         // no longer custom: stale
		"custom/doctype/thing/thing.js",           // not an export file
		"_/server_script/orphan.json",             // deleted: stale
		"projects/server_script/calc.json",        // module not in [modules]: stale
		"custom/notification/alert.json",          // notifications are not synced
		"selling/client_script/old.json",          // module sync = false
		"custom/README.md",                        // not an export file
		"custom/server_script/nested/x.json",      // not weg's layout
		"modules.txt",                             // not an export file
		"weg_workspace/custom/server_script/a.py", // the workspace
	}
	result := &FetchResult{Entities: []Entity{{Type: EntityWorkflow, Name: "Flow", Module: "Workplace", FilePath: "workplace/workflow/flow.json"}}}
	got := StaleExportFiles(committed, result, cfg)
	want := []string{
		"_/server_script/orphan.json",
		"custom/doctype/thing/thing.json",
		"custom/workflow/flow.json",
		"projects/server_script/calc.json",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("stale = %v\nwant    %v", got, want)
	}

	// A Server Script the fetch could not read may be any of them.
	result.Incomplete = map[EntityType][]string{EntityServerScript: {"Calc (502)"}}
	for _, p := range StaleExportFiles(committed, result, cfg) {
		if strings.Contains(p, "/server_script/") {
			t.Errorf("%s is stale although the server script fetch was incomplete", p)
		}
	}

	// Without a DocType's module, files were placed under "_": the layout
	// says nothing about what the site has.
	if got := StaleExportFiles(committed, &FetchResult{Entities: result.Entities, ModuleLookupFailed: []string{"Lead (502)"}}, cfg); len(got) != 0 {
		t.Errorf("stale = %v with a failed module lookup, want none", got)
	}

	cfg.Sync.Entities.Workflow = false
	for _, p := range StaleExportFiles(committed, result, cfg) {
		if strings.Contains(p, "/workflow/") {
			t.Errorf("%s is stale with workflows not synced", p)
		}
	}
}

// A document the fetch lists but cannot read is reported as incomplete
// rather than silently missing, since a missing document would read as
// deleted.
func TestFetchAllReportsUnreadableDocuments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/resource/Server Script":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"name": "Good"}, {"name": "Flaky"}}})
		case "/api/resource/Server Script/Good":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"name": "Good", "script_type": "API"}})
		case "/api/resource/Server Script/Flaky":
			w.WriteHeader(http.StatusBadGateway)
		default:
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
		}
	}))
	defer srv.Close()

	f := NewFetcher(NewClient(srv.URL, "k", "s"), NewSiteConfig(srv.URL, "test"))
	result, err := f.FetchAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 1 || result.Entities[0].Name != "Good" {
		t.Fatalf("entities = %+v", result.Entities)
	}
	if got := result.Incomplete[EntityServerScript]; len(got) != 1 || !strings.HasPrefix(got[0], "Flaky") {
		t.Errorf("incomplete = %v, want Flaky", result.Incomplete)
	}
	if len(result.Incomplete) != 1 {
		t.Errorf("incomplete = %v", result.Incomplete)
	}
}
