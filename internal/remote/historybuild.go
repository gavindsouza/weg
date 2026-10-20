/*
Copyright © 2025 Gavin <me@gavv.in>

Streaming history builder: replays staged Version JSONL in chronological order,
forward-reconstructing each entity's state and yielding one commit per version.
The JSONL is read twice (a planning pass, then the replay). Memory is bounded
to live entity states (O(entities)), not O(versions), except for the diffs kept
to rebuild a state backwards from the current one: those of Custom Field and
Property Setter rows (small), and on a pull those of the few documents missing
from the committed export.
*/
package remote

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// ReconstructedCommit is one git commit's worth of reconstructed state, yielded
// by StreamHistory for the caller to write and commit.
type ReconstructedCommit struct {
	FilePath    string         // relative path to write
	Content     map[string]any // the whole file's reconstructed state at this point
	Author      string         // "Full Name <email>" for git --author
	Timestamp   string         // Version.creation as recorded (naive site time)
	When        time.Time      // Timestamp in the site's time zone; zero if unparseable
	Message     string         // conventional-commit message
	VersionName string         // source Version docname; "" for a creation commit
}

// HistoryOptions controls which Versions StreamHistory replays and what state
// each document starts from.
type HistoryOptions struct {
	// Window bounds the replayed Versions and carries the site's time zone.
	Window HistoryWindow

	// Baseline is the committed export a pull starts from: file path (as
	// Entity.FilePath) to file content. Nil means a clone, which replays from
	// nothing. With a baseline, each document's replay starts from its
	// committed file (or, if it has none, from its current state with the
	// window's changes undone), a document created inside the window gets a
	// creation commit by its owner, and Versions of documents that are
	// neither committed nor exported now are dropped instead of committed.
	Baseline map[string][]byte
}

// HistoryResult summarises a StreamHistory run.
type HistoryResult struct {
	SeenPaths     map[string]bool // file paths history wrote (for reconcile)
	OutsideWindow int             // Versions outside the window (already replayed by an earlier pull)
	Unexported    int             // Versions of documents outside the export (system generated, deleted unseen)
	Dropped       []string        // Versions whose data could not be parsed, one reason each
}

// aggregatedRows names the row list of the entity types that are exported as
// one file per target DocType rather than one file per document.
var aggregatedRows = map[EntityType]string{
	EntityCustomField:    "custom_fields",
	EntityPropertySetter: "property_setters",
}

