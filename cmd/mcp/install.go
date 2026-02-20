package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var installForce bool

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Generate/update .mcp.json with weg server config",
	Long: `Add the weg MCP server entry to the project's .mcp.json file.

This merges the weg entry into the existing configuration, preserving
any other MCP servers already configured.

Examples:
  weg mcp install            # Add weg to .mcp.json
  weg mcp install --force    # Overwrite existing weg entry`,
	RunE:         runInstall,
	SilenceUsage: true,
}

func init() {
	installCmd.Flags().BoolVarP(&installForce, "force", "f", false, "Overwrite existing weg entry")
}

func runInstall(cmd *cobra.Command, args []string) error {
	mcpPath := filepath.Join(".", ".mcp.json")

	// Load existing config or start fresh
	config := make(map[string]any)
	if data, err := os.ReadFile(mcpPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing .mcp.json: %w", err)
		}
	}

	// Ensure mcpServers key exists
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}

	// Check if weg entry already exists
	if _, exists := servers["weg"]; exists && !installForce {
		fmt.Println("weg entry already exists in .mcp.json (use --force to overwrite)")
		return nil
	}

	// Add weg entry
	servers["weg"] = map[string]any{
		"command": "weg",
		"args":    []string{"mcp", "serve"},
	}
	config["mcpServers"] = servers

	// Write back
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize .mcp.json: %w", err)
	}

	if err := os.WriteFile(mcpPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write .mcp.json: %w", err)
	}

	fmt.Printf("Updated %s with weg MCP server\n", mcpPath)
	return nil
}
