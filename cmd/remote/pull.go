/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"fmt"
	"os/exec"
	"time"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull changes from the remote site",
	Long: `Fetch changes from the remote site and update local files.

This command:
  1. Connects to the remote site
  2. Fetches all enabled entity types
  3. Updates local files with remote changes
  4. Creates a git commit for the changes

Examples:
  weg pull                       # Pull all changes
  weg pull -m "Update scripts"   # Pull with custom commit message
  weg pull --dry-run             # Preview changes without applying`,
	RunE: runPull,
}

var (
	pullDryRun  bool
	pullMessage string
)

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Preview changes without applying")
	pullCmd.Flags().StringVarP(&pullMessage, "message", "m", "", "Commit message for pulled changes")
}

func runPull(cobraCmd *cobra.Command, args []string) error {
	// Check if we're in a remote site directory
	if !remote.IsRemoteSite(".") {
		return wegerrors.NotFound("remote clone", ".weg/site.toml")
	}

	// Load config and credentials
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return wegerrors.Config("site.toml", "read", err)
	}

	creds, err := remote.LoadCredentials(".")
	if err != nil {
		return wegerrors.Config("credentials", "read", err)
	}

	// Connect
	output.Infof("Connecting to %s...\n", config.Site.URL)
	client := remote.NewClientFromConfig(config, creds)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	output.Print("Connected")

	// Fetch entities
	output.Print("Fetching customizations...")
	fetcher := remote.NewFetcher(client, config)
	result, err := fetcher.FetchAll()
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if pullDryRun {
		output.Printf("\nDry run - would update %d entities:", len(result.Entities))
		for _, e := range result.Entities {
			output.Printf("  %s: %s", e.Type, e.Name)
		}
		return nil
	}

	// Write entities
	output.Infof("Updating %d entities...\n", len(result.Entities))
	for _, entity := range result.Entities {
		if err := remote.WriteEntity(".", entity); err != nil {
			output.Errorf("Failed to write %s: %v", entity.Name, err)
		}
	}

	// Update config
	config.Sync.LastSync = time.Now()
	if err := config.Save("."); err != nil {
		return wegerrors.Config("site.toml", "write", err)
	}

	// Check for changes and commit
	gitStatus := exec.Command("git", "status", "--porcelain")
	statusOutput, _ := gitStatus.Output()
	if len(statusOutput) > 0 {
		gitAdd := exec.Command("git", "add", "-A")
		gitAdd.Run()

		commitMsg := pullMessage
		if commitMsg == "" {
			commitMsg = fmt.Sprintf("Pull from %s at %s",
				config.Site.URL, time.Now().Format("2006-01-02 15:04"))
		}
		gitCommit := exec.Command("git", "commit", "-m", commitMsg)
		gitCommit.Run()

		output.Print("Changes committed")
	} else {
		output.Print("Already up to date")
	}

	return nil
}
