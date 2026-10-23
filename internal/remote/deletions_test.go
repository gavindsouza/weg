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

// planSite answers GetDoc from a table keyed "<doctype>/<name>": a status
// and a body. Anything else is a 404, as for a record the site does not have.
func planSite(t *testing.T, answers map[string]struct {
	status int
	body   string
}) *Fetcher {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/api/resource/")
		a, ok := answers[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"exc_type": "DoesNotExistError"}`)
			return
		}
		w.WriteHeader(a.status)
		fmt.Fprint(w, a.body)
	}))
	t.Cleanup(srv.Close)
	cfg := NewSiteConfig(srv.URL, "test")
	cfg.Modules["Selling"] = ModuleInfo{App: "erpnext", Sync: false}
	return NewFetcher(NewClient(srv.URL, "k", "s"), cfg)
}

func docJSON(kv ...any) string {
	b, _ := json.Marshal(map[string]any{"data": docJSONMap(kv...)})
	return string(b)
}

// A committed file the fetch does not produce goes only when the site,
// asked for its record by name, confirms it: a 404 for a file that held
// what the site had (a push baseline, or a deletion in the window), a move
// the site's copy places at the new path, a record it no longer exports.
// Every other answer keeps the file.
func TestPlanStale(t *testing.T) {
	type answer = struct {
		status int
		body   string
	}
	ok := func(kv ...any) answer { return answer{http.StatusOK, docJSON(kv...)} }
	f := planSite(t, map[string]answer{
		"Server Script/Live":       ok("name", "Live", "script_type", "API", "module", "Custom"),
		"Server Script/Forbidden":  {http.StatusForbidden, `{"exc_type": "PermissionError"}`},
		"Server Script/Throttled":  {http.StatusTooManyRequests, `{}`},
		"Server Script/Broken":     {http.StatusInternalServerError, `{"exc": "boom"}`},
		"Server Script/Garbled":    {http.StatusOK, `<html>maintenance</html>`},
		"DocType/Thing":            ok("name", "Thing", "module", "Custom", "custom", 0),
		"DocType/Widget":           ok("name", "Widget", "module", "Custom", "custom", 1),
		"Report/Hours":             ok("name", "Hours", "ref_doctype", "Teamplan Request", "is_standard", "Yes"),
		"Print Format/Slip":        ok("name", "Slip", "doc_type", "Teamplan Request", "standard", "No"),
		"Workflow/Flow":            ok("name", "Flow", "document_type", "Teamplan Request"),
		"Client Script/Lost":       ok("name", "Lost", "dt", "Lost Type"),
		"Client Script/Kept":       ok("name", "Kept", "dt", "Sales Order"),
		"DocType/Teamplan Request": ok("name", "Teamplan Request", "module", "Workplace"),
		"Custom Field/Lead-live":   ok("name", "Lead-live", "dt", "Lead"),
		"Custom Field/Lead-sys":    ok("name", "Lead-sys", "dt", "Lead", "is_system_generated", 1),
		"Custom Field/Lead-slow":   {http.StatusBadGateway, `{}`},
	})
	doc := func(kv ...any) []byte { b, _ := json.Marshal(docJSONMap(kv...)); return b }
	committed := map[string][]byte{
		"custom/server_script/gone.json":      doc("name", "Gone"),      // 404, pushed: goes
		"custom/server_script/live.json":      doc("name", "Live"),      // the site has it: stays
		"custom/server_script/forbidden.json": doc("name", "Forbidden"), // 403: stays
		"custom/server_script/throttled.json": doc("name", "Throttled"), // 429: stays
		"custom/server_script/broken.json":    doc("name", "Broken"),    // 500: stays
		"custom/server_script/garbled.json":   doc("name", "Garbled"),   // unparsable: stays
		"custom/server_script/local.json":     doc("name", "Local"),     // 404, local work: stays
		"custom/doctype/thing/thing.json":     doc("name", "Thing"),     // no longer custom: goes
		"custom/doctype/widget/widget.json":   doc("name", "Widget"),    // still custom: stays
		"custom/report/hours/hours.json":      doc("name", "Hours"),     // standard now: goes
		"custom/print_format/slip.json":       doc("name", "Slip"),      // moved, confirmed: goes
		"custom/workflow/flow.json":           doc("name", "Flow"),      // moved, misplaced: stays
		"custom/client_script/lost.json":      doc("name", "Lost"),      // moved to "_": stays
		"selling/client_script/kept.json":     doc("name", "Kept"),      // sync = false, moved: stays
		"selling/client_script/old.json":      doc("name", "Old"),       // sync = false, 404: stays
		"custom/notification/alert.json":      doc("name", "Alert"),     // notifications not synced
		"custom/server_script/nested/x.json":  doc("name", "X"),         // not weg's layout
		"custom/custom_field/lead.json": doc("doctype", "Lead", "custom_fields", []any{
			map[string]any{"name": "Lead-a"}, map[string]any{"name": "Lead-live"}, map[string]any{"name": "Lead-sys"},
			map[string]any{"name": "Lead-gone"}, map[string]any{"name": "Lead-slow"},
		}),
	}
	result := &FetchResult{Entities: []Entity{
		{Type: EntityPrintFormat, Name: "Slip", Module: "Workplace", FilePath: "workplace/print_format/slip.json"},
		{Type: EntityWorkflow, Name: "Flow", Module: "Projects", FilePath: "projects/workflow/flow.json"},
		{Type: EntityClientScript, Name: "Lost", Module: "_", FilePath: "_/client_script/lost.json"},
		{Type: EntityClientScript, Name: "Kept", Module: "Selling", FilePath: "custom/client_script/kept.json"},
		{Type: EntityCustomField, Name: "Lead", Module: "Custom", FilePath: "custom/custom_field/lead.json",
			Data: map[string]any{"doctype": "Lead", "custom_fields": []map[string]any{{"name": "Lead-a"}}}},
	}}
	ev := StaleEvidence{Committed: committed, HasBaseline: true, Local: map[string]bool{"custom/server_script/local.json": true}}
	plan := f.PlanStale(result, ev)

	var removed []string
	for _, r := range plan.Removals {
		removed = append(removed, r.Path+" ("+r.Reason+")")
	}
	want := []string{
		"custom/doctype/thing/thing.json (no longer exported (made standard))",
		"custom/print_format/slip.json (moved to workplace/print_format/slip.json)",
		"custom/report/hours/hours.json (no longer exported (made standard))",
		"custom/server_script/gone.json (deleted)",
	}
	if strings.Join(removed, "\n") != strings.Join(want, "\n") {
		t.Errorf("removals:\n%s\nwant\n%s", strings.Join(removed, "\n"), strings.Join(want, "\n"))
	}
	kept := strings.Join(plan.Kept, "\n")
	for _, p := range []string{"live", "forbidden", "throttled", "broken", "garbled", "local"} {
		if !strings.Contains(kept, "custom/server_script/"+p+".json: ") {
			t.Errorf("no note for the kept custom/server_script/%s.json:\n%s", p, kept)
		}
	}
	for _, p := range []string{"custom/workflow/flow.json", "custom/client_script/lost.json", "selling/client_script/kept.json", "selling/client_script/old.json", "custom/doctype/widget/widget.json"} {
		if !strings.Contains(kept, p+": ") {
			t.Errorf("no note for the kept %s:\n%s", p, kept)
		}
	}
	// Rows the site has or could not be asked about stay in the file.
	var rows []string
	for _, r := range rowList(result.Entities[4].Data["custom_fields"]) {
		rows = append(rows, getString(r, "name"))
	}
	if strings.Join(rows, ",") != "Lead-a,Lead-live,Lead-slow" {
		t.Errorf("Lead rows = %v, want Lead-a,Lead-live,Lead-slow", rows)
	}

	// Without a push baseline a 404 alone removes nothing; a deletion the
	// site recorded inside the window still does.
	ev = StaleEvidence{Committed: map[string][]byte{
		"custom/server_script/gone.json":    doc("name", "Gone"),
		"custom/server_script/dropped.json": doc("name", "Dropped"),
	}, Deletions: []VersionRecord{{Name: "d1", RefDoctype: "Server Script", Docname: "Dropped", Owner: "a@example.com", Creation: "2026-10-20 10:00:00.000000", Deleted: true}}}
	plan = f.PlanStale(&FetchResult{}, ev)
	if len(plan.Removals) != 1 || plan.Removals[0].Path != "custom/server_script/dropped.json" || !strings.Contains(plan.Removals[0].Reason, "a@example.com") {
		t.Errorf("removals without a baseline = %+v, want only dropped.json, deleted by a@example.com", plan.Removals)
	}

	// A failed module lookup, an incomplete fetch or an unknown baseline
	// hold everything.
	for _, r := range []*FetchResult{
		{ModuleLookupFailed: []string{"Lead (502)"}},
		{Incomplete: map[EntityType][]string{EntityServerScript: {"X (502)"}}},
	} {
		if plan := f.PlanStale(r, StaleEvidence{Committed: map[string][]byte{"custom/server_script/gone.json": doc("name", "Gone")}, HasBaseline: true}); len(plan.Removals) != 0 {
			t.Errorf("removals = %+v with %+v", plan.Removals, r)
		}
	}
	if plan := f.PlanStale(&FetchResult{}, StaleEvidence{Committed: map[string][]byte{"custom/server_script/gone.json": doc("name", "Gone")}, HasBaseline: true, Hold: "unknown baseline"}); len(plan.Removals) != 0 {
		t.Errorf("removals = %+v with a hold", plan.Removals)
	}
}

// A deletion in the window is replayed only for a record the site confirms
// is gone, or exports again; one the site still returns (restored, created
// again, missed by the fetch) or cannot be asked about is left out.
func TestPlanStaleGatesDeletions(t *testing.T) {
	f := planSite(t, map[string]struct {
		status int
		body   string
	}{
		"Server Script/Restored": {http.StatusOK, docJSON("name", "Restored", "script_type", "API")},
		"Server Script/Flaky":    {http.StatusBadGateway, `{}`},
		"Custom Field/Lead-back": {http.StatusOK, docJSON("name", "Lead-back", "dt", "Lead")},
	})
	doc := func(kv ...any) []byte { b, _ := json.Marshal(docJSONMap(kv...)); return b }
	committed := map[string][]byte{
		"custom/server_script/gone.json":     doc("name", "Gone"),
		"custom/server_script/restored.json": doc("name", "Restored"),
		"custom/server_script/flaky.json":    doc("name", "Flaky"),
		"custom/server_script/again.json":    doc("name", "Again"),
		"custom/custom_field/lead.json": doc("doctype", "Lead", "custom_fields", []any{
			map[string]any{"name": "Lead-back"}, map[string]any{"name": "Lead-gone"},
		}),
	}
	del := func(doctype, name string) VersionRecord {
		return VersionRecord{Name: "d-" + name, RefDoctype: doctype, Docname: name, Owner: "a@example.com", Creation: "2026-10-20 10:00:00.000000", Deleted: true}
	}
	deletions := []VersionRecord{
		del("Server Script", "Gone"), del("Server Script", "Restored"), del("Server Script", "Flaky"),
		del("Server Script", "Again"), del("Server Script", "Never"),
		del("Custom Field", "Lead-back"), del("Custom Field", "Lead-gone"),
	}
	result := &FetchResult{Entities: []Entity{{Type: EntityServerScript, Name: "Again", FilePath: "custom/server_script/again.json"}}}
	plan := f.PlanStale(result, StaleEvidence{Committed: committed, Deletions: deletions})
	var replayed []string
	for _, d := range plan.Deletions {
		replayed = append(replayed, d.Docname)
	}
	if got := strings.Join(replayed, ","); got != "Gone,Again,Never,Lead-gone" {
		t.Errorf("replayed deletions = %s, want Gone,Again,Never,Lead-gone", got)
	}
	if len(plan.Skipped) != 3 {
		t.Errorf("skipped = %v, want Restored, Flaky and Lead-back", plan.Skipped)
	}
}

func docJSONMap(kv ...any) map[string]any {
	doc := map[string]any{}
	for i := 0; i < len(kv); i += 2 {
		doc[kv[i].(string)] = kv[i+1]
	}
	return doc
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
