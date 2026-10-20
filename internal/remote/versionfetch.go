/*
Copyright © 2025 Gavin <me@gavv.in>

Parallel, resumable fetch of Version records to disk as per-doctype JSONL.

Each versioned doctype is paginated on its own goroutine and streamed to
.weg/tmp/versions/<slug>.jsonl, with a <slug>.cursor recording how many records
have been durably written. Memory stays flat (one page per doctype); a crash
resumes from the cursor instead of refetching.
*/
package remote

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gavindsouza/weg/internal/fsutil"
)

// frappeFilterLayout keeps microseconds in Version filters: Version.creation
// carries them, so a whole-second bound would let a record from earlier in
// the same second as the last sync through again.
const frappeFilterLayout = "2006-01-02 15:04:05.000000"

// versionPageSize is the number of Version rows fetched per request.
const versionPageSize = 100

// versionFetchFields are the Version columns we stage.
var versionFetchFields = []string{"name", "ref_doctype", "docname", "owner", "creation", "data"}

// windowMarkerFile records which window a staged version cache was fetched
// for, so a cache left by an interrupted pull is only resumed for that window.
const windowMarkerFile = "window"

// HistoryWindow bounds the Version records a history replay covers.
//
// Frappe stores Version.creation as naive wall-clock time in the site's time
// zone (System Settings), so the bounds are converted to that zone for the
// API filter, and replayed commits are dated in it. The hour repeated when
// daylight saving ends is ambiguous in that form; it is read as its first
// occurrence, as the site's own filters do.
type HistoryWindow struct {
	Since    time.Time      // exclusive lower bound; zero replays from the beginning (clone)
	Until    time.Time      // inclusive upper bound; zero means no upper bound
	Location *time.Location // the site's time zone; nil means UTC
}

func (w HistoryWindow) location() *time.Location {
	if w.Location == nil {
		return time.UTC
	}
	return w.Location
}

// ParseSiteTime parses a naive Frappe datetime as the site's local time.
func (w HistoryWindow) ParseSiteTime(s string) (time.Time, bool) {
	return parseFrappeTime(s, w.location())
}

// Contains reports whether t falls inside the window.
func (w HistoryWindow) Contains(t time.Time) bool {
	if !w.Since.IsZero() && !t.After(w.Since) {
		return false
	}
	if !w.Until.IsZero() && t.After(w.Until) {
		return false
	}
	return true
}

// marker identifies the window's fetch filter for the resume cache. Until is
// left out on purpose: raising it only appends records, so a cache fetched
// with an earlier Until is still a valid prefix.
func (w HistoryWindow) marker() string {
	since := "-"
	if !w.Since.IsZero() {
		since = w.Since.UTC().Format(time.RFC3339Nano)
	}
	return since + " " + w.location().String() + "\n"
}

// parseFrappeTime parses a Frappe datetime ("2025-01-21 14:31:36.892644") in loc.
func parseFrappeTime(s string, loc *time.Location) (time.Time, bool) {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
	} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// FetchVersionsToDisk fetches Version history for all config-enabled doctypes
// into tmpDir as JSONL, in parallel, resuming from any existing cursors. The
// first failure cancels the remaining fetchers at their next page boundary.
// progress is called (may be nil) with the running record count per doctype.
//
// window restricts the fetch to Version records created inside it; the zero
// window fetches the full history (clone). A cache staged for a different
// lower bound or time zone is discarded rather than resumed.
func (f *Fetcher) FetchVersionsToDisk(ctx context.Context, tmpDir string, window HistoryWindow, progress func(doctype string, total int)) error {
	if err := prepareVersionCache(tmpDir, window); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	doctypes := f.enabledVersionedDoctypes()
	errs := make([]error, len(doctypes))
	done := make(chan int, len(doctypes))

	for i, dt := range doctypes {
		go func(i int, dt string) {
			errs[i] = f.fetchDoctypeVersions(ctx, tmpDir, dt, window, progress)
			if errs[i] != nil {
				cancel()
			}
			done <- i
		}(i, dt)
	}
	for range doctypes {
		<-done
	}
	return errors.Join(errs...)
}

