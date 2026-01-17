/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"github.com/spf13/cobra"
)

// RemoteCmd represents the remote command group
var RemoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Work with remote Frappe sites",
	Long: `Work with remote Frappe sites without direct bench access.

This enables the "remote-site" development flow - the third major weg workflow
alongside app-centric and bench-centric development.

Commands:
  clone    Clone a remote site's customizations to a local git-backed directory
  pull     Fetch changes from the remote site
  push     Push local changes to the remote site
  sync     Bidirectional sync with optional commit message
  status   Show differences between local and remote

The remote workflow creates a git-backed directory that mirrors the site's
customization structure, enabling:
  - Local file editing with AI assistance
  - Version control via git
  - Team collaboration through git workflows
  - Easy migration to proper Frappe apps

Example workflow:
  weg clone https://mysite.frappe.cloud mysite
  cd mysite
  # Edit files locally...
  weg sync -m "Add priority field to Todo"`,
}

func init() {
	// Subcommands are added by their respective init() functions
	RemoteCmd.AddCommand(cloneCmd)
	RemoteCmd.AddCommand(pullCmd)
	RemoteCmd.AddCommand(pushCmd)
	RemoteCmd.AddCommand(syncCmd)
	RemoteCmd.AddCommand(statusCmd)
	RemoteCmd.AddCommand(infoCmd)
	RemoteCmd.AddCommand(loginCmd)
	RemoteCmd.AddCommand(logoutCmd)
}