// StreamHistory merges the staged per-doctype JSONL and calls emit once per
// replayed version, in global chronological order, with the whole file's
// reconstructed state. customDoctypes gates DocType versions to
// confirmed-custom docnames; other tracked doctypes are customizations by
// nature and always included. users resolves author emails to names (may be nil).
//
// A document's last change in the replay is committed as the document is
// now (entities), so the trailing reconcile commit only carries what no
// Version explains (deletions, untracked fields). Custom Field and Property
// Setter Versions update their row in the per-DocType file.
//
// Versions of documents outside the export are counted, and a Version whose
// data cannot be parsed is dropped and named, in the result; the error return
// is reserved for I/O failures and emit errors.
func (f *Fetcher) StreamHistory(
	tmpDir string,
	customDoctypes map[string]bool,
	entities []Entity,
	users func(emails []string) map[string]UserInfo,
	opts HistoryOptions,
	emit func(ReconstructedCommit) error,
) (*HistoryResult, error) {
	h := newHistoryIndex(entities, opts.Baseline)
	w := &historyWalker{
		f:              f,
		tmpDir:         tmpDir,
		customDoctypes: customDoctypes,
		index:          h,
		window:         opts.Window,
	}
	if !h.clone {
		w.synthetic = h.creations(opts.Window)
	}

	// Pass 1: find each document's last change, the owners to resolve, and
	// the changes needed to rebuild uncommitted documents backwards.
	last := make(map[string]string)
	undo := make(map[string][]map[string]any)
	owners := make(map[string]bool)
	if _, err := w.walk(func(ev historyEvent) error {
		last[ev.ref.unit] = ev.id()
		if ev.rec.Owner != "" {
			owners[ev.rec.Owner] = true
		}
		if !ev.created && h.rebuildsBackwards(ev.ref) {
			undo[ev.ref.unit] = append(undo[ev.ref.unit], ev.data)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	userInfo := map[string]UserInfo{}
	if users != nil && len(owners) > 0 {
		emails := make([]string, 0, len(owners))
		for o := range owners {
			emails = append(emails, o)
		}
		sort.Strings(emails)
		if u := users(emails); u != nil {
			userInfo = u
		}
	}

	// Pass 2: replay.
	result := &HistoryResult{SeenPaths: make(map[string]bool)}
	files := make(map[string]map[string]any)
	stats, err := w.walk(func(ev historyEvent) error {
		ref := ev.ref
		state, ok := files[ref.file]
		if !ok {
			state = h.initialFile(ref, undo)
		}
		isLast := last[ref.unit] == ev.id()

		var entityData map[string]any
		if rowsKey, agg := aggregatedRows[ref.et]; agg {
			row := findRow(state[rowsKey], ref.docname)
			if row == nil {
				row = h.initialRow(ref, undo)
				state[rowsKey] = insertRow(state[rowsKey], row, h.rowOrder[ref.file])
			}
			if ev.created {
				stampCreation(row, ev.rec)
			} else {
				applyVersionForward(row, ev.data)
			}
			if cur, ok := h.currentRows[ref.unit]; ok && isLast {
				row = deepCopyMap(cur)
				state[rowsKey] = replaceRow(state[rowsKey], row)
			}
			entityData = row
		} else {
			if ev.created {
				stampCreation(state, ev.rec)
			} else {
				state = applyVersionForward(state, ev.data)
			}
			if cur, ok := h.current[ref.file]; ok && isLast {
				state = deepCopyMap(cur.Data)
			}
			entityData = state
		}
		files[ref.file] = state
		if len(state) == 0 {
			// An insert Version replayed from nothing: no content to commit.
			return nil
		}

		filePath := h.path(ref)
		result.SeenPaths[filePath] = true

		action := ""
		if ev.created || isInsertVersion(ev.data) {
			action = "create"
		}
		entry := HistoryEntry{
			Timestamp:   ev.rec.Creation,
			Author:      ev.rec.Owner,
			DocType:     ev.rec.RefDoctype,
			DocName:     ref.docname,
			Action:      action,
			VersionData: ev.rec.Data,
			VersionName: ev.rec.Name,
			EntityType:  ref.et,
			Module:      h.module(ref),
			FilePath:    filePath,
			EntityData:  entityData,
		}
		return emit(ReconstructedCommit{
			FilePath:    filePath,
			Content:     deepCopyMap(state),
			Author:      formatAuthor(ev.rec.Owner, userInfo),
			Timestamp:   ev.rec.Creation,
			When:        ev.when,
			Message:     generateCommitMessage(entry),
			VersionName: ev.rec.Name,
		})
	})
	result.OutsideWindow = stats.outsideWindow
	result.Unexported = stats.unexported
	result.Dropped = stats.dropped
	return result, err
}

// historyEvent is one Version (or a synthesized creation) resolved to the
// document and file it changes.
type historyEvent struct {
	rec     VersionRecord
	data    map[string]any
	ref     unitRef
	when    time.Time
	created bool // synthesized from the document's own creation/owner
}

// id identifies the event among a document's changes.
func (ev historyEvent) id() string {
	if ev.created {
		return "+created"
	}
	return ev.rec.Name
}

// unitRef identifies what a Version is about: the document (unit) and the
// exported file holding it. They differ only for Custom Field and Property
// Setter rows, which live in their target DocType's file.
type unitRef struct {
	unit    string // "<type>:<docname>"
	file    string // "<type>:<entity name>" of the exported file
	et      EntityType
	docname string
}

func entityKey(et EntityType, name string) string { return string(et) + ":" + name }

type walkStats struct {
	outsideWindow int
	unexported    int
	dropped       []string
}

// historyWalker merges the staged Version streams (plus any synthesized
// creations) and resolves each record to a historyEvent.
type historyWalker struct {
	f              *Fetcher
	tmpDir         string
	customDoctypes map[string]bool
	index          *historyIndex
	window         HistoryWindow
	synthetic      []VersionRecord
}

func (w *historyWalker) walk(fn func(historyEvent) error) (walkStats, error) {
	var stats walkStats

	var readers []io.Reader
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()
	for _, dt := range w.f.enabledVersionedDoctypes() {
		file, err := os.Open(versionJSONLPath(w.tmpDir, dt))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return stats, err
		}
		readers = append(readers, file)
		closers = append(closers, file)
	}
	if len(w.synthetic) > 0 {
		var buf bytes.Buffer
		for _, rec := range w.synthetic {
			line, err := json.Marshal(rec)
			if err != nil {
				return stats, err
			}
			buf.Write(line)
			buf.WriteByte('\n')
		}
		readers = append(readers, &buf)
	}

	err := mergeVersions(readers, func(v VersionRecord) error {
		et, ok := DoctypeToEntityType[v.RefDoctype]
		if !ok {
			return nil
		}
		// DocType history is dominated by framework churn; keep only the
		// docnames confirmed custom (current custom set + confirmed-deleted).
		if et == EntityDocType && !w.customDoctypes[v.Docname] {
			return nil
		}

		when, hasWhen := w.window.ParseSiteTime(v.Creation)
		if !v.Synthetic && hasWhen && !w.window.Contains(when) {
			stats.outsideWindow++
			return nil
		}

		ref, ok := w.index.resolve(et, v.Docname)
		if !ok {
			stats.unexported++
			return nil
		}

		var data map[string]any
		if v.Data != "" {
			if err := json.Unmarshal([]byte(v.Data), &data); err != nil {
				stats.dropped = append(stats.dropped,
					fmt.Sprintf("version %s (%s/%s): bad data JSON: %v", v.Name, v.RefDoctype, v.Docname, err))
				return nil
			}
		}
		return fn(historyEvent{rec: v, data: data, ref: ref, when: when, created: v.Synthetic})
	})
	return stats, err
}

// historyIndex knows which documents exist now (entities) and in the
// committed export (baseline), and where each one's file lives.
type historyIndex struct {
	clone       bool                      // no baseline: replay from nothing
	current     map[string]Entity         // file key -> current entity
	currentRows map[string]map[string]any // row unit -> current row
	rowFile     map[string]string         // row unit -> file key (current, else committed)
	rowOrder    map[string]map[string]int // file key -> row name -> position now
	basePath    map[string]string         // file key -> committed path
	baseDoc     map[string]map[string]any // file key -> committed content
	baseRows    map[string]bool           // row units present in committed files
}

func newHistoryIndex(entities []Entity, baseline map[string][]byte) *historyIndex {
	h := &historyIndex{
		clone:       baseline == nil,
		current:     make(map[string]Entity),
		currentRows: make(map[string]map[string]any),
		rowFile:     make(map[string]string),
		rowOrder:    make(map[string]map[string]int),
		basePath:    make(map[string]string),
		baseDoc:     make(map[string]map[string]any),
		baseRows:    make(map[string]bool),
	}
	for _, e := range entities {
		key := entityKey(e.Type, e.Name)
		h.current[key] = e
		rowsKey, agg := aggregatedRows[e.Type]
		if !agg {
			continue
		}
		order := make(map[string]int)
		for i, row := range rowList(e.Data[rowsKey]) {
			name := getString(row, "name")
			if name == "" {
				continue
			}
			unit := entityKey(e.Type, name)
			h.currentRows[unit] = row
			h.rowFile[unit] = key
			order[name] = i
		}
		h.rowOrder[key] = order
	}

	paths := make([]string, 0, len(baseline))
	for p := range baseline {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		et, ok := entityTypeOfPath(p)
		if !ok {
			continue
		}
		var doc map[string]any
		if json.Unmarshal(baseline[p], &doc) != nil || doc == nil {
			continue
		}
		var key string
		rowsKey, agg := aggregatedRows[et]
		if agg {
			key = entityKey(et, getString(doc, "doctype"))
		} else {
			key = entityKey(et, getString(doc, "name"))
		}
		if strings.HasSuffix(key, ":") {
			continue
		}
		// A document committed at two paths (moved module): prefer the
		// path it is exported at now.
		if prev, dup := h.basePath[key]; dup && (prev == h.currentPath(key) || p != h.currentPath(key)) {
			continue
		}
		h.basePath[key] = p
		h.baseDoc[key] = doc
		if !agg {
			continue
		}
		for _, row := range rowList(doc[rowsKey]) {
			name := getString(row, "name")
			if name == "" {
				continue
			}
			unit := entityKey(et, name)
			h.baseRows[unit] = true
			if _, ok := h.rowFile[unit]; !ok {
				h.rowFile[unit] = key
			}
		}
	}
	return h
}

func (h *historyIndex) currentPath(key string) string {
	if e, ok := h.current[key]; ok {
		return e.FilePath
	}
	return ""
}

// resolve maps a Version's document to its unit and file. It fails for a
// Custom Field / Property Setter row that is in no exported file (system
// generated, or created and deleted unseen) and, on a pull, for any document
// that is neither committed nor exported now.
func (h *historyIndex) resolve(et EntityType, docname string) (unitRef, bool) {
	unit := entityKey(et, docname)
	if _, agg := aggregatedRows[et]; agg {
		file, ok := h.rowFile[unit]
		if !ok {
			return unitRef{}, false
		}
		return unitRef{unit: unit, file: file, et: et, docname: docname}, true
	}
	if !h.clone {
		_, cur := h.current[unit]
		_, base := h.basePath[unit]
		if !cur && !base {
			return unitRef{}, false
		}
	}
	return unitRef{unit: unit, file: unit, et: et, docname: docname}, true
}

// path is the file a unit's changes are written to: where the document is
// exported now, else where it was committed, else (a clone replaying a
// deleted document) under the catch-all module.
func (h *historyIndex) path(ref unitRef) string {
	if e, ok := h.current[ref.file]; ok {
		return e.FilePath
	}
	if p, ok := h.basePath[ref.file]; ok {
		return p
	}
	// ponytail: unresolved module for vanished entities lands under _/.
	return buildFilePath(ref.et, ref.docname, "_")
}

func (h *historyIndex) module(ref unitRef) string {
	if e, ok := h.current[ref.file]; ok {
		return e.Module
	}
	return "_"
}

// rebuildsBackwards reports whether a unit's starting state is derived from
// its current state by undoing the replayed changes: a row exported now but
// not committed, or, on a pull, a document exported now but not committed.
func (h *historyIndex) rebuildsBackwards(ref unitRef) bool {
	if _, agg := aggregatedRows[ref.et]; agg {
		_, cur := h.currentRows[ref.unit]
		return cur && !h.baseRows[ref.unit]
	}
	if h.clone {
		return false
	}
	_, cur := h.current[ref.unit]
	_, base := h.basePath[ref.unit]
	return cur && !base
}

// initialFile is a file's state before the first replayed change to it.
func (h *historyIndex) initialFile(ref unitRef, undo map[string][]map[string]any) map[string]any {
	if doc, ok := h.baseDoc[ref.file]; ok {
		return deepCopyMap(doc)
	}
	if rowsKey, agg := aggregatedRows[ref.et]; agg {
		target := strings.TrimPrefix(ref.file, string(ref.et)+":")
		return map[string]any{"doctype": target, rowsKey: []any{}}
	}
	if h.rebuildsBackwards(ref) {
		return undoChanges(h.current[ref.file].Data, undo[ref.unit])
	}
	return map[string]any{}
}

// initialRow is a Custom Field / Property Setter row's state before its first
// replayed change, for a row its file does not hold yet.
func (h *historyIndex) initialRow(ref unitRef, undo map[string][]map[string]any) map[string]any {
	if cur, ok := h.currentRows[ref.unit]; ok {
		return undoChanges(cur, undo[ref.unit])
	}
	return map[string]any{"name": ref.docname}
}

// creations synthesizes a creation record for every document (or row)
// exported now, missing from the baseline, and created inside the window.
// Frappe writes no Version on insert, so without these a new document would
// first appear in whichever later change touched it, credited to that editor.
func (h *historyIndex) creations(window HistoryWindow) []VersionRecord {
	var recs []VersionRecord
	add := func(et EntityType, docname string, doc map[string]any) {
		creation := getString(doc, "creation")
		t, ok := window.ParseSiteTime(creation)
		if !ok || (!window.Since.IsZero() && !t.After(window.Since)) {
			return
		}
		recs = append(recs, VersionRecord{
			RefDoctype: entityTypeDoctype(et),
			Docname:    docname,
			Owner:      getString(doc, "owner"),
			Creation:   creation,
			Synthetic:  true,
		})
	}
	for key, e := range h.current {
		if _, agg := aggregatedRows[e.Type]; agg || entityTypeDoctype(e.Type) == "" {
			continue
		}
		if _, base := h.basePath[key]; !base {
			add(e.Type, e.Name, e.Data)
		}
	}
	for unit, row := range h.currentRows {
		if !h.baseRows[unit] {
			add(h.current[h.rowFile[unit]].Type, getString(row, "name"), row)
		}
	}
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Creation != recs[j].Creation {
			return recs[i].Creation < recs[j].Creation
		}
		return recs[i].RefDoctype+recs[i].Docname < recs[j].RefDoctype+recs[j].Docname
	})
	return recs
}

