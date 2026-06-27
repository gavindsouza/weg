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
