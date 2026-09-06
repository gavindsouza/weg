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

// frappeDatetimeLayout is how Frappe serializes and filters datetimes.
const frappeDatetimeLayout = "2006-01-02 15:04:05"

// versionPageSize is the number of Version rows fetched per request.
const versionPageSize = 100

// versionFetchFields are the Version columns we stage.
var versionFetchFields = []string{"name", "ref_doctype", "docname", "owner", "creation", "data"}

// FetchVersionsToDisk fetches Version history for all config-enabled doctypes
// into tmpDir as JSONL, in parallel, resuming from any existing cursors. The
// first failure cancels the remaining fetchers at their next page boundary.
// progress is called (may be nil) with the running record count per doctype.
//
// A non-zero since restricts the fetch to Version records created after that
// time (incremental pull); the zero value fetches the full history (clone).
func (f *Fetcher) FetchVersionsToDisk(ctx context.Context, tmpDir string, since time.Time, progress func(doctype string, total int)) error {
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	doctypes := f.enabledVersionedDoctypes()
	errs := make([]error, len(doctypes))
	done := make(chan int, len(doctypes))

	for i, dt := range doctypes {
		go func(i int, dt string) {
			errs[i] = f.fetchDoctypeVersions(ctx, tmpDir, dt, since, progress)
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

// versionJSONLPath / versionCursorPath give the on-disk staging paths for a doctype.
func versionJSONLPath(tmpDir, doctype string) string {
	return filepath.Join(tmpDir, slugifyDoctype(doctype)+".jsonl")
}
func versionCursorPath(tmpDir, doctype string) string {
	return filepath.Join(tmpDir, slugifyDoctype(doctype)+".cursor")
}

func (f *Fetcher) fetchDoctypeVersions(ctx context.Context, tmpDir, doctype string, since time.Time, progress func(string, int)) error {
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
		page, err := f.getVersionPage(doctype, offset, since)
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

// buildVersionFilters builds the Frappe filter set for a doctype's Version rows.
// A non-zero since adds a `creation > since` clause so only records newer than
// the last sync are returned; the zero value returns the full history.
func buildVersionFilters(doctype string, since time.Time) map[string]any {
	filters := map[string]any{"ref_doctype": doctype}
	if !since.IsZero() {
		filters["creation"] = []any{">", since.Format(frappeDatetimeLayout)}
	}
	return filters
}

// getVersionPage fetches one page of Version rows for a doctype, ordered oldest
// first so the on-disk stream is sorted for the k-way merge.
func (f *Fetcher) getVersionPage(doctype string, offset int, since time.Time) ([]VersionRecord, error) {
	filters, _ := json.Marshal(buildVersionFilters(doctype, since))
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
