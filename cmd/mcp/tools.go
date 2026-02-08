package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// Tier 1 — Replace anti-patterns

var toolWegPy = mcp.NewTool("weg_py",
	mcp.WithDescription("Run Python code with frappe pre-connected to the site. "+
		"Use this instead of manually activating the bench environment or writing scripts that import frappe directly."),
	mcp.WithString("code",
		mcp.Required(),
		mcp.Description("Python code to execute. frappe is already imported and connected."),
	),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

var toolWegApiGet = mcp.NewTool("weg_api_get",
	mcp.WithDescription("Get Frappe documents via the REST-style API. "+
		"Use this instead of curl, mysql queries, or raw SQL selects."),
	mcp.WithString("doctype",
		mcp.Required(),
		mcp.Description("DocType to query, optionally with /name (e.g. 'User' or 'User/Administrator')"),
	),
	mcp.WithString("filters",
		mcp.Description("JSON filters object (e.g. '{\"enabled\": 1}')"),
	),
	mcp.WithString("fields",
		mcp.Description("JSON array of fields to return (e.g. '[\"name\",\"email\"]')"),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of records to return"),
	),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

var toolWegApiCall = mcp.NewTool("weg_api_call",
	mcp.WithDescription("Call a whitelisted Frappe method. "+
		"Use this instead of curl to localhost or direct Python imports."),
	mcp.WithString("method",
		mcp.Required(),
		mcp.Description("Dotted method path (e.g. 'frappe.ping' or 'frappe.client.get_list')"),
	),
	mcp.WithString("args",
		mcp.Description("Method arguments as JSON object or key=value pairs passed as extra CLI args"),
	),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

var toolWegExec = mcp.NewTool("weg_exec",
	mcp.WithDescription("Run a bench subcommand in the project context. "+
		"Use this for any bench/frappe CLI command not covered by other tools."),
	mcp.WithString("command",
		mcp.Required(),
		mcp.Description("The full command to pass to bench (e.g. 'frappe --site mysite console')"),
	),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

// Tier 2 — Common dev operations

var toolWegTest = mcp.NewTool("weg_test",
	mcp.WithDescription("Run Frappe tests for an app or module."),
	mcp.WithString("app",
		mcp.Description("App to test (default: current app)"),
	),
	mcp.WithString("module",
		mcp.Description("Specific module to test"),
	),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

var toolWegBuild = mcp.NewTool("weg_build",
	mcp.WithDescription("Build frontend assets for Frappe apps."),
	mcp.WithString("app",
		mcp.Description("Specific app to build (default: all)"),
	),
	mcp.WithBoolean("production",
		mcp.Description("Production build with minification"),
	),
)

var toolWegMigrate = mcp.NewTool("weg_migrate",
	mcp.WithDescription("Run database migrations. Applies schema changes and runs patches for all installed apps."),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

var toolWegCacheClear = mcp.NewTool("weg_cache_clear",
	mcp.WithDescription("Clear all caches for a site."),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

// Tier 3 — Introspection (context for AI)

var toolWegStatus = mcp.NewTool("weg_status",
	mcp.WithDescription("Get project status: installed apps, sites, frappe version, and environment info. "+
		"Use this to understand the current project before making changes."),
)

var toolWegDoctypeShow = mcp.NewTool("weg_doctype_show",
	mcp.WithDescription("Show DocType field definitions including fieldnames, types, and options. "+
		"Use this to understand a DocType's schema before writing code that interacts with it."),
	mcp.WithString("doctype",
		mcp.Required(),
		mcp.Description("Name of the DocType to inspect"),
	),
	mcp.WithString("site",
		mcp.Description("Target site (default: auto-detect)"),
	),
)

var toolWegSiteList = mcp.NewTool("weg_site_list",
	mcp.WithDescription("List all sites in the project with their status."),
)

var toolWegAppList = mcp.NewTool("weg_app_list",
	mcp.WithDescription("List all installed apps in the project."),
)
