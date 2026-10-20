package remote

import (
	"encoding/json"
	"testing"
)

// diff is a tiny helper to build a Version.data payload for tests.
func diff(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("bad diff json: %v", err)
	}
	return m
}

// Full lifecycle of a deleted entity, reconstructed forward from nothing:
// create → change a field → add a child row → remove that child row.
// This is the core design assumption: no current state is needed.
func TestApplyVersionForward_Lifecycle(t *testing.T) {
	var state map[string]any // nil == entity does not exist yet

	// creation: first version sets initial fields
	state = applyVersionForward(state, diff(t, `{"changed":[["label","","Todo"],["module","","Custom"]]}`))
	if state["label"] != "Todo" || state["module"] != "Custom" {
		t.Fatalf("after create: %v", state)
	}

	// change a field to a new value
	state = applyVersionForward(state, diff(t, `{"changed":[["label","Todo","Task"]]}`))
	if state["label"] != "Task" {
		t.Fatalf("after change: label=%v", state["label"])
	}
	if state["module"] != "Custom" {
		t.Fatalf("unrelated field lost: %v", state["module"])
	}

	// add a child row
	state = applyVersionForward(state, diff(t, `{"added":[["fields",{"name":"r1","fieldname":"priority"}]]}`))
	rows, _ := state["fields"].([]any)
	if len(rows) != 1 {
		t.Fatalf("after add: rows=%v", state["fields"])
	}

	// remove the child row by name
	state = applyVersionForward(state, diff(t, `{"removed":[["fields",{"name":"r1"}]]}`))
	if rows, _ := state["fields"].([]any); len(rows) != 0 {
		t.Fatalf("after remove: rows=%v", state["fields"])
	}
}

// Adding two rows then removing one leaves exactly the other.
func TestApplyVersionForward_ChildRowIdentity(t *testing.T) {
	state := applyVersionForward(nil, diff(t, `{"added":[["fields",{"name":"a","fieldname":"x"}],["fields",{"name":"b","fieldname":"y"}]]}`))
	state = applyVersionForward(state, diff(t, `{"removed":[["fields",{"name":"a"}]]}`))
	rows, _ := state["fields"].([]any)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %v", rows)
	}
	row := rows[0].(map[string]any)
	if row["name"] != "b" {
		t.Fatalf("wrong row survived: %v", row)
	}
}

// Malformed diffs must be ignored, not panic.
func TestApplyVersionForward_Malformed(t *testing.T) {
	state := applyVersionForward(nil, diff(t, `{"changed":[["only-two","x"]],"added":["not-an-array"],"removed":[[123,{}]]}`))
	if len(state) != 0 {
		t.Fatalf("malformed diff mutated state: %v", state)
	}
}

// revertVersion is the inverse of applyVersionForward: undoing a diff on the
// state after it gives back the state before it, for every kind of change.
func TestRevertVersion_InvertsForward(t *testing.T) {
	rowA := map[string]any{"name": "a", "label": "A", "idx": float64(1)}
	rowB := map[string]any{"name": "b", "label": "B", "idx": float64(2)}
	tests := []struct {
		name   string
		before map[string]any
		diff   string
	}{
		{"field change", map[string]any{"script": "x", "n": float64(1)},
			`{"changed":[["script","x","y"]]}`},
		{"row edit", map[string]any{"fields": []any{rowA, rowB}},
			`{"row_changed":[["fields",1,"b",[["label","B","BB"]]]]}`},
		{"row added", map[string]any{"fields": []any{rowA}},
			`{"added":[["fields",{"name":"b","label":"B","idx":2}]]}`},
		{"first row removed", map[string]any{"fields": []any{rowA, rowB}},
			`{"removed":[["fields",{"name":"a","label":"A","idx":1}]]}`},
		{"last row removed", map[string]any{"fields": []any{rowA}},
			`{"removed":[["fields",{"name":"a","label":"A","idx":1}]]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			after := applyVersionForward(deepCopyMap(tt.before), diff(t, tt.diff))
			back := revertVersion(deepCopyMap(after), diff(t, tt.diff))
			want, _ := json.Marshal(tt.before)
			got, _ := json.Marshal(back)
			if string(got) != string(want) {
				t.Fatalf("revert(apply(x)) = %s, want %s (after = %v)", got, want, after)
			}
		})
	}
}

// Removing a table's last row leaves an empty list, as the export has it,
// not null.
func TestApplyVersionForward_EmptiedTableStaysList(t *testing.T) {
	state := applyVersionForward(map[string]any{"fields": []any{map[string]any{"name": "a"}}},
		diff(t, `{"removed":[["fields",{"name":"a"}]]}`))
	b, _ := json.Marshal(state)
	if string(b) != `{"fields":[]}` {
		t.Fatalf("got %s, want an empty list", b)
	}
}

// Frappe records some values in display form; a numeric string replayed
// onto a numeric field stays a number, anything else is taken as recorded.
func TestApplyVersionForward_KeepsNumericKind(t *testing.T) {
	tests := []struct {
		current any
		value   any
		want    any
	}{
		{float64(3), "4", float64(4)},
		{float64(3), "4.5", 4.5},
		{float64(3), "", ""},
		{float64(3), "abc", "abc"},
		{"3", "4", "4"},
		{nil, "4", "4"},
		{float64(0), float64(1), float64(1)},
	}
	for _, tt := range tests {
		state := applyVersionForward(map[string]any{"f": tt.current},
			map[string]any{"changed": []any{[]any{"f", tt.current, tt.value}}})
		if state["f"] != tt.want {
			t.Errorf("current %#v + %#v = %#v, want %#v", tt.current, tt.value, state["f"], tt.want)
		}
	}
}

func TestIsInsertVersion(t *testing.T) {
	tests := []struct {
		data string
		want bool
	}{
		{`{"creation":"2026-10-20 10:00:00","updater_reference":{"doctype":"Data Import"},"created_by":"a@b.c"}`, true},
		{`{"changed":[],"added":[],"removed":[],"row_changed":[]}`, true},
		{`{"changed":[["x",1,2]]}`, false},
		{`{"row_changed":[["fields",0,"f1",[["label","A","B"]]]]}`, false},
	}
	for _, tt := range tests {
		if got := isInsertVersion(diff(t, tt.data)); got != tt.want {
			t.Errorf("isInsertVersion(%s) = %v, want %v", tt.data, got, tt.want)
		}
	}
}
