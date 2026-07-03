package remote

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// newTestFetcher builds a Fetcher with a default site config and no client.
// StreamHistory and enabledVersionedDoctypes only touch f.Config.
func newTestFetcher(t *testing.T) *Fetcher {
	t.Helper()
	return &Fetcher{Config: NewSiteConfig("https://test.example", "test")}
}

// writeVersionJSONL stages version records for a doctype the way
// FetchVersionsToDisk would: one JSON object per line, oldest first.
func writeVersionJSONL(t *testing.T, tmpDir, doctype string, recs ...VersionRecord) {
	t.Helper()
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal record: %v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(versionJSONLPath(tmpDir, doctype), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// Default config ships with Notification (and Workspace) off, so the
// versioned-doctype list must exclude "Notification" — disabled entities are
// neither fetched nor replayed into history.
func TestEnabledVersionedDoctypes_DefaultExcludesNotification(t *testing.T) {
	f := newTestFetcher(t)

	got := f.enabledVersionedDoctypes()
	for _, dt := range got {
		if dt == "Notification" {
			t.Fatalf("Notification is disabled by default but present in %v", got)
		}
	}
	if len(got) != len(VersionedDoctypes())-1 {
		t.Fatalf("expected all versioned doctypes except Notification, got %v", got)
	}
}

// With every entity enabled, the filtered list matches VersionedDoctypes()
// exactly (same members, same order).
func TestEnabledVersionedDoctypes_AllEnabled(t *testing.T) {
	f := newTestFetcher(t)
	f.Config.Sync.Entities.Notification = true

	got := f.enabledVersionedDoctypes()
	want := VersionedDoctypes()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// StreamHistory happy path: version records staged across two doctypes must be
// emitted in global chronological order, with each commit carrying the
// forward-accumulated entity state at that point (earlier commits must not be
// mutated by later versions of the same entity).
func TestStreamHistory_ChronologicalForwardStates(t *testing.T) {
	f := newTestFetcher(t)
	tmpDir := t.TempDir()

	writeVersionJSONL(t, tmpDir, "Client Script",
		VersionRecord{
			Name: "v1", RefDoctype: "Client Script", Docname: "My Script",
			Owner: "alice@example.com", Creation: "2024-01-01 00:00:00",
			Data: `{"changed":[["script","","console.log(1)"],["dt","","ToDo"]]}`,
		},
		VersionRecord{
			Name: "v3", RefDoctype: "Client Script", Docname: "My Script",
			Owner: "alice@example.com", Creation: "2024-01-03 00:00:00",
			Data: `{"changed":[["script","console.log(1)","console.log(2)"]]}`,
		},
	)
	writeVersionJSONL(t, tmpDir, "Server Script",
		VersionRecord{
			Name: "v2", RefDoctype: "Server Script", Docname: "Calc",
			Owner: "bob@example.com", Creation: "2024-01-02 00:00:00",
			Data: `{"changed":[["script","","x = 1"]]}`,
		},
	)

	users := map[string]UserInfo{
		"alice@example.com": {Email: "alice@example.com", FullName: "Alice Doe"},
	}

	var commits []ReconstructedCommit
	seenPaths, err := f.StreamHistory(tmpDir, map[string]bool{}, nil, users, func(c ReconstructedCommit) error {
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamHistory: %v", err)
	}

	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}

	// Global chronological order across both doctype streams.
	wantOrder := []string{"v1", "v2", "v3"}
	for i, want := range wantOrder {
		if commits[i].VersionName != want {
			t.Fatalf("commit order = [%s %s %s], want %v",
				commits[0].VersionName, commits[1].VersionName, commits[2].VersionName, wantOrder)
		}
		if i > 0 && commits[i].Timestamp < commits[i-1].Timestamp {
			t.Fatalf("timestamps not chronological: %s before %s", commits[i-1].Timestamp, commits[i].Timestamp)
		}
	}

	// States accumulate forward: v1 creates, v3 updates the same entity.
	if got := commits[0].Content["script"]; got != "console.log(1)" {
		t.Errorf("v1 content script = %v, want console.log(1)", got)
	}
	if got := commits[2].Content["script"]; got != "console.log(2)" {
		t.Errorf("v3 content script = %v, want console.log(2)", got)
	}
	// Fields untouched by v3 survive from v1 (forward accumulation, not replace).
	if got := commits[2].Content["dt"]; got != "ToDo" {
		t.Errorf("v3 content dt = %v, want ToDo (accumulated from v1)", got)
	}

	// Author resolution: known users get their full name, unknown ones a
	// name derived from the email.
	if commits[0].Author != "Alice Doe <alice@example.com>" {
		t.Errorf("author = %q, want resolved full name", commits[0].Author)
	}
	if !strings.Contains(commits[1].Author, "<bob@example.com>") {
		t.Errorf("author = %q, want fallback containing <bob@example.com>", commits[1].Author)
	}

	// Entities were not passed in, so paths are built under the "_" module.
	for _, p := range []string{"_/client_script/my_script.json", "_/server_script/calc.json"} {
		if !seenPaths[p] {
			t.Errorf("seenPaths missing %s: %v", p, seenPaths)
		}
	}
	for i, c := range commits {
		if c.Message == "" {
			t.Errorf("commit %d has empty message", i)
		}
	}
}

// A Version record whose data field is not valid JSON must fail the stream
// with an error that names the offending version for traceability.
func TestStreamHistory_MalformedVersionData(t *testing.T) {
	f := newTestFetcher(t)
	tmpDir := t.TempDir()

	writeVersionJSONL(t, tmpDir, "Server Script",
		VersionRecord{
			Name: "v-bad", RefDoctype: "Server Script", Docname: "Broken",
			Owner: "a@example.com", Creation: "2024-01-01 00:00:00",
			Data: `{"changed":[["script"`, // truncated JSON
		},
	)

	var commits []ReconstructedCommit
	_, err := f.StreamHistory(tmpDir, map[string]bool{}, nil, nil, func(c ReconstructedCommit) error {
		commits = append(commits, c)
		return nil
	})
	if err == nil {
		t.Fatal("expected error for malformed version data")
	}
	if !strings.Contains(err.Error(), "v-bad") {
		t.Errorf("error should name the version, got: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("no commit should be emitted for the malformed record, got %d", len(commits))
	}
}
