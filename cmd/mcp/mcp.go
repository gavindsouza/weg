package mcp

import (
	"github.com/spf13/cobra"
)

// McpCmd is the parent command for MCP server operations
var McpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server for AI assistant integration",
	Long: `Model Context Protocol (MCP) server that exposes weg commands as
structured tools for AI assistants like Claude Code.

Subcommands:
  serve     Start the MCP server on stdio
  install   Generate/update .mcp.json configuration

Quick start:
  weg mcp install        # Add weg to .mcp.json
  weg mcp serve          # Start server (used by AI tools)`,
}

func init() {
	McpCmd.AddCommand(serveCmd)
	McpCmd.AddCommand(installCmd)
}
