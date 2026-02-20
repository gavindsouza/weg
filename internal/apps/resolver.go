package apps

import (
	"fmt"
	"strings"
)

// ResolveResult is the output of dependency resolution
type ResolveResult struct {
	// InstallOrder is the topologically sorted list of apps to install.
	// Dependencies come before dependents — install in this order.
	InstallOrder []AppSpec

	// Graph is the full dependency adjacency map: app -> [dependencies]
	Graph map[string][]string

	// Sources records where each app's dependency info was read from
	Sources map[string]DependencySource

	// Warnings collects non-fatal issues during resolution
	Warnings []string

	// AlreadyInstalled lists apps that were skipped because they're already present
	AlreadyInstalled []string
}

// ResolveOptions configures the dependency resolver
type ResolveOptions struct {
	// BenchPath is the bench root (used to check which apps are already installed)
	BenchPath string

	// AppsDir is the path to the apps/ directory
	AppsDir string

	// AllowRemote enables fetching dependency info from GitHub without cloning
	AllowRemote bool

	// InstalledApps is a set of app names already installed (skipped during resolution).
	// If nil, the resolver reads from AppsDir.
	InstalledApps map[string]bool

	// MaxDepth limits recursion depth to prevent runaway resolution (0 = default 20)
	MaxDepth int

	// Verbose enables progress logging
	Verbose bool

	// LogFunc is called with progress messages when Verbose is true
	LogFunc func(format string, args ...any)
}

func (o *ResolveOptions) log(format string, args ...any) {
	if o.Verbose && o.LogFunc != nil {
		o.LogFunc(format, args...)
	}
}

func (o *ResolveOptions) maxDepth() int {
	if o.MaxDepth > 0 {
		return o.MaxDepth
	}
	return 20
}

// ResolveDependencies performs full transitive dependency resolution for an app.
//
// Given a root app (specified by AppSpec), it:
//  1. Reads the app's dependencies (from local clone or remote GitHub)
//  2. Recursively resolves each dependency's dependencies
//  3. Builds a dependency graph
//  4. Performs topological sort (dependencies before dependents)
//  5. Detects circular dependencies
//  6. Filters out already-installed apps
//
// The returned InstallOrder includes the root app itself as the last element.
func ResolveDependencies(root AppSpec, opts ResolveOptions) (*ResolveResult, error) {
	result := &ResolveResult{
		Graph:   make(map[string][]string),
		Sources: make(map[string]DependencySource),
	}

	// Build the installed set
	installed := opts.InstalledApps
	if installed == nil {
		installed = make(map[string]bool)
		if opts.AppsDir != "" {
			dirs, _ := ReadInstalledAppDirs(opts.AppsDir)
			for _, d := range dirs {
				installed[d] = true
			}
		}
	}

	// Track all discovered app specs by name
	specMap := map[string]AppSpec{
		root.Name: root,
	}

	// BFS/DFS to resolve the full graph
	visited := make(map[string]bool)
	inStack := make(map[string]bool) // for cycle detection during DFS

	var resolve func(name string, depth int) error
	resolve = func(name string, depth int) error {
		if depth > opts.maxDepth() {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("max depth %d reached resolving %s — possible deep dependency chain", opts.maxDepth(), name))
			return nil
		}

		if visited[name] {
			return nil
		}
		visited[name] = true
		inStack[name] = true
		defer func() { inStack[name] = false }()

		spec, hasSpec := specMap[name]

		// Read dependencies
		var deps []AppDependency
		var source DependencySource

		// 1. Try local clone
		if opts.AppsDir != "" {
			localPath := fmt.Sprintf("%s/%s", opts.AppsDir, name)
			if IsGitRepo(localPath) {
				opts.log("  Reading dependencies for %s (local)...", name)
				deps, source, _ = ReadDependencies(localPath)
			}
		}

		// 2. Try remote fetch
		if len(deps) == 0 && opts.AllowRemote && hasSpec && spec.URL != "" {
			opts.log("  Fetching dependencies for %s (remote)...", name)
			deps, source, _ = ReadDependenciesRemote(spec.URL, spec.Branch)
		}

		result.Sources[name] = source
		result.Graph[name] = nil

		for _, dep := range deps {
			// Record in graph
			result.Graph[name] = append(result.Graph[name], dep.Name)

			// Track spec (don't overwrite if already known with more info)
			if _, exists := specMap[dep.Name]; !exists {
				specMap[dep.Name] = AppSpec{
					Name:   dep.Name,
					URL:    dep.URL,
					Branch: dep.Branch,
				}
			}

			// Check for cycle
			if inStack[dep.Name] {
				cycle := buildCyclePath(name, dep.Name, result.Graph)
				return fmt.Errorf("circular dependency detected: %s", cycle)
			}

			// Recurse
			if err := resolve(dep.Name, depth+1); err != nil {
				return err
			}
		}

		return nil
	}

	opts.log("Resolving dependencies for %s...", root.Name)
	if err := resolve(root.Name, 0); err != nil {
		return result, err
	}

	// Topological sort
	order, err := topoSort(result.Graph)
	if err != nil {
		return result, fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Build the install order, filtering installed apps
	for _, name := range order {
		if name == "frappe" {
			// frappe is managed separately
			continue
		}
		if installed[name] {
			result.AlreadyInstalled = append(result.AlreadyInstalled, name)
			opts.log("  %s: already installed, skipping", name)
			continue
		}
		spec, ok := specMap[name]
		if !ok {
			spec = AppSpec{Name: name}
		}
		result.InstallOrder = append(result.InstallOrder, spec)
	}

	return result, nil
}