// stampCreation sets a document rebuilt for its creation commit to how it
// was saved on insert: Frappe sets modified to the creation time and
// modified_by to the owner. Fields the export does not carry stay absent.
func stampCreation(doc map[string]any, rec VersionRecord) {
	if _, ok := doc["modified"]; ok {
		doc["modified"] = rec.Creation
	}
	if _, ok := doc["modified_by"]; ok && rec.Owner != "" {
		doc["modified_by"] = rec.Owner
	}
}

// undoChanges returns a copy of doc with the given Version diffs (oldest
// first) undone, newest first.
func undoChanges(doc map[string]any, diffs []map[string]any) map[string]any {
	state := deepCopyMap(doc)
	for i := len(diffs) - 1; i >= 0; i-- {
		state = revertVersion(state, diffs[i])
	}
	return state
}

// entityTypeOfPath recognises an exported entity file by its layout:
// <module>/<type>/<name>.json, or <module>/<type>/<name>/<name>.json for
// DocTypes and Reports.
func entityTypeOfPath(p string) (EntityType, bool) {
	parts := strings.Split(p, "/")
	if len(parts) < 3 || !strings.HasSuffix(p, ".json") || strings.HasPrefix(parts[0], ".") {
		return "", false
	}
	et := EntityType(parts[1])
	if entityTypeDoctype(et) == "" {
		return "", false
	}
	switch et {
	case EntityDocType, EntityReport:
		if len(parts) != 4 || parts[2]+".json" != parts[3] {
			return "", false
		}
	default:
		if len(parts) != 3 {
			return "", false
		}
	}
	return et, true
}

