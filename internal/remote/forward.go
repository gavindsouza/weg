/*
Copyright © 2025 Gavin <me@gavv.in>

Forward reconstruction of entity state from Frappe Version diffs.

The streaming history builder applies versions chronologically (oldest first),
maintaining live state per entity. Unlike backward-from-current reconstruction,
it needs no current state — so it can rebuild deleted entities whose current
doc no longer exists.

A Version's data is the diff Frappe's get_diff produced:

	{"changed":     [[field, old, new], ...],
	 "added":       [[table, {row}], ...],
	 "removed":     [[table, {row}], ...],
	 "row_changed": [[table, idx, row_name, [[field, old, new], ...]], ...]}

An insert Version (only written for data imports and similar) carries no diff
keys at all, just creation metadata.
*/
package remote

import "strconv"

// applyVersionForward applies a Version's diff to state, returning the state
// AFTER this version (the inverse direction to backward-from-current
// reconstruction). state is mutated in place and also returned for chaining.
// A nil state starts a fresh entity (creation).
func applyVersionForward(state map[string]any, versionData map[string]any) map[string]any {
	if state == nil {
		state = map[string]any{}
	}

	// Field changes: set field to its new value (arr = [field, old, new]).
	setFields(state, versionData["changed"], 2)

	// Child table additions: add the row.
	if added, ok := versionData["added"].([]any); ok {
		for _, item := range added {
			table, row, ok := childRow(item)
			if !ok {
				continue
			}
			state[table] = appendRow(state[table], row)
		}
	}

	// Child table removals: drop the row by name.
	if removed, ok := versionData["removed"].([]any); ok {
		for _, item := range removed {
			table, row, ok := childRow(item)
			if !ok {
				continue
			}
			if existing, ok := state[table].([]any); ok {
				state[table] = removeRowFromTable(existing, row)
			}
		}
	}

	// Child row edits: set each changed field on the named row.
	applyRowChanges(state, versionData["row_changed"], 2)

	return state
}

// revertVersion undoes a Version's diff on state, returning the state BEFORE
// this version. It is the inverse of applyVersionForward and lets the history
// builder derive a document's state at the start of a pull window from its
// current state when no committed copy exists. state is mutated in place.
func revertVersion(state map[string]any, versionData map[string]any) map[string]any {
	if state == nil {
		state = map[string]any{}
	}

	applyRowChanges(state, versionData["row_changed"], 1)

	// A row this version added did not exist before it.
	if added, ok := versionData["added"].([]any); ok {
		for _, item := range added {
			table, row, ok := childRow(item)
			if !ok {
				continue
			}
			if existing, ok := state[table].([]any); ok {
				state[table] = removeRowFromTable(existing, row)
			}
		}
	}

	// A row this version removed existed before it.
	if removed, ok := versionData["removed"].([]any); ok {
		for _, item := range removed {
			table, row, ok := childRow(item)
			if !ok {
				continue
			}
			state[table] = appendRow(state[table], row)
		}
	}

	setFields(state, versionData["changed"], 1)
	return state
}

// isInsertVersion reports whether a Version records a document's creation
// rather than a change: Frappe's insert Versions carry no diff at all.
func isInsertVersion(versionData map[string]any) bool {
	for _, key := range []string{"changed", "added", "removed", "row_changed"} {
		if list, ok := versionData[key].([]any); ok && len(list) > 0 {
			return false
		}
	}
	return true
}

// setFields applies [[field, old, new], ...] entries to state, taking the
// value at position pick (1 = old, 2 = new).
func setFields(state map[string]any, changes any, pick int) {
	list, ok := changes.([]any)
	if !ok {
		return
	}
	for _, c := range list {
		arr, ok := c.([]any)
		if !ok || len(arr) < 3 {
			continue
		}
		field, ok := arr[0].(string)
		if !ok {
			continue
		}
		state[field] = matchKind(state[field], arr[pick])
	}
}

// applyRowChanges applies row_changed entries ([table, idx, row_name,
// [[field, old, new], ...]]) to the named rows of state, taking the value at
// position pick. Rows not present in state are left alone.
func applyRowChanges(state map[string]any, rowChanges any, pick int) {
	list, ok := rowChanges.([]any)
	if !ok {
		return
	}
	for _, item := range list {
		arr, ok := item.([]any)
		if !ok || len(arr) < 4 {
			continue
		}
		table, ok1 := arr[0].(string)
		rowName, ok2 := arr[2].(string)
		if !ok1 || !ok2 {
			continue
		}
		rows, ok := state[table].([]any)
		if !ok {
			continue
		}
		for _, r := range rows {
			row, ok := r.(map[string]any)
			if !ok || getString(row, "name") != rowName {
				continue
			}
			setFields(row, arr[3], pick)
		}
	}
}

// appendRow adds row to a child table value at its idx position (1-based,
// as Frappe numbers rows), or at the end when it has none. Any row of the
// same name is replaced, so a replayed add is idempotent.
func appendRow(table any, row map[string]any) any {
	existing, _ := table.([]any)
	if name := getString(row, "name"); name != "" {
		existing = removeRowFromTable(existing, row)
	}
	at := len(existing)
	if idx, ok := row["idx"].(float64); ok && idx >= 1 && int(idx)-1 < at {
		at = int(idx) - 1
	}
	out := make([]any, 0, len(existing)+1)
	out = append(out, existing[:at]...)
	out = append(out, row)
	return append(out, existing[at:]...)
}

// matchKind keeps a field's JSON kind stable across a replayed change. Frappe
// records some values in their display form (an Int as "3"), while the
// exported document holds the raw number; when the field currently holds a
// number and the recorded value is a numeric string, keep it a number so an
// untouched field does not flip between 3 and "3" from commit to commit.
func matchKind(current, value any) any {
	if _, isNum := current.(float64); !isNum {
		return value
	}
	s, ok := value.(string)
	if !ok {
		return value
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n
	}
	return value
}

// childRow unpacks an added/removed entry of the form [tableName, rowData].
func childRow(item any) (table string, row map[string]any, ok bool) {
	arr, ok := item.([]any)
	if !ok || len(arr) < 2 {
		return "", nil, false
	}
	table, ok = arr[0].(string)
	if !ok {
		return "", nil, false
	}
	row, ok = arr[1].(map[string]any)
	if !ok {
		return "", nil, false
	}
	return table, row, true
}
