/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
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

By default, pull reconstructs the site's version history since the last sync:
it fetches the Version records newer than the recorded last_sync and replays
them as individual, authored git commits (the same style as clone), so
'git log' and 'git blame' stay meaningful. Any current-state changes that have
no Version record land in a final snapshot commit, so nothing is dropped.

Pass --no-history for the fast path: a single snapshot commit of the current
state (the pre-history behavior).

This command:
  1. Connects to the remote site
  2. Fetches all enabled entity types
  3. Replays version history since last_sync as per-document commits
     (or a single snapshot commit with --no-history)
  4. Updates last_sync

Examples:
  weg remote pull                    # Pull, reconstructing history since last sync
  weg remote pull --no-history       # Pull as a single snapshot commit
  weg remote pull -m "Update scripts" # Snapshot pull with a custom message (implies --no-history)
  weg remote pull --dry-run          # Preview changes without applying`,
	RunE: runPull,
}

var (
	pullDryRun    bool
	pullMessage   string
	pullNoHistory bool
)

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Preview changes without applying")
	pullCmd.Flags().StringVarP(&pullMessage, "message", "m", "", "Commit message for a snapshot pull (implies --no-history)")
	pullCmd.Flags().BoolVar(&pullNoHistory, "no-history", false, "Skip version history (single snapshot commit)")
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

	// A custom commit message only makes sense for a single snapshot commit.
	snapshotOnly := pullNoHistory || pullMessage != ""
	if snapshotOnly {
		return pullSnapshot(config, result)
	}
	return pullWithHistory(cobraCmd.Context(), config, fetcher, result)
}

// pullSnapshot writes the current entity state and records it as a single git
// commit (the --no-history path, and the pre-history default behavior).
func pullSnapshot(config *remote.SiteConfig, result *remote.FetchResult) error {
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
	if len(statusOutput) == 0 {
		output.Print("Already up to date")
		return nil
	}

	gitAdd := exec.Command("git", "add", "-A")
	if err := gitAdd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	commitMsg := pullMessage
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Pull from %s at %s",
			config.Site.URL, time.Now().Format("2006-01-02 15:04"))
	}
	gitCommit := exec.Command("git", "commit", "-m", commitMsg)
	if out, err := gitCommit.CombinedOutput(); err != nil {
		return wegerrors.Operation("git commit", strings.TrimSpace(string(out)), err)
	}

	output.Print("Changes committed")
	return nil
}

// pullWithHistory replays Version records created since last_sync as individual
// authored commits, then reconciles the current state into a final snapshot
// commit. last_sync is advanced only after the pipeline runs, so a failed pull
// re-fetches the same window on the next run.
func pullWithHistory(ctx context.Context, config *remote.SiteConfig, fetcher *remote.Fetcher, result *remote.FetchResult) error {
	since := config.Sync.LastSync

	// Advance last_sync before reconstruction so the updated site.toml is swept
	// into the trailing reconcile commit (mirrors clone's ordering).
	config.Sync.LastSync = time.Now()
	if err := config.Save("."); err != nil {
		return wegerrors.Config("site.toml", "write", err)
	}

	return reconstructHistory(ctx, ".", config.Site.URL, config.Site.Frappe.Version, "modules.txt", fetcher, result, since)
}
