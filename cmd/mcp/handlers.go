package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// runWegCommand runs a weg subcommand and returns its combined output.
func runWegCommand(args ...string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to find weg binary: %w", err)
	}

	cmd := exec.Command(exe, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Include output in error for context
		if len(out) > 0 {
			return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
		}
		return "", err
	}
	return string(out), nil
}

// siteArgs returns ["--site", site] if site is non-empty, else nil.
func siteArgs(site string) []string {
	if site != "" {
		return []string{"--site", site}
	}
	return nil
}

// --- Tier 1: Subprocess handlers ---

func handleWegPy(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	site := request.GetString("site", "")

	args := []string{"py"}
	args = append(args, siteArgs(site)...)
	args = append(args, code)

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

func handleWegApiGet(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	doctype, err := request.RequireString("doctype")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	site := request.GetString("site", "")
	filters := request.GetString("filters", "")
	fields := request.GetString("fields", "")

	args := []string{"api", "get", doctype, "--raw"}
	args = append(args, siteArgs(site)...)
	if filters != "" {
		args = append(args, "--filters", filters)
	}
	if fields != "" {
		args = append(args, "--fields", fields)
	}

	// Handle limit
	if limitVal, ok := request.GetArguments()["limit"]; ok {
		if limitNum, ok := limitVal.(float64); ok {
			args = append(args, "--limit", fmt.Sprintf("%d", int(limitNum)))
		}
	}

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

func handleWegApiCall(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	method, err := request.RequireString("method")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	site := request.GetString("site", "")
	extraArgs := request.GetString("args", "")

	args := []string{"api", "call", method, "--raw"}
	args = append(args, siteArgs(site)...)
	if extraArgs != "" {
		// Pass extra args as additional CLI arguments
		args = append(args, strings.Fields(extraArgs)...)
	}

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

func handleWegExec(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	command, err := request.RequireString("command")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	site := request.GetString("site", "")

	args := []string{"exec"}
	args = append(args, siteArgs(site)...)
	args = append(args, strings.Fields(command)...)

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

// --- Tier 2: Common dev operations ---

func handleWegTest(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	app := request.GetString("app", "")
	module := request.GetString("module", "")
	site := request.GetString("site", "")

	args := []string{"test"}
	args = append(args, siteArgs(site)...)
	if app != "" {
		args = append(args, "--app", app)
	}
	if module != "" {
		args = append(args, "--module", module)
	}

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

func handleWegBuild(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	app := request.GetString("app", "")

	args := []string{"build"}
	if app != "" {
		args = append(args, app)
	}

	// Handle production boolean
	if prodVal, ok := request.GetArguments()["production"]; ok {
		if prod, ok := prodVal.(bool); ok && prod {
			args = append(args, "--production")
		}
	}

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

func handleWegMigrate(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	site := request.GetString("site", "")

	args := []string{"db", "migrate"}
	if site != "" {
		args = append(args, site)
	}

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

func handleWegCacheClear(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	site := request.GetString("site", "")

	args := []string{"cache", "clear"}
	args = append(args, siteArgs(site)...)

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

// --- Tier 3: In-process introspection handlers ---

func handleWegStatus(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	absPath, err := filepath.Abs(".")
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to get working directory: %v", err)), nil
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to detect context: %v", err)), nil
	}

	status := map[string]interface{}{
		"context":     result.Context.String(),
		"path":        result.Path,
		"description": result.ContextDescription(),
	}

	if result.AppName != "" {
		status["app_name"] = result.AppName
	}
	if result.BenchPath != "" {
		status["bench_path"] = result.BenchPath
	}

	// Load state if available
	st, err := state.Load(absPath)
	if err == nil && !st.IsEmpty() {
		status["frappe_version"] = st.Frappe.Version
		status["database"] = st.Frappe.Database
		status["last_sync"] = st.LastSync.String()

		apps := make([]string, 0, len(st.Apps))
		for name := range st.Apps {
			apps = append(apps, name)
		}
		status["apps"] = apps

		sites := make([]string, 0, len(st.Sites))
		for name := range st.Sites {
			sites = append(sites, name)
		}
		status["sites"] = sites
		status["default_site"] = st.GetDefaultSite()
	}

	data, _ := json.MarshalIndent(status, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

func handleWegDoctypeShow(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	doctype, err := request.RequireString("doctype")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	site := request.GetString("site", "")

	args := []string{"doctype", "show", doctype, "--json"}
	args = append(args, siteArgs(site)...)

	out, err := runWegCommand(args...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(out), nil
}

func handleWegSiteList(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	absPath, err := filepath.Abs(".")
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to get working directory: %v", err)), nil
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to detect context: %v", err)), nil
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	case config.ContextWegBench:
		benchPath = absPath
	default:
		return mcplib.NewToolResultError("not a weg-managed project"), nil
	}

	st, err := state.Load(absPath)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to load state: %v", err)), nil
	}

	type siteInfo struct {
		Name    string   `json:"name"`
		Apps    []string `json:"apps,omitempty"`
		Default bool     `json:"default,omitempty"`
	}

	sites := []siteInfo{}

	if len(st.Sites) > 0 {
		for name, s := range st.Sites {
			sites = append(sites, siteInfo{
				Name:    name,
				Apps:    s.Apps,
				Default: s.DefaultSite,
			})
		}
	} else {
		// Fallback: scan sites directory
		sitesDir := filepath.Join(benchPath, "sites")
		entries, _ := os.ReadDir(sitesDir)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "assets" {
				cfgPath := filepath.Join(sitesDir, e.Name(), "site_config.json")
				if _, err := os.Stat(cfgPath); err == nil {
					sites = append(sites, siteInfo{Name: e.Name()})
				}
			}
		}
	}

	data, _ := json.MarshalIndent(sites, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

func handleWegAppList(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	absPath, err := filepath.Abs(".")
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to get working directory: %v", err)), nil
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to detect context: %v", err)), nil
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	case config.ContextWegBench:
		benchPath = absPath
	default:
		return mcplib.NewToolResultError("not a weg-managed project"), nil
	}

	st, err := state.Load(absPath)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to load state: %v", err)), nil
	}

	type appInfo struct {
		Name   string `json:"name"`
		Branch string `json:"branch,omitempty"`
		Commit string `json:"commit,omitempty"`
		Path   string `json:"path,omitempty"`
	}

	apps := []appInfo{}

	if len(st.Apps) > 0 {
		for name, a := range st.Apps {
			apps = append(apps, appInfo{
				Name:   name,
				Branch: a.Branch,
				Commit: a.Commit,
				Path:   a.Path,
			})
		}
	} else {
		// Fallback: scan apps directory
		appsDir := filepath.Join(benchPath, "apps")
		entries, _ := os.ReadDir(appsDir)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				apps = append(apps, appInfo{Name: e.Name()})
			}
		}
	}

	data, _ := json.MarshalIndent(apps, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}
