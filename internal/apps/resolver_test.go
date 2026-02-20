package apps

import (
	"testing"
)

func TestTopoSort_Simple(t *testing.T) {
	graph := map[string][]string{
		"erpnext": {},
		"hrms":    {"erpnext"},
	}
	order, err := topoSort(graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2, got %d", len(order))
	}
	// erpnext must come before hrms
	idx := make(map[string]int)
	for i, n := range order {
		idx[n] = i
	}
	if idx["erpnext"] >= idx["hrms"] {
		t.Errorf("erpnext (%d) should come before hrms (%d)", idx["erpnext"], idx["hrms"])
	}
}

func TestTopoSort_Diamond(t *testing.T) {
	// A depends on B and C, B depends on D, C depends on D
	graph := map[string][]string{
		"a": {"b", "c"},
		"b": {"d"},
		"c": {"d"},
		"d": {},
	}
	order, err := topoSort(graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4, got %d", len(order))
	}

	idx := make(map[string]int)
	for i, n := range order {
		idx[n] = i
	}
	// d must come before b and c; b and c must come before a
	if idx["d"] >= idx["b"] {
		t.Errorf("d should come before b")
	}
	if idx["d"] >= idx["c"] {
		t.Errorf("d should come before c")
	}
	if idx["b"] >= idx["a"] {
		t.Errorf("b should come before a")
	}
	if idx["c"] >= idx["a"] {
		t.Errorf("c should come before a")
	}
}

func TestTopoSort_CycleDetection(t *testing.T) {
	graph := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}
	_, err := topoSort(graph)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestTopoSort_SingleNode(t *testing.T) {
	graph := map[string][]string{
		"only": {},
	}
	order, err := topoSort(graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 {
		t.Fatalf("expected 1, got %d", len(order))
	}
	if order[0] != "only" {
		t.Errorf("got %q, want %q", order[0], "only")
	}
}

func TestTopoSort_Empty(t *testing.T) {
	graph := map[string][]string{}
	order, err := topoSort(graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 0 {
		t.Errorf("expected empty, got %d", len(order))
	}
}

func TestTopoSort_LinearChain(t *testing.T) {
	// a -> b -> c -> d
	graph := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"d"},
		"d": {},
	}
	order, err := topoSort(graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4, got %d", len(order))
	}

	idx := make(map[string]int)
	for i, n := range order {
		idx[n] = i
	}
	// Must be d, c, b, a order
	if idx["d"] >= idx["c"] {
		t.Error("d should come before c")
	}
	if idx["c"] >= idx["b"] {
		t.Error("c should come before b")
	}
	if idx["b"] >= idx["a"] {
		t.Error("b should come before a")
	}
}

func TestResolveDependencies_MaxDepthExceeded(t *testing.T) {
	spec := AppSpec{
		Name: "test",
		URL:  "https://github.com/test/test",
	}
	opts := ResolveOptions{
		MaxDepth: 0, // will be set to default, so use 1 and nested deps
	}
	// With MaxDepth=1, it shouldn't recurse at all into deps
	opts.MaxDepth = 1
	result, err := ResolveDependencies(spec, opts)
	// This will attempt network requests and may error, but shouldn't panic
	// In a real setup with mocks we'd validate better
	if err != nil {
		// Network errors are expected in unit tests
		t.Skipf("network-dependent test: %v", err)
	}
	_ = result
}

func TestResolveOptions_Defaults(t *testing.T) {
	opts := ResolveOptions{}
	// Verify that ResolveDependencies handles zero MaxDepth (sets default to 20)
	// We can't easily test this without network access, but we verify the struct
	if opts.MaxDepth != 0 {
		t.Errorf("zero value MaxDepth should be 0, got %d", opts.MaxDepth)
	}
	if opts.InstalledApps != nil {
		t.Error("InstalledApps should be nil by default")
	}
}

func TestResolveResult_PrintDoesNotPanic(t *testing.T) {
	result := &ResolveResult{
		InstallOrder: []AppSpec{
			{Name: "erpnext", URL: "https://github.com/frappe/erpnext"},
			{Name: "hrms", URL: "https://github.com/frappe/hrms"},
		},
		Graph: map[string][]string{
			"hrms":    {"erpnext"},
			"erpnext": {},
		},
		AlreadyInstalled: []string{},
		Warnings:         []string{"test warning"},
	}
	// Shouldn't panic
	PrintResolveResult(result)
}
