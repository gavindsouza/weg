package remote

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestSlugifyDoctype(t *testing.T) {
	cases := map[string]string{"Custom Field": "custom_field", "DocType": "doctype", "Client Script": "client_script"}
	for in, want := range cases {
		if got := slugifyDoctype(in); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
