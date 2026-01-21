/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var (
	syncMessage string
	syncDryRun  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Bidirectional sync with the remote site",
	Long: `Sync local changes with the remote Frappe site.

This command:
  1. Pulls remote changes first (to avoid conflicts)
  2. Commits local changes with the provided message
  3. Pushes local changes to the remote site

This is the recommended workflow for making changes:
  1. Edit files locally
  2. Run 'weg sync -m "description"'

Examples:
  weg sync -m "Add priority field to Todo"
  weg sync --dry-run      # Preview changes`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().StringVarP(&syncMessage, "message", "m", "", "Commit message for local changes")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Preview changes without syncing")
}

func runSync(cobraCmd *cobra.Command, args []string) error {
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

	// Check for local changes
	gitStatus := exec.Command("git", "status", "--porcelain")
	statusOutput, _ := gitStatus.Output()
	hasLocalChanges := len(strings.TrimSpace(string(statusOutput))) > 0

	if hasLocalChanges && syncMessage == "" && !syncDryRun {
		return fmt.Errorf("local changes detected; provide a commit message with -m \"message\"")
	}

	if syncDryRun {
		fmt.Println("Dry run mode - no changes will be made")
		fmt.Println()
	}

	// Connect
	output.Infof("Connecting to %s...\n", config.Site.URL)
	client := remote.NewClientFromConfig(config, creds)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Println("Connected")

	// Step 1: Commit local changes FIRST (before pull overwrites them)
	if hasLocalChanges {
		fmt.Println("\n[1/3] Committing local changes...")

		if !syncDryRun {
			// Stage all changes
			gitAdd := exec.Command("git", "add", "-A")
			if err := gitAdd.Run(); err != nil {
				return fmt.Errorf("failed to stage changes: %w", err)
			}

			// Commit
			commitMsg := syncMessage
			if commitMsg == "" {
				commitMsg = fmt.Sprintf("Sync with %s at %s",
					config.Site.URL, time.Now().Format("2006-01-02 15:04"))
			}
			gitCommit := exec.Command("git", "commit", "-m", commitMsg)
			if err := gitCommit.Run(); err != nil {
				// Might fail if nothing to commit, that's ok
			} else {
				fmt.Printf("  Committed: %s\n", commitMsg)
			}
		} else {
			fmt.Printf("  Would commit: %s\n", syncMessage)
		}
	} else {
		fmt.Println("\n[1/3] No local changes to commit")
	}

	// Step 2: Push local changes to remote
	fmt.Println("\n[2/3] Pushing to remote...")

	var pushFailed int
	if syncDryRun {
		entities, _ := findLocalEntities(".")
		fmt.Printf("  Would push %d entities\n", len(entities))
	} else {
		entities, err := findLocalEntities(".")
		if err != nil {
			return fmt.Errorf("failed to find entities: %w", err)
		}

		pushed := 0
		for _, e := range entities {
			if err := pushEntity(client, e); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed: %s - %v\n", e.name, err)
				pushFailed++
			} else {
				pushed++
			}
		}
		fmt.Printf("  Pushed: %d, Failed: %d\n", pushed, pushFailed)
	}

	// Step 3: Pull remote changes (to get any other changes from remote)
	fmt.Println("\n[3/3] Pulling remote changes...")
	fetcher := remote.NewFetcher(client, config)
	result, err := fetcher.FetchAll()
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if !syncDryRun {
		for _, entity := range result.Entities {
			if err := remote.WriteEntity(".", entity); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to write %s: %v\n", entity.Name, err)
			}
		}

		// Commit pulled changes if any
		gitStatus = exec.Command("git", "status", "--porcelain")
		statusOutput, _ = gitStatus.Output()
		if len(statusOutput) > 0 {
			gitAdd := exec.Command("git", "add", "-A")
			gitAdd.Run()
			gitCommit := exec.Command("git", "commit", "-m", fmt.Sprintf("Pull from %s", config.Site.URL))
			gitCommit.Run()
		}

		// Update sync timestamp
		config.Sync.LastSync = time.Now()
		if err := config.Save("."); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	fmt.Printf("  Fetched %d entities\n", len(result.Entities))

	fmt.Println()

	if pushFailed > 0 {
		fmt.Printf("Sync completed with %d failures\n", pushFailed)
		return fmt.Errorf("%d entities failed to sync", pushFailed)
	}

	fmt.Println("Sync complete")
	return nil
}
