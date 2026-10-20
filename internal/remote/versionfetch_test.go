package remote

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// buildVersionFilters gates the incremental pull: a zero window fetches the
// full history (clone); bounds are sent in the site's time zone, because
// Version.creation is naive site-local time, and keep microseconds so a
// record from earlier in the last sync's second is not fetched again.
func TestBuildVersionFilters(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, 10, 20, 5, 30, 35, 288949976, time.UTC)
	until := time.Date(2026, 10, 21, 5, 17, 30, 0, time.UTC)
	winter := time.Date(2026, 11, 2, 5, 30, 0, 0, time.UTC)

	tests := []struct {
		name   string
		window HistoryWindow
		want   []any
	}{
		{
			name:   "zero window fetches full history",
			window: HistoryWindow{},
			want:   []any{[]any{"Version", "ref_doctype", "=", "Server Script"}},
		},
		{
			name:   "no location means UTC",
			window: HistoryWindow{Since: since},
			want: []any{
				[]any{"Version", "ref_doctype", "=", "Server Script"},
				[]any{"Version", "creation", ">", "2026-10-20 05:30:35.288949"},
			},
		},
		{
			name:   "summer time bounds in site time",
			window: HistoryWindow{Since: since, Until: until, Location: berlin},
			want: []any{
				[]any{"Version", "ref_doctype", "=", "Server Script"},
				[]any{"Version", "creation", ">", "2026-10-20 07:30:35.288949"},
				[]any{"Version", "creation", "<=", "2026-10-21 07:17:30.000000"},
			},
		},
		{
			name:   "winter time offset",
			window: HistoryWindow{Since: winter, Location: berlin},
			want: []any{
				[]any{"Version", "ref_doctype", "=", "Server Script"},
				[]any{"Version", "creation", ">", "2026-11-02 06:30:00.000000"},
			},
		},
		{
			name:   "clone bounded above only",
			window: HistoryWindow{Until: until, Location: berlin},
			want: []any{
				[]any{"Version", "ref_doctype", "=", "Server Script"},
				[]any{"Version", "creation", "<=", "2026-10-21 07:17:30.000000"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildVersionFilters("Server Script", tt.window)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("filters =\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

// The window parses naive Frappe datetimes as site time and excludes what
// the previous pull already covered (at or before since) and what the next
// one will cover (after until).
func TestHistoryWindow_ParseAndContains(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	w := HistoryWindow{
		Since:    time.Date(2026, 10, 20, 5, 30, 35, 288949976, time.UTC),
		Until:    time.Date(2026, 10, 21, 5, 17, 30, 0, time.UTC),
		Location: berlin,
	}
	tests := []struct {
		creation string
		want     bool
		offset   string
	}{
		{"2026-10-20 07:30:35.100000", false, "+02:00"}, // same second, before since
		{"2026-10-20 07:30:35.288949", false, "+02:00"}, // equal (truncated to µs) -> covered
		{"2026-10-20 07:30:35.288950", true, "+02:00"},
		{"2026-10-20 06:50:00", false, "+02:00"}, // the hour weg used to replay again
		{"2026-10-21 07:17:30", true, "+02:00"},  // until is inclusive
		{"2026-10-21 07:17:30.000001", false, "+02:00"},
		{"2026-10-20T12:00:00", true, "+02:00"},
	}
	for _, tt := range tests {
		t.Run(tt.creation, func(t *testing.T) {
			at, ok := w.ParseSiteTime(tt.creation)
			if !ok {
				t.Fatalf("ParseSiteTime(%q) failed", tt.creation)
			}
			if got := at.Format("-07:00"); got != tt.offset {
				t.Errorf("offset = %s, want %s", got, tt.offset)
			}
			if got := w.Contains(at); got != tt.want {
				t.Errorf("Contains(%s) = %v, want %v", at, got, tt.want)
			}
		})
	}
	if _, ok := w.ParseSiteTime("not a time"); ok {
		t.Error("ParseSiteTime accepted garbage")
	}
}

// A staged cache is resumed only for the window it was fetched for: a cache
// left by a failed pull with another last_sync would otherwise make the
// cursors skip records of the new window.
func TestPrepareVersionCache(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	since := time.Date(2026, 10, 20, 5, 30, 35, 0, time.UTC)
	w := HistoryWindow{Since: since, Until: since.Add(24 * time.Hour), Location: berlin}

	tests := []struct {
		name     string
		next     HistoryWindow
		wantKept bool
	}{
		{"same window resumes", w, true},
		{"later until still resumes", HistoryWindow{Since: since, Until: since.Add(48 * time.Hour), Location: berlin}, true},
		{"other since resets", HistoryWindow{Since: since.Add(time.Hour), Location: berlin}, false},
		{"other zone resets", HistoryWindow{Since: since}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "versions")
			if err := prepareVersionCache(dir, w); err != nil {
				t.Fatal(err)
			}
			staged := filepath.Join(dir, "server_script.jsonl")
			os.WriteFile(staged, []byte("{}\n"), 0o644)

			if err := prepareVersionCache(dir, tt.next); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(staged)
			if kept := err == nil; kept != tt.wantKept {
				t.Fatalf("staged records kept = %v, want %v", kept, tt.wantKept)
			}
		})
	}

	// A cache without a marker (older weg) is never resumed.
	dir := filepath.Join(t.TempDir(), "versions")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "x.jsonl"), []byte("{}\n"), 0o644)
	if err := prepareVersionCache(dir, w); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "x.jsonl")); err == nil {
		t.Fatal("unmarked cache was resumed")
	}
}

func TestTruncateJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.jsonl")
	// 3 complete lines + a partial trailing line (crash mid-write).
	os.WriteFile(path, []byte("a\nb\nc\npar"), 0o644)

	// Cursor acknowledged only 2 records → realign to 2 lines.
	if err := truncateJSONL(path, 2); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "a\nb\n" {
		t.Fatalf("got %q, want %q", got, "a\nb\n")
	}
}

func TestTruncateJSONL_NoopCases(t *testing.T) {
	dir := t.TempDir()

	// Missing file, n==0: no error, no file created.
	if err := truncateJSONL(filepath.Join(dir, "absent.jsonl"), 0); err != nil {
		t.Fatalf("missing file: %v", err)
	}

	// Cursor >= lines present: keep everything.
	path := filepath.Join(dir, "v.jsonl")
	os.WriteFile(path, []byte("a\nb\n"), 0o644)
	if err := truncateJSONL(path, 5); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "a\nb\n" {
		t.Fatalf("over-truncated: %q", got)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.cursor")
	if got := readCursor(path); got != 0 {
		t.Fatalf("absent cursor = %d, want 0", got)
	}
	if err := writeCursor(path, 342); err != nil {
		t.Fatal(err)
	}
	if got := readCursor(path); got != 342 {
		t.Fatalf("round-trip = %d, want 342", got)
	}
}

// A torn cursor write (possible before writeCursor became atomic via
// fsutil.AtomicWrite) leaves an empty or garbage cursor file. Resume semantics
// require that such a cursor reads as 0 so the JSONL is truncated to zero and
// the whole stream is refetched — the pipeline restarts cleanly instead of
// trusting a corrupt offset and skipping records.
func TestReadCursor_TornWriteResumesFromZero(t *testing.T) {
	dir := t.TempDir()

	cases := map[string]string{
		"empty":    "",     // torn write: file created, contents never landed
		"garbage":  "abc",  // partial/corrupt contents
		"negative": "-5\n", // never valid
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			cursorPath := filepath.Join(dir, name+".cursor")
			if err := os.WriteFile(cursorPath, []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := readCursor(cursorPath); got != 0 {
				t.Fatalf("readCursor(%q) = %d, want 0", contents, got)
			}
		})
	}

	// With the cursor read as 0, resume must discard everything already
	// staged: truncateJSONL(path, 0) empties the JSONL so the refetch
	// starts from a clean slate (cursor never lags behind data on disk).
	jsonlPath := filepath.Join(dir, "v.jsonl")
	if err := os.WriteFile(jsonlPath, []byte("{\"name\":\"a\"}\n{\"name\":\"b\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := truncateJSONL(jsonlPath, 0); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("JSONL not emptied on zero-cursor resume: %q", got)
	}
}

// writeCursor must be atomic: after a successful write the cursor is never
// observed empty, and a rewrite replaces the value completely.
func TestWriteCursor_AtomicOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.cursor")

	if err := writeCursor(path, 100); err != nil {
		t.Fatal(err)
	}
	if err := writeCursor(path, 7); err != nil {
		t.Fatal(err)
	}
	if got := readCursor(path); got != 7 {
		t.Fatalf("cursor after overwrite = %d, want 7", got)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("cursor file empty after write")
	}
}

func TestSlugifyDoctype(t *testing.T) {
	cases := map[string]string{"Custom Field": "custom_field", "DocType": "doctype", "Client Script": "client_script"}
	for in, want := range cases {
		if got := slugifyDoctype(in); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
