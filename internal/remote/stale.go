/*
Copyright © 2025 Gavin <me@gavv.in>

What a pull removes from the committed export. A list fetch can come back
short (a page boundary shifting under an edit, a list narrowed by the API
user's permissions, a document it could not read), so nothing is removed on
the fetch's word: each record is asked for by name.
*/
package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// probeWorkers bounds the GetDoc requests a plan has in flight.
const probeWorkers = 8

// RecordProbe is what the site says about one record right now (GetDoc).
type RecordProbe struct {
	Doc  map[string]any // the record, when the site returned it
	Gone bool           // the site answered 404: it does not have the record
	// Err is any other answer (403, 429, 5xx, a timeout, a body that does
	// not parse): the site's state is unknown.
	Err error
}

type recordKey struct{ doctype, name string }

// siteCheck asks the site for records by name, once each.
type siteCheck struct {
	client *Client
	mu     sync.Mutex
	probes map[recordKey]RecordProbe
}

func (c *siteCheck) ask(k recordKey) RecordProbe {
	doc, err := c.client.GetDoc(k.doctype, k.name)
	switch {
	case err == nil && doc != nil:
		return RecordProbe{Doc: doc}
	case err == nil:
		return RecordProbe{Err: errors.New("the site answered without the record")}
	case IsNotFound(err):
		return RecordProbe{Gone: true}
	}
	return RecordProbe{Err: err}
}

// prefetch asks for every record in keys not asked yet, a few at a time.
func (c *siteCheck) prefetch(keys []recordKey) {
	var todo []recordKey
	seen := make(map[recordKey]bool)
	for _, k := range keys {
		if _, done := c.probes[k]; !done && !seen[k] {
			seen[k] = true
			todo = append(todo, k)
		}
	}
	work := make(chan recordKey)
	var wg sync.WaitGroup
	for i := 0; i < min(probeWorkers, len(todo)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := range work {
				p := c.ask(k)
				c.mu.Lock()
				c.probes[k] = p
				c.mu.Unlock()
			}
		}()
	}
	for _, k := range todo {
		work <- k
	}
	close(work)
	wg.Wait()
}

func (c *siteCheck) get(doctype, name string) RecordProbe {
	k := recordKey{doctype, name}
	c.mu.Lock()
	p, ok := c.probes[k]
	c.mu.Unlock()
	if !ok {
		p = c.ask(k)
		c.mu.Lock()
		c.probes[k] = p
		c.mu.Unlock()
	}
	return p
}

// placement returns the path the fetch would export a record at, from the
// site's own copy of it: its module, or the module of the DocType it
// belongs to, read with GetDoc. A record whose module cannot be read is an
// error, never the fetch's catch-all module "_", which says nothing about
// where the record belongs.
func (c *siteCheck) placement(et EntityType, name string, doc map[string]any) (string, error) {
	module, ref := "", ""
	switch et {
	case EntityDocType:
		module = getString(doc, "module")
	case EntityServerScript:
		if getString(doc, "script_type") == "DocType Event" {
			ref = getString(doc, "reference_doctype")
		} else {
			module = getString(doc, "module")
		}
	case EntityClientScript:
		ref = getString(doc, "dt")
	case EntityReport:
		ref = getString(doc, "ref_doctype")
	case EntityPrintFormat:
		ref = getString(doc, "doc_type")
	case EntityWorkflow, EntityNotification:
		ref = getString(doc, "document_type")
	case EntityLetterHead:
		module = "Custom"
	case EntityCustomField, EntityPropertySetter:
		ref = name
	default:
		return "", fmt.Errorf("no export layout for %s", et)
	}
	if ref != "" {
		p := c.get("DocType", ref)
		switch {
		case p.Err != nil:
			return "", fmt.Errorf("could not read DocType %q (%v)", ref, p.Err)
		case p.Gone:
			return "", fmt.Errorf("the site has no DocType %q", ref)
		}
		module = getString(p.Doc, "module")
	}
	if module == "" || pathSegment(module) == "_" {
		return "", errors.New("the site gives it no module")
	}
	return buildFilePath(et, name, module), nil
}

// Removal is a committed export file the site confirmed should go, and why.
type Removal struct {
	Path   string
	Reason string
}

// StaleEvidence is what a pull knows besides the site's answers.
type StaleEvidence struct {
	// Committed is the committed export (path -> content).
	Committed map[string][]byte
	// Local marks the files with local work not yet pushed: they stay.
	Local map[string]bool
	// HasBaseline reports a push baseline (.weg/last_push_commit), which
	// tells local work apart: a committed file outside Local holds what the
	// site had. Without one (a CI checkout), a file the site has no record
	// of may be a document made locally and not pushed yet.
	HasBaseline bool
	// Hold, when set, keeps every file, for this reason.
	Hold string
	// Deletions are the site's Deleted Document rows inside the pull's
	// window (FetchDeletions).
	Deletions []VersionRecord
}

