package remote

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildVersionFilters gates the incremental pull: a zero `since` fetches the
// full history (clone), a non-zero `since` adds a `creation > since` clause so
// only records newer than the last sync are replayed.
func TestBuildVersionFilters(t *testing.T) {
	t.Run("zero since fetches full history", func(t *testing.T) {
		f := buildVersionFilters("Server Script", time.Time{})
		if f["ref_doctype"] != "Server Script" {
			t.Fatalf("ref_doctype = %v, want Server Script", f["ref_doctype"])
		}
		if _, ok := f["creation"]; ok {
			t.Fatalf("zero since must not add a creation filter, got %v", f["creation"])
		}
	})

	t.Run("non-zero since adds creation lower bound", func(t *testing.T) {
		since := time.Date(2026, 7, 1, 9, 30, 15, 0, time.UTC)
		f := buildVersionFilters("Client Script", since)
		if f["ref_doctype"] != "Client Script" {
			t.Fatalf("ref_doctype = %v, want Client Script", f["ref_doctype"])
		}
		clause, ok := f["creation"].([]any)
		if !ok || len(clause) != 2 {
			t.Fatalf("creation clause = %v, want [op, value]", f["creation"])
		}
		if clause[0] != ">" {
			t.Fatalf("operator = %v, want >", clause[0])
		}
		if clause[1] != "2026-07-01 09:30:15" {
			t.Fatalf("value = %v, want Frappe datetime 2026-07-01 09:30:15", clause[1])
		}
	})
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
