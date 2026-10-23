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
'git log' and 'git blame' stay meaningful. Each replayed commit holds the whole
document: replay starts from the committed file, and a document's last change
in the window is committed as the site has it now. Custom Field and Property
Setter changes update their row in the per-DocType file, and a document
created in the window is added in a commit by its creator. Any current-state
changes that have no Version record land in a final "chore(sync): reconcile"
commit, so nothing is dropped.

Frappe writes no Version on delete, so deletions are replayed from the site's
Deleted Document rows: each deletion of a document the export holds becomes a
commit by the user who deleted it, removing its file (or its row from the
per-DocType Custom Field / Property Setter file). The reconcile commit then
removes the committed files the site no longer exports at all: a deletion
without a trash row, the old path of a document that moved to another module,
a DocType that is no longer custom. Files of entity types disabled in
[sync.entities] and of modules marked sync = false are never removed. A
snapshot pull (--no-history) removes the same files.

Frappe records times as wall-clock time in the site's time zone (System
Settings, or Asia/Kolkata when unset), so commits carry that zone's UTC offset.
If System Settings cannot be read (the API user needs read access on it), pull
stops before replaying anything and exits non-zero, leaving last_sync as it
was: the version window cannot be placed without the zone.

If the version history cannot be fetched, pull still commits the current state
as a single snapshot so the working tree is usable, prints warnings tagged
"history-fallback:" and exits non-zero, so scheduled jobs notice that the
window was recorded without per-change history. If the replay fails or is
interrupted (Ctrl-C), pull restores the export to the commit it started from,
keeping no replayed commits and leaving last_sync as it was, so a rerun
replays the same window cleanly.

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

	// Fetch entities. The sync time is taken before the fetch, so a site
	// change made while it runs is picked up again by the next pull rather
	// than missed; Versions newer than it are left to the next pull too.
	output.Print("Fetching customizations...")
	syncedAt := time.Now()
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
		stale, err := planStaleExport(".", result, config)
		if err != nil {
			return err
		}
		if len(stale) > 0 {
			output.Printf("\nWould remove %d files the site no longer exports:", len(stale))
			for _, p := range stale {
				output.Printf("  - %s", p)
			}
		}
		return nil
	}

	// A custom commit message only makes sense for a single snapshot commit.
	snapshotOnly := pullNoHistory || pullMessage != ""
	if snapshotOnly {
		return pullSnapshot(config, result, syncedAt)
	}
	return pullWithHistory(cobraCmd.Context(), config, fetcher, result, syncedAt)
}

// pullSnapshot writes the current entity state and records it as a single git
// commit (the --no-history path, and the pre-history default behavior).
func pullSnapshot(config *remote.SiteConfig, result *remote.FetchResult, syncedAt time.Time) error {
	// Write entities
	output.Infof("Updating %d entities...\n", len(result.Entities))
	for _, entity := range result.Entities {
		if err := remote.WriteEntity(".", entity); err != nil {
			output.Errorf("Failed to write %s: %v", entity.Name, err)
		}
	}
	// Drop what the site no longer exports, so a snapshot converges too.
	if _, err := pruneStaleExport(".", result, config, nil); err != nil {
		return err
	}

	// Keep the workspace in step with the freshly-written JSONs
	refreshWorkspaceAfterPull(".")

	// Update config
	config.Sync.LastSync = syncedAt
	if err := config.Save("."); err != nil {
		return wegerrors.Config("site.toml", "write", err)
	}

	// Check for changes and commit. Only the export is staged: in a
	// subdirectory (weg -C), the rest of the repository is not the site's.
	gitStatus := exec.Command("git", "status", "--porcelain", "--", ".")
	statusOutput, _ := gitStatus.Output()
	if len(statusOutput) == 0 {
		output.Print("Already up to date")
		return nil
	}

	gitAdd := exec.Command("git", "add", "-A", "--", ".")
	if err := gitAdd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	commitMsg := pullMessage
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Pull from %s at %s",
			config.Site.URL, time.Now().Format("2006-01-02 15:04"))
	}
	gitCommit := exec.Command("git", "commit", "-m", commitMsg, "--", ".")
	if out, err := gitCommit.CombinedOutput(); err != nil {
		return wegerrors.Operation("git commit", strings.TrimSpace(string(out)), err)
	}

	output.Print("Changes committed")
	return nil
}

// pullWithHistory replays Version records created since last_sync as individual
// authored commits, starting each document from its committed file, then
// reconciles the current state into a final snapshot commit. last_sync is
// written only right before that commit (and lands in it), so a failed pull
// re-fetches the same window on the next run.
func pullWithHistory(ctx context.Context, config *remote.SiteConfig, fetcher *remote.Fetcher, result *remote.FetchResult, syncedAt time.Time) error {
	return reconstructHistory(ctx, historyRun{
		dir:           ".",
		siteURL:       config.Site.URL,
		frappeVersion: config.Site.Frappe.Version,
		modulesFile:   "modules.txt",
		fetcher:       fetcher,
		result:        result,
		since:         config.Sync.LastSync,
		until:         syncedAt,
		fromCommitted: true,
		recordSync: func() (func(), error) {
			prev := config.Sync.LastSync
			config.Sync.LastSync = syncedAt
			if err := config.Save("."); err != nil {
				config.Sync.LastSync = prev
				return nil, wegerrors.Config("site.toml", "write", err)
			}
			return func() {
				config.Sync.LastSync = prev
				if err := config.Save("."); err != nil {
					output.Warningf("Could not restore last_sync in site.toml: %v", err)
				}
			}, nil
		},
	})
}
