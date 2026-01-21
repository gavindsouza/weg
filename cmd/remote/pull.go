/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"fmt"
	"os"
	"os/exec"
	"time"

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
  weg pull                # Pull all changes
  weg pull --dry-run      # Preview changes without applying`,
	RunE: runPull,
}

var pullDryRun bool

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Preview changes without applying")
}

func runPull(cobraCmd *cobra.Command, args []string) error {
	// Check if we're in a remote site directory
	if !remote.IsRemoteSite(".") {
		return fmt.Errorf("not a remote site clone (no .weg/site.toml found)")
	}

	// Load config and credentials
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	creds, err := remote.LoadCredentials(".")
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	// Connect
	fmt.Printf("Connecting to %s...\n", config.Site.URL)
	client := remote.NewClientFromConfig(config, creds)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Println("Connected")

	// Fetch entities
	fmt.Println("Fetching customizations...")
	fetcher := remote.NewFetcher(client, config)
	result, err := fetcher.FetchAll()
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if pullDryRun {
		fmt.Printf("\nDry run - would update %d entities:\n", len(result.Entities))
		for _, e := range result.Entities {
			fmt.Printf("  %s: %s\n", e.Type, e.Name)
		}
		return nil
	}

	// Write entities
	fmt.Printf("Updating %d entities...\n", len(result.Entities))
	for _, entity := range result.Entities {
		if err := remote.WriteEntity(".", entity); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to write %s: %v\n", entity.Name, err)
		}
	}

	// Update config
	config.Sync.LastSync = time.Now()
	if err := config.Save("."); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Check for changes and commit
	gitStatus := exec.Command("git", "status", "--porcelain")
	output, _ := gitStatus.Output()
	if len(output) > 0 {
		gitAdd := exec.Command("git", "add", "-A")
		gitAdd.Run()

		commitMsg := fmt.Sprintf("Pull from %s at %s",
			config.Site.URL, time.Now().Format("2006-01-02 15:04"))
		gitCommit := exec.Command("git", "commit", "-m", commitMsg)
		gitCommit.Run()

		fmt.Println("Changes committed")
	} else {
		fmt.Println("Already up to date")
	}

	return nil
}
