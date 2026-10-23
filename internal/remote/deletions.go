/*
Copyright © 2025 Gavin <me@gavv.in>

Removals: the site's Deleted Document rows, replayed as deletion commits, and
the committed export files the current fetch no longer produces.
*/
package remote

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
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

// StaleFile is a committed export file the current fetch no longer
// produces. Whether it goes is up to ConfirmStale, which asks the site.
type StaleFile struct {
	Path string
	Type EntityType
	// Unsynced marks a file of a module marked sync = false: it goes only
	// when the site records its document's deletion (or a Version in the
	// window shows the site had it), never for a module move or a DocType
	// that is no longer custom.
	Unsynced bool
}

// StaleExportFiles returns, sorted by path, the committed export files that
// the current fetch no longer produces: a deleted document, the old path of
// a document that moved module, a DocType that is no longer custom.
// committed lists the files committed in the export, relative to it.
//
// Only entity files weg itself lays out are candidates, and only for entity
// types enabled in [sync.entities]: the fetch does not cover the others, so
// their absence from it says nothing about the site. For the same reason an
// entity type whose fetch was incomplete (result.Incomplete) has no stale
// files, and nothing is stale when a DocType's module could not be read,
// since files were then placed away from their usual paths.
func StaleExportFiles(committed []string, result *FetchResult, cfg *SiteConfig) []StaleFile {
	if len(result.ModuleLookupFailed) > 0 {
		return nil
	}
	current := make(map[string]bool, len(result.Entities))
	for _, e := range result.Entities {
		current[e.FilePath] = true
	}
	unsynced := make(map[string]bool)
	for name, info := range cfg.Modules {
		if !info.Sync {
			unsynced[pathSegment(name)] = true
		}
	}
	var stale []StaleFile
	for _, p := range committed {
		if current[p] || CheckExportPath(p) != nil {
			continue
		}
		et, ok := entityTypeOfPath(p)
		if !ok || !cfg.Sync.Entities.Enabled(et) || len(result.Incomplete[et]) > 0 {
			continue
		}
		module, _, _ := strings.Cut(p, "/")
		stale = append(stale, StaleFile{Path: p, Type: et, Unsynced: unsynced[module]})
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].Path < stale[j].Path })
	return stale
}

// Removal is a stale export file the site confirmed is gone, and why.
type Removal struct {
	Path   string
	Reason string
	// Deletion is the site's latest Deleted Document row for what the file
	// held, when it has one: who deleted it, and when.
	Deletion *VersionRecord
	// Doc is the file's committed content.
	Doc map[string]any
}

// StaleEvidence is what ConfirmStale knows about the stale files beyond
// the site's answers.
type StaleEvidence struct {
	// Content is each file's committed content.
	Content map[string][]byte
	// Seen holds the files a Version in the pull's window wrote: the site
	// had their document then.
	Seen map[string]bool
	// Pushed reports whether a file holds nothing local that a push has yet
	// to send, so the site had it. Nil when there is no push baseline to
	// tell, as in a CI checkout: a file then goes only on the site's word.
	Pushed func(path string) bool
}

// ConfirmStale asks the site about each stale file and returns the ones
// that should go, with the reason, plus a line per file it keeps and why.
// The list fetch alone never removes a file: it can come back short (a page
// boundary shifting under a concurrent edit, a list narrowed by the API
// user's permissions), so a document is gone only when the site says so.
//
//   - Its document is exported now at another path (a module move): goes.
//   - The site has the document but no longer exports it (a DocType that is
//     no longer custom, a Report or Print Format made standard): goes.
//   - The site has the document and would export it: kept, the list missed it.
//   - The site does not have it: goes when a Deleted Document row records
//     its deletion (at any time, not only inside the pull's window), when a
//     Version in the window wrote it, or when it was pushed or pulled (no
//     local work since the push baseline). Otherwise it may be a document
//     made locally and not pushed yet, and it is kept.
//
// A file of a module marked sync = false only goes on a Deleted Document
// row or a Version in the window. Any error from the site keeps the file.
func (f *Fetcher) ConfirmStale(stale []StaleFile, result *FetchResult, ev StaleEvidence) ([]Removal, []string) {
	exported := make(map[string]string, len(result.Entities))
	for _, e := range result.Entities {
		exported[entityKey(e.Type, e.Name)] = e.FilePath
	}
	var remove []Removal
	var kept []string
	keep := func(p, format string, args ...any) {
		kept = append(kept, p+": "+fmt.Sprintf(format, args...))
	}
	trashReadable := true
	for _, s := range stale {
		var doc map[string]any
		if err := json.Unmarshal(ev.Content[s.Path], &doc); err != nil {
			keep(s.Path, "cannot read which document it holds (%v)", err)
			continue
		}
		doctype := entityTypeDoctype(s.Type)
		rowsKey, agg := aggregatedRows[s.Type]
		name := getString(doc, "name")
		if agg {
			name = getString(doc, "doctype")
		}
		if name == "" {
			keep(s.Path, "cannot tell which document it holds")
			continue
		}
		if p, ok := exported[entityKey(s.Type, name)]; ok {
			if !s.Unsynced {
				remove = append(remove, Removal{Path: s.Path, Reason: "moved to " + p, Doc: doc})
			}
			continue
		}

		// Does the site still have it?
		names := []string{name}
		if agg {
			names = nil
			for _, row := range rowList(doc[rowsKey]) {
				if n := getString(row, "name"); n != "" {
					names = append(names, n)
				}
			}
			has, err := f.hasRows(s.Type, name)
			if err != nil {
				keep(s.Path, "could not ask the site for the %s rows of %s (%v)", doctype, name, err)
				continue
			}
			if has {
				keep(s.Path, "the site still has %s rows for %s, but the fetch did not list them", doctype, name)
				continue
			}
		} else {
			siteDoc, err := f.Client.GetDoc(doctype, name)
			switch {
			case err == nil && !exportable(s.Type, siteDoc):
				if !s.Unsynced {
					remove = append(remove, Removal{Path: s.Path, Reason: "no longer exported (made standard)", Doc: doc})
				}
				continue
			case err == nil:
				keep(s.Path, "the site still has %s %q, but the fetch did not list it", doctype, name)
				continue
			case !IsNotFound(err):
				keep(s.Path, "could not ask the site for %s %q (%v)", doctype, name, err)
				continue
			}
		}

		// The site does not have it: deleted, or never pushed.
		var deletion *VersionRecord
		if trashReadable && len(names) > 0 {
			d, err := f.latestDeletion(doctype, names)
			if err != nil {
				trashReadable = false
				kept = append(kept, fmt.Sprintf("(could not read the site's Deleted Document rows: %v)", err))
			}
			deletion = d
		}
		switch {
		case deletion != nil:
			remove = append(remove, Removal{Path: s.Path, Reason: "deleted by " + deletion.Owner, Deletion: deletion, Doc: doc})
		case ev.Seen[s.Path]:
			remove = append(remove, Removal{Path: s.Path, Reason: "deleted or renamed", Doc: doc})
		case s.Unsynced:
		case ev.Pushed != nil && ev.Pushed(s.Path):
			remove = append(remove, Removal{Path: s.Path, Reason: "deleted", Doc: doc})
		case ev.Pushed == nil:
			keep(s.Path, "the site has no %s %q and no Deleted Document row for it, and without a push baseline it may be local work not pushed yet; remove it by hand if it was deleted", doctype, name)
		default:
			keep(s.Path, "the site has no %s %q, but the file has local work not pushed yet", doctype, name)
		}
	}
	return remove, kept
}