// topoSort performs Kahn's algorithm for topological sorting.
// Returns nodes in dependency-first order (leaves first, root last).
func topoSort(graph map[string][]string) ([]string, error) {
	// Build in-degree map
	inDegree := make(map[string]int)
	for node := range graph {
		if _, ok := inDegree[node]; !ok {
			inDegree[node] = 0
		}
		for _, dep := range graph[node] {
			inDegree[dep]++ // dep is depended upon by node
			if _, ok := graph[dep]; !ok {
				// Leaf node not in graph — add it
				graph[dep] = nil
			}
		}
	}

	// Seed queue with zero in-degree nodes (leaves / no dependents needing them first)
	// Wait — we want dependency-first order. In our graph, edges go from
	// app -> its dependencies. So "erpnext" -> ["frappe"] means erpnext depends on frappe.
	//
	// For install order, we want frappe (leaf) first, erpnext last.
	// In Kahn's algorithm on this graph direction, nodes with zero in-degree are
	// the ones nobody depends on (i.e., the root apps). We want the reverse:
	// nodes with zero out-degree (no dependencies) should come first.
	//
	// So we reverse the edges: dep -> [dependents] and run standard Kahn's.
	reversed := make(map[string][]string)
	revInDegree := make(map[string]int)

	for node := range graph {
		if _, ok := revInDegree[node]; !ok {
			revInDegree[node] = 0
		}
	}

	for node, deps := range graph {
		revInDegree[node] += 0 // ensure it exists
		for _, dep := range deps {
			reversed[dep] = append(reversed[dep], node)
			revInDegree[node]++
		}
	}

	var queue []string
	for node, deg := range revInDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}

	var order []string
	for len(queue) > 0 {
		// Pop
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, dependent := range reversed[node] {
			revInDegree[dependent]--
			if revInDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(revInDegree) {
		// Cycle exists
		var remaining []string
		for node, deg := range revInDegree {
			if deg > 0 {
				remaining = append(remaining, node)
			}
		}
		return nil, fmt.Errorf("circular dependency among: %s", strings.Join(remaining, ", "))
	}

	return order, nil
}

// buildCyclePath constructs a human-readable cycle path for error messages
func buildCyclePath(from, to string, graph map[string][]string) string {
	// Simple: just show the direct edge that closes the cycle
	return fmt.Sprintf("%s -> ... -> %s -> %s", to, from, to)
}

// PrintResolveResult formats the resolution result for display
func PrintResolveResult(result *ResolveResult) string {
	var sb strings.Builder

	if len(result.InstallOrder) == 0 {
		sb.WriteString("No new apps to install.\n")
		if len(result.AlreadyInstalled) > 0 {
			sb.WriteString(fmt.Sprintf("Already installed: %s\n", strings.Join(result.AlreadyInstalled, ", ")))
		}
		return sb.String()
	}

	sb.WriteString("Dependency resolution complete:\n\n")

	// Show the graph
	sb.WriteString("  Dependency graph:\n")
	for app, deps := range result.Graph {
		if len(deps) == 0 {
			sb.WriteString(fmt.Sprintf("    %s (no dependencies)\n", app))
		} else {
			sb.WriteString(fmt.Sprintf("    %s -> %s\n", app, strings.Join(deps, ", ")))
		}
	}

	sb.WriteString("\n  Install order:\n")
	for i, spec := range result.InstallOrder {
		source := ""
		if s, ok := result.Sources[spec.Name]; ok && s != SourceNone {
			source = fmt.Sprintf(" [from %s]", s)
		}
		sb.WriteString(fmt.Sprintf("    %d. %s%s\n", i+1, spec, source))
	}

	if len(result.AlreadyInstalled) > 0 {
		sb.WriteString(fmt.Sprintf("\n  Already installed: %s\n", strings.Join(result.AlreadyInstalled, ", ")))
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\n  Warnings:\n")
		for _, w := range result.Warnings {
			sb.WriteString(fmt.Sprintf("    - %s\n", w))
		}
	}

	return sb.String()
}