// entityTypeDoctype is the Frappe doctype an entity type's Versions carry.
func entityTypeDoctype(et EntityType) string {
	for dt, t := range DoctypeToEntityType {
		if t == et {
			return dt
		}
	}
	return ""
}

// rowList returns the map rows of a child-table-like list value: []any as
// decoded from JSON, or []map[string]any as the fetcher builds the
// per-DocType Custom Field / Property Setter lists.
func rowList(v any) []map[string]any {
	if rows, ok := v.([]map[string]any); ok {
		return rows
	}
	list, _ := v.([]any)
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if row, ok := item.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func findRow(list any, name string) map[string]any {
	for _, row := range rowList(list) {
		if getString(row, "name") == name {
			return row
		}
	}
	return nil
}

// anyRows returns a row list as []any, converting the fetcher's
// []map[string]any shape.
func anyRows(list any) []any {
	if typed, ok := list.([]map[string]any); ok {
		rows := make([]any, len(typed))
		for i, r := range typed {
			rows[i] = r
		}
		return rows
	}
	rows, _ := list.([]any)
	return rows
}

func replaceRow(list any, row map[string]any) []any {
	rows := anyRows(list)
	name := getString(row, "name")
	for i, item := range rows {
		if r, ok := item.(map[string]any); ok && getString(r, "name") == name {
			rows[i] = row
		}
	}
	return rows
}

// insertRow adds row to list and orders the rows as the current export
// does (order: row name -> position); rows it does not know keep their
// relative order at the end.
func insertRow(list any, row map[string]any, order map[string]int) []any {
	rows := append(anyRows(list), row)
	pos := func(item any) int {
		if r, ok := item.(map[string]any); ok {
			if i, ok := order[getString(r, "name")]; ok {
				return i
			}
		}
		return len(order)
	}
	sort.SliceStable(rows, func(i, j int) bool { return pos(rows[i]) < pos(rows[j]) })
	return rows
}

// ResolveCustomDoctypes returns the set of DocType docnames whose history is in
// scope: every currently-custom DocType, plus any deleted DocType confirmed to
// have been custom via Frappe's Deleted Document trash. currentCustom is the set
// of DocType names fetched with custom=1.
func (f *Fetcher) ResolveCustomDoctypes(tmpDir string, currentCustom map[string]bool) (map[string]bool, error) {
	result := make(map[string]bool, len(currentCustom))
	for k := range currentCustom {
		result[k] = true
	}

	// Gather DocType docnames seen in history that we can't already vouch for.
	file, err := os.Open(versionJSONLPath(tmpDir, "DocType"))
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	unknown := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var v VersionRecord
		if json.Unmarshal(scanner.Bytes(), &v) == nil && v.Docname != "" && !result[v.Docname] {
			unknown[v.Docname] = true
		}
	}
	file.Close()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(unknown) == 0 {
		return result, nil
	}

	// Confirm via trash. Deleted doctypes are few, so fetch them all and
	// intersect rather than sending a huge `in` filter.
	// ponytail: if trash is disabled/pruned, unprovable names are skipped.
	deleted, err := f.Client.GetAll("Deleted Document",
		map[string]any{"deleted_doctype": "DocType"},
		[]string{"deleted_name", "data"})
	if err != nil {
		return result, nil
	}
	for _, d := range deleted {
		name := getString(d, "deleted_name")
		if !unknown[name] {
			continue
		}
		var doc map[string]any
		if json.Unmarshal([]byte(getString(d, "data")), &doc) == nil && isTruthy(doc["custom"]) {
			result[name] = true
		}
	}
	return result, nil
}