// prepareVersionCache creates tmpDir and makes sure any staged records in it
// were fetched for the same window; otherwise their cursors would skip
// records of the new window.
func prepareVersionCache(tmpDir string, window HistoryWindow) error {
	markerPath := filepath.Join(tmpDir, windowMarkerFile)
	want := window.marker()
	if got, err := os.ReadFile(markerPath); err != nil || string(got) != want {
		if err := os.RemoveAll(tmpDir); err != nil {
			return fmt.Errorf("reset version cache: %w", err)
		}
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	return fsutil.AtomicWrite(markerPath, []byte(want), 0o644)
}

// versionJSONLPath / versionCursorPath give the on-disk staging paths for a doctype.
func versionJSONLPath(tmpDir, doctype string) string {
	return filepath.Join(tmpDir, slugifyDoctype(doctype)+".jsonl")
}
func versionCursorPath(tmpDir, doctype string) string {
	return filepath.Join(tmpDir, slugifyDoctype(doctype)+".cursor")
}

func (f *Fetcher) fetchDoctypeVersions(ctx context.Context, tmpDir, doctype string, window HistoryWindow, progress func(string, int)) error {
	jsonlPath := versionJSONLPath(tmpDir, doctype)
	cursorPath := versionCursorPath(tmpDir, doctype)

	// Resume: trust the cursor, discard any JSONL written past it (a crash may
	// have flushed rows the cursor never acknowledged).
	offset := readCursor(cursorPath)
	if err := truncateJSONL(jsonlPath, offset); err != nil {
		return fmt.Errorf("%s: resume truncate: %w", doctype, err)
	}

	file, err := os.OpenFile(jsonlPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	for {
		if err := ctx.Err(); err != nil {
			return nil // another fetcher failed; its error carries the cause
		}
		page, err := f.getVersionPage(doctype, offset, window)
		if err != nil {
			return fmt.Errorf("%s: %w", doctype, err)
		}

		w := bufio.NewWriter(file)
		for _, rec := range page {
			line, err := json.Marshal(rec)
			if err != nil {
				return fmt.Errorf("%s: marshal record: %w", doctype, err)
			}
			w.Write(line)
			w.WriteByte('\n')
		}
		if err := w.Flush(); err != nil {
			return fmt.Errorf("%s: write: %w", doctype, err)
		}
		if err := file.Sync(); err != nil {
			return fmt.Errorf("%s: sync: %w", doctype, err)
		}

		offset += len(page)
		// Cursor written only after data is durable — JSONL never lags the cursor.
		if err := writeCursor(cursorPath, offset); err != nil {
			return fmt.Errorf("%s: cursor: %w", doctype, err)
		}
		if progress != nil {
			progress(doctype, offset)
		}

		if len(page) < versionPageSize {
			return nil
		}
	}
}

// buildVersionFilters builds the Frappe filter list for a doctype's Version
// rows inside window. The bounds are converted to the site's time zone, since
// Version.creation is naive site-local time; the zero window returns the full
// history.
func buildVersionFilters(doctype string, window HistoryWindow) []any {
	filters := []any{[]any{"Version", "ref_doctype", "=", doctype}}
	loc := window.location()
	if !window.Since.IsZero() {
		filters = append(filters, []any{"Version", "creation", ">", window.Since.In(loc).Format(frappeFilterLayout)})
	}
	if !window.Until.IsZero() {
		filters = append(filters, []any{"Version", "creation", "<=", window.Until.In(loc).Format(frappeFilterLayout)})
	}
	return filters
}

// getVersionPage fetches one page of Version rows for a doctype, ordered oldest
// first so the on-disk stream is sorted for the k-way merge.
func (f *Fetcher) getVersionPage(doctype string, offset int, window HistoryWindow) ([]VersionRecord, error) {
	filters, _ := json.Marshal(buildVersionFilters(doctype, window))
	fields, _ := json.Marshal(versionFetchFields)

	params := url.Values{}
	params.Set("filters", string(filters))
	params.Set("fields", string(fields))
	params.Set("order_by", "creation asc, name asc")
	params.Set("limit_page_length", fmt.Sprintf("%d", versionPageSize))
	params.Set("limit_start", fmt.Sprintf("%d", offset))

	endpoint := "/api/resource/Version?" + params.Encode()
	body, err := f.Client.request("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []VersionRecord `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse versions: %w", err)
	}
	return result.Data, nil
}

// slugifyDoctype turns a doctype name into a filesystem-safe stem.
func slugifyDoctype(doctype string) string {
	return strings.ToLower(strings.ReplaceAll(doctype, " ", "_"))
}

// readCursor returns the record count recorded for a stream, 0 if absent/invalid.
func readCursor(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &n); err != nil || n < 0 {
		return 0
	}
	return n
}

// writeCursor persists the cursor atomically: a torn write would leave an empty
// cursor, and resume would truncate the JSONL to zero and refetch everything.
func writeCursor(path string, n int) error {
	return fsutil.AtomicWrite(path, []byte(fmt.Sprintf("%d\n", n)), 0o644)
}

// truncateJSONL keeps the first n complete lines of path and drops the rest.
// Missing file with n==0 is a no-op. Used on resume to realign JSONL to cursor.
func truncateJSONL(path string, n int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	offset, count := 0, 0
	for count < n {
		i := indexByteFrom(data, '\n', offset)
		if i < 0 {
			// Fewer complete lines than the cursor claims: keep what we have.
			return nil
		}
		offset = i + 1
		count++
	}
	if offset == len(data) {
		return nil
	}
	return os.WriteFile(path, data[:offset], 0o644)
}

func indexByteFrom(b []byte, c byte, from int) int {
	for i := from; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}
