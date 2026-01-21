package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Table wraps tabwriter with consistent defaults for CLI table output.
type Table struct {
	w       *tabwriter.Writer
	headers []string
	written bool
}

// NewTable creates a table that writes to the configured Writer (stdout by default).
// Headers are printed as the first row in uppercase.
//
// Example:
//
//	t := output.NewTable("NAME", "STATUS", "APPS")
//	t.Row("mysite", "Running", 3)
//	t.Row("test", "Stopped", 1)
//	t.Flush()
func NewTable(headers ...string) *Table {
	return NewTableWriter(Writer, headers...)
}

// NewTableWriter creates a table that writes to a custom writer.
func NewTableWriter(w io.Writer, headers ...string) *Table {
	// Standard settings: minwidth=0, tabwidth=0, padding=2, padchar=' '
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	return &Table{
		w:       tw,
		headers: headers,
	}
}

// Row adds a row to the table. Values are converted to strings.
// The header row is automatically written before the first data row.
func (t *Table) Row(values ...any) {
	if !t.written && len(t.headers) > 0 {
		t.writeHeaders()
	}
	t.written = true

	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = fmt.Sprintf("%v", v)
	}
	fmt.Fprintln(t.w, strings.Join(strs, "\t"))
}

// writeHeaders writes the header row in uppercase.
func (t *Table) writeHeaders() {
	upper := make([]string, len(t.headers))
	for i, h := range t.headers {
		upper[i] = strings.ToUpper(h)
	}
	fmt.Fprintln(t.w, strings.Join(upper, "\t"))
}

// Flush writes any buffered data to the underlying writer.
// Must be called after all rows are added.
func (t *Table) Flush() error {
	return t.w.Flush()
}

// Empty returns true if no rows have been written.
func (t *Table) Empty() bool {
	return !t.written
}
