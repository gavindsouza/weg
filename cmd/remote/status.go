/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var statusRemote bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status between local and remote",
	Long: `Show the current sync status between local files and the remote site.

Displays:
  - Local changes not yet pushed
  - Remote changes not yet pulled (with --remote flag)
  - Last sync timestamp

Examples:
  weg status             # Show local changes
  weg status --remote    # Also check for remote changes`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusRemote, "remote", false, "Check for remote changes (requires network)")
}

func runStatus(cobraCmd *cobra.Command, args []string) error {
	// Check if we're in a remote site directory
	if !remote.IsRemoteSite(".") {
		return fmt.Errorf("not a remote site clone (no .weg/site.toml found)")
	}

	// Load config
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Remote site: %s\n", config.Site.URL)
	fmt.Printf("Last sync:   %s\n", config.Sync.LastSync.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Check git status for local changes
	gitStatus := exec.Command("git", "status", "--porcelain")
	output, err := gitStatus.Output()
	if err != nil {
		return fmt.Errorf("failed to get git status: %w", err)
	}

	localChanges := strings.TrimSpace(string(output))
	if localChanges == "" {
		fmt.Println("Local:  No changes")
	} else {
		lines := strings.Split(localChanges, "\n")
		fmt.Printf("Local:  %d file(s) changed\n", len(lines))
		for _, line := range lines {
			if len(line) > 3 {
				status := line[:2]
				file := line[3:]
				switch {
				case strings.HasPrefix(status, "M"):
					fmt.Printf("  modified:   %s\n", file)
				case strings.HasPrefix(status, "A"):
					fmt.Printf("  added:      %s\n", file)
				case strings.HasPrefix(status, "D"):
					fmt.Printf("  deleted:    %s\n", file)
				case strings.HasPrefix(status, "?"):
					fmt.Printf("  untracked:  %s\n", file)
				default:
					fmt.Printf("  %s %s\n", status, file)
				}
			}
		}
	}

	// Check for unpushed commits (only if upstream is configured)
	gitLog := exec.Command("git", "log", "--oneline", "@{u}..", "--")
	unpushed, err := gitLog.Output()
	if err == nil && len(strings.TrimSpace(string(unpushed))) > 0 {
		lines := strings.Split(strings.TrimSpace(string(unpushed)), "\n")
		fmt.Printf("\nUnpushed commits: %d\n", len(lines))
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
	}

	// Check remote if requested
	if statusRemote {
		fmt.Println()
		fmt.Println("Checking remote...")

		creds, err := remote.LoadCredentials(".")
		if err != nil {
			return fmt.Errorf("failed to load credentials: %w", err)
		}

		client := remote.NewClientFromConfig(config, creds)
		if err := client.Ping(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		// TODO: Implement proper remote change detection
		// For now, just confirm connectivity
		fmt.Println("Remote: Connected (detailed diff not yet implemented)")
	}

	// Show sync instructions
	if localChanges != "" {
		fmt.Println()
		fmt.Println("To sync changes:")
		fmt.Println("  weg sync -m \"description\"")
	}

	return nil
}