// StalePlan is what a pull does with the committed records the fetch does
// not reproduce.
type StalePlan struct {
	// Removals are the files to remove, sorted by path.
	Removals []Removal
	// Deletions are the window's deletions the replay may apply.
	Deletions []VersionRecord
	// Kept says, per file or row, why something the fetch did not produce
	// stays.
	Kept []string
	// Skipped says, per deletion left out of Deletions, why.
	Skipped []string
}

// PlanStale asks the site, record by record, about every committed export
// record the fetch does not reproduce, and plans what goes. A committed
// file (or a Custom Field / Property Setter row of one) goes only when the
// site confirms it right now:
//
//   - GetDoc on the record answers 404, and the file held what the site
//     had: the site recorded the record's deletion inside the pull's window
//     (that Deleted Document row's replay removes it, credited to who
//     deleted it), or a push baseline shows the file has no local work
//     (then it goes in the reconcile commit). A Deleted Document row from
//     before the window is not evidence: the file may have been brought
//     back since.
//   - The fetch exports the record at another path, and the site's copy
//     places it exactly there (a module move).
//   - The site returns the record and would not export it (a DocType that
//     is no longer custom, a Report or Print Format made standard, a
//     system-generated Custom Field).
//
// Anything else keeps the file, with a line in Kept: an error from the
// site, a record it still has and would export, a move it does not
// confirm. Rows the site still has (or cannot be asked about) are merged
// back into their DocType's fetched file in result, so a short list loses
// none. Files with local work stay, nothing goes when a DocType's module
// could not be read, files of an entity type whose fetch was incomplete
// stay, and files of a module marked sync = false go only on a deletion
// recorded in the window.
//
// The window's deletions are checked the same way: one of a record the
// fetch does not export now is replayed only when GetDoc confirms the
// record is gone, so a record restored or created again since, which the
// fetch missed, is never removed.
func (f *Fetcher) PlanStale(result *FetchResult, ev StaleEvidence) *StalePlan {
	plan := &StalePlan{}
	check := &siteCheck{client: f.Client, probes: make(map[recordKey]RecordProbe)}
	cfg := f.Config

	// What the fetch exports now.
	exportedAt := make(map[string]string)  // document unit -> path
	byPath := make(map[string]bool)        // fetched paths
	rowsNow := make(map[string]bool)       // fetched row units
	aggregated := make(map[string]*Entity) // Custom Field / Property Setter file unit -> entity
	for i := range result.Entities {
		e := &result.Entities[i]
		byPath[e.FilePath] = true
		if rowsKey, agg := aggregatedRows[e.Type]; agg {
			aggregated[entityKey(e.Type, e.Name)] = e
			for _, row := range rowList(e.Data[rowsKey]) {
				rowsNow[entityKey(e.Type, getString(row, "name"))] = true
			}
			continue
		}
		exportedAt[entityKey(e.Type, e.Name)] = e.FilePath
	}
	unsynced := make(map[string]bool)
	for name, info := range cfg.Modules {
		if !info.Sync {
			unsynced[pathSegment(name)] = true
		}
	}

	// The committed records the fetch does not reproduce.
	type docFile struct {
		path string
		et   EntityType
		name string
	}
	type rowsFile struct {
		path    string
		et      EntityType
		target  string
		missing []map[string]any
	}
	var docs []docFile
	var files []rowsFile
	var keys []recordKey
	paths := make([]string, 0, len(ev.Committed))
	for p := range ev.Committed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if CheckExportPath(p) != nil {
			continue
		}
		et, ok := entityTypeOfPath(p)
		if !ok || !cfg.Sync.Entities.Enabled(et) {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(ev.Committed[p], &doc); err != nil || doc == nil {
			if !byPath[p] {
				plan.keep(p, "cannot read which document it holds")
			}
			continue
		}
		rowsKey, agg := aggregatedRows[et]
		if !agg {
			if byPath[p] {
				continue
			}
			name := getString(doc, "name")
			if name == "" {
				plan.keep(p, "cannot tell which document it holds")
				continue
			}
			docs = append(docs, docFile{p, et, name})
			keys = append(keys, recordKey{entityTypeDoctype(et), name})
			continue
		}
		target := getString(doc, "doctype")
		if target == "" {
			if !byPath[p] {
				plan.keep(p, "cannot tell which DocType its rows belong to")
			}
			continue
		}
		have := make(map[string]bool)
		if e := aggregated[entityKey(et, target)]; e != nil {
			for _, row := range rowList(e.Data[rowsKey]) {
				have[getString(row, "name")] = true
			}
		}
		var missing []map[string]any
		for _, row := range rowList(doc[rowsKey]) {
			if n := getString(row, "name"); n != "" && !have[n] {
				missing = append(missing, row)
				keys = append(keys, recordKey{entityTypeDoctype(et), n})
			}
		}
		if e := aggregated[entityKey(et, target)]; e != nil && e.FilePath == p && len(missing) == 0 {
			continue
		}
		files = append(files, rowsFile{p, et, target, missing})
	}
	check.prefetch(keys)

	// The window's deletions: the newest per record, and which the replay
	// may apply.
	inWindow := make(map[string]VersionRecord)
	for _, d := range ev.Deletions {
		et, ok := DoctypeToEntityType[d.RefDoctype]
		if !ok {
			plan.Deletions = append(plan.Deletions, d)
			continue
		}
		unit := entityKey(et, d.Docname)
		if prev, seen := inWindow[unit]; !seen || lessVersion(prev, d) {
			inWindow[unit] = d
		}
		_, exported := exportedAt[unit]
		if exported || rowsNow[unit] {
			// Created again since: the replay removes it, then brings it back.
			plan.Deletions = append(plan.Deletions, d)
			continue
		}
		probe, held := check.probes[recordKey{d.RefDoctype, d.Docname}]
		switch {
		case !held || probe.Gone:
			plan.Deletions = append(plan.Deletions, d)
		case probe.Err != nil:
			plan.Skipped = append(plan.Skipped, fmt.Sprintf("%s %q (deleted by %s at %s): could not ask the site whether it still has it (%v)",
				d.RefDoctype, d.Docname, d.Owner, d.Creation, probe.Err))
		default:
			plan.Skipped = append(plan.Skipped, fmt.Sprintf("%s %q (deleted by %s at %s): the site has it again",
				d.RefDoctype, d.Docname, d.Owner, d.Creation))
		}
	}
	deletedBy := func(unit string) string {
		d, ok := inWindow[unit]
		if !ok {
			return ""
		}
		return fmt.Sprintf("deleted by %s at %s", d.Owner, d.Creation)
	}

	hold := func(et EntityType, p string) string {
		switch {
		case ev.Hold != "":
			return ev.Hold
		case len(result.ModuleLookupFailed) > 0:
			return "the module of some DocTypes could not be read, so files may have been placed away from their usual paths"
		case len(result.Incomplete[et]) > 0:
			return fmt.Sprintf("the fetch could not read every %s", et)
		case ev.Local[p]:
			return "it has local changes not yet pushed"
		}
		return ""
	}
	isUnsynced := func(p string) bool {
		module, _, _ := strings.Cut(p, "/")
		return unsynced[module]
	}
	noBaseline := "without a push baseline (.weg/last_push_commit) the file may be local work not pushed yet; remove it by hand if the record was deleted"
	unsyncedGone := "its module is marked sync = false, where only a deletion recorded in this pull's window removes a file"

	for _, d := range docs {
		doctype := entityTypeDoctype(d.et)
		unit := entityKey(d.et, d.name)
		probe := check.get(doctype, d.name)
		held, off := hold(d.et, d.path), isUnsynced(d.path)
		switch {
		case probe.Gone && deletedBy(unit) != "":
			if held == "" {
				plan.remove(d.path, deletedBy(unit))
			}
		case held != "":
			plan.keep(d.path, held)
		case probe.Err != nil:
			plan.keep(d.path, fmt.Sprintf("could not ask the site for %s %q (%v)", doctype, d.name, probe.Err))
		case probe.Gone && off:
			plan.keep(d.path, fmt.Sprintf("the site has no %s %q, but %s", doctype, d.name, unsyncedGone))
		case probe.Gone && ev.HasBaseline:
			plan.remove(d.path, "deleted")
		case probe.Gone:
			plan.keep(d.path, fmt.Sprintf("the site has no %s %q and recorded no deletion of it in this pull's window, and %s", doctype, d.name, noBaseline))
		case !exportable(d.et, probe.Doc):
			if off {
				plan.keep(d.path, fmt.Sprintf("the site no longer exports %s %q, but its module is marked sync = false", doctype, d.name))
			} else {
				plan.remove(d.path, "no longer exported (made standard)")
			}
		case exportedAt[unit] != "":
			to := exportedAt[unit]
			want, err := check.placement(d.et, d.name, probe.Doc)
			switch {
			case err != nil:
				plan.keep(d.path, fmt.Sprintf("the fetch exports %s %q at %s, but its placement could not be confirmed: %v", doctype, d.name, to, err))
			case want != to:
				plan.keep(d.path, fmt.Sprintf("the fetch exports %s %q at %s, but the site places it at %s", doctype, d.name, to, want))
			case off:
				plan.keep(d.path, fmt.Sprintf("%s %q moved to %s, but its old module is marked sync = false", doctype, d.name, to))
			default:
				plan.remove(d.path, "moved to "+to)
			}
		default:
			plan.keep(d.path, fmt.Sprintf("the site still has %s %q and would export it, but the fetch did not list it", doctype, d.name))
		}
	}

	for _, rf := range files {
		rowsKey := aggregatedRows[rf.et]
		doctype := entityTypeDoctype(rf.et)
		held, off := hold(rf.et, rf.path), isUnsynced(rf.path)
		var kept []any
		var why []string
		for _, row := range rf.missing {
			n := getString(row, "name")
			unit := entityKey(rf.et, n)
			probe := check.get(doctype, n)
			reason := ""
			switch {
			case probe.Gone && deletedBy(unit) != "":
			case probe.Err != nil:
				reason = fmt.Sprintf("could not ask the site for %s %q (%v)", doctype, n, probe.Err)
			case probe.Gone && off:
				reason = fmt.Sprintf("the site has no %s %q, but %s", doctype, n, unsyncedGone)
			case probe.Gone && ev.HasBaseline && !ev.Local[rf.path]:
			case probe.Gone:
				reason = fmt.Sprintf("the site has no %s %q and recorded no deletion of it in this pull's window, and %s", doctype, n, noBaseline)
			case !exportableRow(rf.et, probe.Doc):
				if off {
					reason = fmt.Sprintf("the site no longer exports %s %q, but the module is marked sync = false", doctype, n)
				}
			case rowTarget(rf.et, probe.Doc) != rf.target && rowsNow[unit]:
				// Exported in its own DocType's file now.
			default:
				reason = fmt.Sprintf("the site still has %s %q and would export it, but the fetch did not list it", doctype, n)
			}
			if reason != "" {
				kept = append(kept, row)
				why = append(why, reason)
			}
		}

		e := aggregated[entityKey(rf.et, rf.target)]
		switch {
		case e != nil && e.FilePath == rf.path:
			mergeRows(e, rowsKey, kept)
			for _, w := range why {
				plan.keep(rf.path, "kept a row: "+w)
			}
		case e != nil:
			want, err := check.placement(rf.et, rf.target, nil)
			switch {
			case err != nil:
				plan.keep(rf.path, fmt.Sprintf("the fetch exports the %s rows of %s at %s, but their placement could not be confirmed: %v", doctype, rf.target, e.FilePath, err))
			case want != e.FilePath:
				plan.keep(rf.path, fmt.Sprintf("the fetch exports the %s rows of %s at %s, but the site places them at %s", doctype, rf.target, e.FilePath, want))
			case held != "":
				plan.keep(rf.path, held)
			case off:
				plan.keep(rf.path, fmt.Sprintf("the %s rows of %s moved to %s, but the old module is marked sync = false", doctype, rf.target, e.FilePath))
			default:
				mergeRows(e, rowsKey, kept)
				for _, w := range why {
					plan.keep(e.FilePath, "kept a row: "+w)
				}
				plan.remove(rf.path, "moved to "+e.FilePath)
			}
		case held != "":
			plan.keep(rf.path, held)
		case len(kept) > 0:
			for _, w := range why {
				plan.keep(rf.path, w)
			}
		default:
			plan.remove(rf.path, fmt.Sprintf("the site no longer has its %s rows", doctype))
		}
	}

	sort.SliceStable(plan.Removals, func(i, j int) bool { return plan.Removals[i].Path < plan.Removals[j].Path })
	return plan
}

func (p *StalePlan) keep(path, why string) { p.Kept = append(p.Kept, path+": "+why) }

func (p *StalePlan) remove(path, why string) {
	p.Removals = append(p.Removals, Removal{Path: path, Reason: why})
}

// mergeRows adds rows to a fetched Custom Field / Property Setter file.
func mergeRows(e *Entity, rowsKey string, rows []any) {
	if len(rows) == 0 {
		return
	}
	e.Data[rowsKey] = append(anyRows(e.Data[rowsKey]), rows...)
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

// exportableRow reports whether the fetch would export a Custom Field or
// Property Setter row the site has: false for a system-generated field.
func exportableRow(et EntityType, row map[string]any) bool {
	return et != EntityCustomField || !truthy(row["is_system_generated"])
}

// rowTarget is the DocType a Custom Field or Property Setter row belongs to.
func rowTarget(et EntityType, row map[string]any) string {
	if et == EntityPropertySetter {
		return getString(row, "doc_type")
	}
	return getString(row, "dt")
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
