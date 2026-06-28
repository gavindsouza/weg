/*
Copyright © 2025 Gavin <me@gavv.in>

Forward reconstruction of entity state from Frappe Version diffs.

The streaming history builder applies versions chronologically (oldest first),
maintaining live state per entity. Unlike backward-from-current reconstruction,
it needs no current state — so it can rebuild deleted entities whose current
doc no longer exists.
*/
package remote

// applyVersionForward applies a Version's diff to state, returning the state
// AFTER this version (the inverse direction to backward-from-current
// reconstruction). state is mutated in place and also returned for chaining.
// A nil state starts a fresh entity (creation).
func applyVersionForward(state map[string]any, versionData map[string]any) map[string]any {
	if state == nil {
		state = map[string]any{}
	}

	// Field changes: set field to its new value (arr = [field, old, new]).
	if changed, ok := versionData["changed"].([]any); ok {
		for _, c := range changed {
			arr, ok := c.([]any)
			if !ok || len(arr) < 3 {
				continue
			}
			field, ok := arr[0].(string)
			if !ok {
				continue
			}
			state[field] = arr[2]
		}
	}

	// Child table additions: add the row.
	if added, ok := versionData["added"].([]any); ok {
		for _, item := range added {
			table, row, ok := childRow(item)
			if !ok {
				continue
			}
			if existing, ok := state[table].([]any); ok {
				state[table] = append(existing, row)
			} else {
				state[table] = []any{row}
			}
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

	return state
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
