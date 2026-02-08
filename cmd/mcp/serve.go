package mcp

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// Version is set by the parent package to avoid import cycles.
var Version = "dev"

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server on stdio",
	Long: `Start the Model Context Protocol server that communicates over stdin/stdout.

This is typically invoked by AI tools (e.g. Claude Code) via .mcp.json configuration,
not run manually. Use 'weg mcp install' to set up the configuration.`,
	RunE:         runServe,
	SilenceUsage: true,
}

func runServe(c *cobra.Command, args []string) error {
	s := server.NewMCPServer(
		"weg",
		Version,
		server.WithToolCapabilities(false),
		server.WithInstructions(serverInstructions),
	)

	registerAllTools(s)

	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}
	return nil
}

const serverInstructions = `weg is a modern CLI for managing Frappe development environments.

NEVER manually activate the bench environment, write scripts that import frappe directly, or use bench commands.
NEVER modify files inside .weg/ — it is managed infrastructure.

Use the provided tools instead of shell commands for all Frappe operations.`

func registerAllTools(s *server.MCPServer) {
	// Tier 1 — Replace anti-patterns
	s.AddTool(toolWegPy, handleWegPy)
	s.AddTool(toolWegApiGet, handleWegApiGet)
	s.AddTool(toolWegApiCall, handleWegApiCall)
	s.AddTool(toolWegExec, handleWegExec)

	// Tier 2 — Common dev operations
	s.AddTool(toolWegTest, handleWegTest)
	s.AddTool(toolWegBuild, handleWegBuild)
	s.AddTool(toolWegMigrate, handleWegMigrate)
	s.AddTool(toolWegCacheClear, handleWegCacheClear)

	// Tier 3 — Introspection
	s.AddTool(toolWegStatus, handleWegStatus)
	s.AddTool(toolWegDoctypeShow, handleWegDoctypeShow)
	s.AddTool(toolWegSiteList, handleWegSiteList)
	s.AddTool(toolWegAppList, handleWegAppList)
}
