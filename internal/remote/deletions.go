/*
Copyright © 2025 Gavin <me@gavv.in>

Removals: the site's Deleted Document rows, replayed as deletion commits, and
the committed export files (or Custom Field / Property Setter rows) the
current fetch no longer produces, removed only when the site confirms it.
*/
package remote

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
)

// deletedDocumentFields are the Deleted Document columns a replay needs:
// what was deleted, by whom and when.
var deletedDocumentFields = []string{"name", "deleted_doctype", "deleted_name", "owner", "creation"}

// buildDeletionFilters builds the Frappe filter list for the Deleted Document
// rows of doctypes inside window. Deleted Document.creation is naive site
// time, like Version.creation, so the bounds are converted the same way.
func buildDeletionFilters(doctypes []string, window HistoryWindow) []any {
	filters := []any{[]any{"Deleted Document", "deleted_doctype", "in", doctypes}}
	loc := window.location()
	if !window.Since.IsZero() {
		filters = append(filters, []any{"Deleted Document", "creation", ">", window.Since.In(loc).Format(frappeFilterLayout)})
	}
	if !window.Until.IsZero() {
		filters = append(filters, []any{"Deleted Document", "creation", "<=", window.Until.In(loc).Format(frappeFilterLayout)})
	}
	return filters
}

// FetchDeletions returns the site's Deleted Document rows for the enabled
// entity doctypes created inside window, oldest first, as records with
// Deleted set: Docname is the deleted document, Owner the user who deleted
// it and Creation when. Frappe writes no Version on delete, so these rows
// are the only record of who removed a document.
func (f *Fetcher) FetchDeletions(window HistoryWindow) ([]VersionRecord, error) {
	doctypes := f.enabledVersionedDoctypes()
	if len(doctypes) == 0 {
		return nil, nil
	}
	filters, _ := json.Marshal(buildDeletionFilters(doctypes, window))
	fields, _ := json.Marshal(deletedDocumentFields)

	var recs []VersionRecord
	for offset := 0; ; offset += versionPageSize {
		params := url.Values{}
		params.Set("filters", string(filters))
		params.Set("fields", string(fields))
		params.Set("order_by", "creation asc, name asc")
		params.Set("limit_page_length", fmt.Sprintf("%d", versionPageSize))
		params.Set("limit_start", fmt.Sprintf("%d", offset))
		body, err := f.Client.request("GET", "/api/resource/"+url.PathEscape("Deleted Document")+"?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Data []struct {
				Name           string `json:"name"`
				DeletedDoctype string `json:"deleted_doctype"`
				DeletedName    string `json:"deleted_name"`
				Owner          string `json:"owner"`
				Creation       string `json:"creation"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("parse deleted documents: %w", err)
		}
		for _, d := range page.Data {
			recs = append(recs, VersionRecord{
				Name:       d.Name,
				RefDoctype: d.DeletedDoctype,
				Docname:    d.DeletedName,
				Owner:      d.Owner,
				Creation:   d.Creation,
				Deleted:    true,
			})
		}
		if len(page.Data) < versionPageSize {
			break
		}
	}
	sort.SliceStable(recs, func(i, j int) bool { return lessVersion(recs[i], recs[j]) })
	return recs, nil
}

// Enabled reports whether an entity type is synced.
func (e EntitySyncSettings) Enabled(et EntityType) bool {
	switch et {
	case EntityDocType:
		return e.DocType
	case EntityCustomField:
		return e.CustomField
	case EntityPropertySetter:
		return e.PropertySetter
	case EntityClientScript:
		return e.ClientScript
	case EntityServerScript:
		return e.ServerScript
	case EntityReport:
		return e.Report
	case EntityPrintFormat:
		return e.PrintFormat
	case EntityWorkflow:
		return e.Workflow
	case EntityNotification:
		return e.Notification
	case EntityLetterHead:
		return e.LetterHead
	case EntityWorkspace:
		return e.Workspace
	case EntityWebTemplate:
		return e.WebTemplate
	case EntityNumberCard:
		return e.NumberCard
	case EntityDashboard:
		return e.Dashboard
	case EntityDashboardChart:
		return e.DashboardChart
	}
	return false
}