// exportable reports whether the fetch would export doc, a document of
// type et the site has: false for a DocType that is no longer custom and a
// Report or Print Format that is now standard.
func exportable(et EntityType, doc map[string]any) bool {
	switch et {
	case EntityDocType:
		return truthy(doc["custom"])
	case EntityReport:
		return getString(doc, "is_standard") == "No"
	case EntityPrintFormat:
		return getString(doc, "standard") == "No"
	}
	return true
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != "" && x != "0"
	}
	return false
}

// hasRows reports whether the site has any exported Custom Field or
// Property Setter rows for the DocType target.
func (f *Fetcher) hasRows(et EntityType, target string) (bool, error) {
	filters := map[string]any{"doc_type": target}
	if et == EntityCustomField {
		filters = map[string]any{"dt": target, "is_system_generated": 0}
	}
	rows, err := f.Client.GetList(entityTypeDoctype(et), filters, []string{"name"}, 1)
	return len(rows) > 0, err
}

// latestDeletion returns the site's latest Deleted Document row for any of
// the documents names of doctype, at any time, or nil when there is none.
func (f *Fetcher) latestDeletion(doctype string, names []string) (*VersionRecord, error) {
	filters, _ := json.Marshal([]any{
		[]any{"Deleted Document", "deleted_doctype", "=", doctype},
		[]any{"Deleted Document", "deleted_name", "in", names},
	})
	fields, _ := json.Marshal(deletedDocumentFields)
	params := url.Values{}
	params.Set("filters", string(filters))
	params.Set("fields", string(fields))
	params.Set("order_by", "creation desc, name desc")
	params.Set("limit_page_length", "1")
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
	if len(page.Data) == 0 {
		return nil, nil
	}
	d := page.Data[0]
	return &VersionRecord{Name: d.Name, RefDoctype: d.DeletedDoctype, Docname: d.DeletedName,
		Owner: d.Owner, Creation: d.Creation, Deleted: true}, nil
}

// DeletionCommit is the commit that removes a file whose deletion the site
// records (Removal.Deletion) before the pull's window: by the user who
// deleted it, dated when, in window's time zone.
func DeletionCommit(r Removal, window HistoryWindow, users map[string]UserInfo) ReconstructedCommit {
	d := r.Deletion
	when, _ := window.ParseSiteTime(d.Creation)
	et, _ := entityTypeOfPath(r.Path)
	module, _, _ := strings.Cut(r.Path, "/")
	if m := getString(r.Doc, "module"); m != "" {
		module = m
	}
	data := r.Doc
	if rowsKey, agg := aggregatedRows[et]; agg {
		if row := findRow(r.Doc[rowsKey], d.Docname); row != nil {
			data = row
		}
	}
	return ReconstructedCommit{
		Removed:     []string{r.Path},
		Author:      formatAuthor(d.Owner, users),
		Timestamp:   d.Creation,
		When:        when,
		VersionName: d.Name,
		Message: generateCommitMessage(HistoryEntry{
			Timestamp:   d.Creation,
			Author:      d.Owner,
			DocType:     d.RefDoctype,
			DocName:     d.Docname,
			Action:      "delete",
			VersionName: d.Name,
			EntityType:  et,
			Module:      module,
			FilePath:    r.Path,
			EntityData:  data,
		}),
	}
}
