/*
Copyright © 2025 Gavin <me@gavv.in>

Streaming history builder: replays staged Version JSONL in chronological order,
forward-reconstructing each entity's state and yielding one commit per version.
Memory is bounded to live entity states (O(entities)), not O(versions).
*/
package remote

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

// ReconstructedCommit is one git commit's worth of reconstructed state, yielded
// by StreamHistory for the caller to write and commit.
type ReconstructedCommit struct {
	FilePath    string         // relative path to write
	Content     map[string]any // reconstructed entity state at this point
	Author      string         // "Full Name <email>" for git --author
	Timestamp   string         // Version.creation, for git --date
	Message     string         // conventional-commit message
	VersionName string         // source Version docname (traceability)
}

// StreamHistory merges the staged per-doctype JSONL and calls emit once per
// version, in global chronological order, with the entity's forward-reconstructed
// state. customDoctypes gates DocType versions to confirmed-custom docnames;
// other tracked doctypes are customizations by nature and always included.
// Returns the set of file paths history touched (for reconcile) and the first error.
func (f *Fetcher) StreamHistory(
	tmpDir string,
	customDoctypes map[string]bool,
	entities []Entity,
	users map[string]UserInfo,
	emit func(ReconstructedCommit) error,
) (map[string]bool, error) {
	entityMap := make(map[string]Entity, len(entities))
	for _, e := range entities {
		entityMap[string(e.Type)+":"+e.Name] = e
	}

	var readers []io.Reader
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()
	for _, dt := range VersionedDoctypes() {
		file, err := os.Open(versionJSONLPath(tmpDir, dt))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		readers = append(readers, file)
		closers = append(closers, file)
	}

	liveState := make(map[string]map[string]any)
	seenPaths := make(map[string]bool)

	err := mergeVersions(readers, func(v VersionRecord) error {
		et, ok := DoctypeToEntityType[v.RefDoctype]
		if !ok {
			return nil
		}
		// DocType history is dominated by framework churn; keep only the
		// docnames confirmed custom (current custom set + confirmed-deleted).
		if et == EntityDocType && !customDoctypes[v.Docname] {
			return nil
		}

		key := string(et) + ":" + v.Docname
		module, filePath := "_", ""
		if e, exists := entityMap[key]; exists {
			module, filePath = e.Module, e.FilePath
		} else {
			// Deleted/renamed entity: module unknown, file it under "_".
			// ponytail: unresolved module for vanished entities lands under _/.
			filePath = buildFilePath(et, v.Docname, module)
		}

		var vdata map[string]any
		if v.Data != "" {
			json.Unmarshal([]byte(v.Data), &vdata)
		}
		state := applyVersionForward(liveState[key], vdata)
		liveState[key] = state
		seenPaths[filePath] = true

		entry := HistoryEntry{
			Timestamp:   v.Creation,
			Author:      v.Owner,
			DocType:     v.RefDoctype,
			DocName:     v.Docname,
			VersionData: v.Data,
			VersionName: v.Name,
			EntityType:  et,
			Module:      module,
			FilePath:    filePath,
			EntityData:  state,
		}

		return emit(ReconstructedCommit{
			FilePath:    filePath,
			Content:     deepCopyMap(state),
			Author:      formatAuthor(v.Owner, users),
			Timestamp:   v.Creation,
			Message:     generateCommitMessage(entry),
			VersionName: v.Name,
		})
	})
	return seenPaths, err
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

// CollectVersionOwners scans the staged JSONL and returns the unique author
// emails, so their full names can be resolved before replaying commits.
func CollectVersionOwners(tmpDir string) ([]string, error) {
	seen := make(map[string]bool)
	for _, dt := range VersionedDoctypes() {
		file, err := os.Open(versionJSONLPath(tmpDir, dt))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
		for scanner.Scan() {
			var v VersionRecord
			if json.Unmarshal(scanner.Bytes(), &v) == nil && v.Owner != "" {
				seen[v.Owner] = true
			}
		}
		file.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	owners := make([]string, 0, len(seen))
	for o := range seen {
		owners = append(owners, o)
	}
	return owners, nil
}
