/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/gavindsouza/weg/internal/workspace"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	cloneAPIKey         string
	cloneAPISecret      string
	cloneModules        string
	cloneExclude        string
	cloneNonInteractive bool
	cloneNoHistory      bool
)

var cloneCmd = &cobra.Command{
	Use:   "clone <url> [directory]",
	Short: "Clone a remote site's customizations",
	Long: `Clone a remote Frappe site's customizations to a local git-backed directory.

This creates a local directory that mirrors the site's customization structure,
enabling local file editing, version control via git, and team collaboration.

The clone includes:
  - Custom DocTypes
  - Custom Fields (user-created, not system-generated)
  - Property Setters
  - Client Scripts
  - Server Scripts
  - Custom Reports
  - Custom Print Formats
  - Workflows
  - Notifications
  - Letter Heads

Authentication can be provided via:
  - Environment variables: WEG_API_KEY and WEG_API_SECRET
  - Command flags: --api-key and --api-secret
  - Interactive prompt during clone

Examples:
  weg clone https://mysite.frappe.cloud
  weg clone https://mysite.frappe.cloud mysite
  weg clone https://mysite.frappe.cloud --api-key=KEY --api-secret=SECRET
  weg clone https://mysite.frappe.cloud --modules=Custom,Selling`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runClone,
}

func init() {
	cloneCmd.Flags().StringVar(&cloneAPIKey, "api-key", "", "API key for authentication")
	cloneCmd.Flags().StringVar(&cloneAPISecret, "api-secret", "", "API secret for authentication")
	cloneCmd.Flags().StringVar(&cloneModules, "modules", "", "Comma-separated list of modules to sync")
	cloneCmd.Flags().StringVar(&cloneExclude, "exclude", "", "Comma-separated list of entity types to exclude")
	cloneCmd.Flags().BoolVar(&cloneNonInteractive, "non-interactive", false, "Skip interactive prompts")
	cloneCmd.Flags().BoolVar(&cloneNoHistory, "no-history", false, "Skip version history (faster, single commit)")
}

func runClone(cobraCmd *cobra.Command, args []string) error {
	siteURL := args[0]

	// Parse URL to extract site name
	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Ensure scheme is set; upgrade to https for non-localhost
	isLocalhost := strings.Contains(parsedURL.Hostname(), "localhost") ||
		parsedURL.Hostname() == "127.0.0.1" ||
		strings.HasSuffix(parsedURL.Hostname(), ".local")

	if parsedURL.Scheme == "" {
		if isLocalhost {
			parsedURL.Scheme = "http"
		} else {
			parsedURL.Scheme = "https"
		}
		siteURL = parsedURL.String()
	} else if parsedURL.Scheme == "http" && !isLocalhost {
		// Upgrade to https for remote sites
		parsedURL.Scheme = "https"
		siteURL = parsedURL.String()
	}

	// Determine directory name
	var dirName string
	if len(args) > 1 {
		dirName = args[1]
	} else {
		// Use hostname without domain
		host := parsedURL.Hostname()
		parts := strings.Split(host, ".")
		dirName = parts[0]
	}

	// Check if directory already exists. A dir holding a staged version cache is
	// a resumable partial clone: keep the expensive cache, wipe the rest, rebuild.
	if _, err := os.Stat(dirName); err == nil {
		if _, err := os.Stat(filepath.Join(dirName, ".weg", "tmp", "versions")); err == nil {
			output.Print("Resuming previous clone (reusing cached version history)...")
			if err := resetForResume(dirName); err != nil {
				return fmt.Errorf("failed to prepare resume: %w", err)
			}
		} else {
			return wegerrors.Validation("path", fmt.Sprintf("directory %s already exists", dirName))
		}
	}

	// Get credentials - resolution hierarchy:
	// 1. Command flags
	// 2. Environment variables
	// 3. Global credentials (~/.config/weg/credentials.toml)
	// 4. Interactive prompt

	apiKey := cloneAPIKey
	apiSecret := cloneAPISecret
	siteHost := parsedURL.Hostname()
	fromGlobal := false

	// Try environment variables
	if apiKey == "" {
		apiKey = os.Getenv("WEG_API_KEY")
	}
	if apiSecret == "" {
		apiSecret = os.Getenv("WEG_API_SECRET")
	}

	// Try global credentials
	if apiKey == "" || apiSecret == "" {
		if remote.HasGlobalCredentials(siteHost) {
			globalCreds, err := remote.LoadGlobalCredentials()
			if err == nil {
				if auth := globalCreds.Sites[siteHost]; auth != nil {
					apiKey = auth.APIKey
					apiSecret = auth.APISecret
					fromGlobal = true
					output.Printf("Using saved credentials for %s", siteHost)
				}
			}
		}
	}

	// Interactive prompt if needed
	if apiKey == "" || apiSecret == "" {
		if cloneNonInteractive {
			return wegerrors.Validation("credentials", "credentials required: set WEG_API_KEY and WEG_API_SECRET, use --api-key/--api-secret, or save globally with 'weg remote login'")
		}

		output.Print("")
		output.Print("SECURITY SETUP REQUIRED")
		output.Print("")
		output.Print("Remote sync requires API access to modify site customizations.")
		output.Print("Before proceeding, ensure you have API credentials for the site.")
		output.Print("")
		output.Print("To create API credentials on your Frappe site:")
		output.Print("  1. Go to User Settings > API Access")
		output.Print("  2. Generate a new API Key + Secret")
		output.Print("  3. Ensure the user has permissions for customizations")
		output.Print("")

		reader := bufio.NewReader(os.Stdin)

		if apiKey == "" {
			fmt.Print("API Key: ")
			apiKey, _ = reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)
		}

		if apiSecret == "" {
			fmt.Print("API Secret: ")
			apiSecret, _ = reader.ReadString('\n')
			apiSecret = strings.TrimSpace(apiSecret)
		}
	}

	if apiKey == "" || apiSecret == "" {
		return wegerrors.Validation("credentials", "API key and secret are required")
	}

	// Create client and test connection
	output.Infof("Connecting to %s...\n", siteURL)
	client := remote.NewClient(siteURL, apiKey, apiSecret)

	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect to site: %w", err)
	}
	output.Print("Connected")

	// Offer to save credentials globally if not already from global
	if !fromGlobal && !cloneNonInteractive {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Save credentials globally for future use? [Y/n]: ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			if err := remote.SaveGlobalCredentials(siteHost, &remote.CredentialAuth{
				APIKey:    apiKey,
				APISecret: apiSecret,
			}); err != nil {
				output.Warningf("Failed to save global credentials: %v", err)
			} else {
				output.Print("Credentials saved to ~/.config/weg/credentials.toml")
			}
		}
	}

	// Get Frappe version
	frappeVersion, err := client.GetFrappeVersion()
	if err != nil {
		frappeVersion = "unknown"
	}
	output.Printf("Frappe version: %s", frappeVersion)

	// Create site config
	config := remote.NewSiteConfig(siteURL, dirName)
	config.Site.Frappe.Version = frappeVersion

	// Parse module filter
	if cloneModules != "" {
		modules := strings.Split(cloneModules, ",")
		for _, m := range modules {
			m = strings.TrimSpace(m)
			if m != "" {
				config.Modules[m] = remote.ModuleInfo{App: "_site", Sync: true}
			}
		}
	}

	// Parse exclude filter
	if cloneExclude != "" {
		excludes := strings.Split(cloneExclude, ",")
		for _, e := range excludes {
			e = strings.TrimSpace(e)
			switch e {
			case "doctype":
				config.Sync.Entities.DocType = false
			case "custom_field":
				config.Sync.Entities.CustomField = false
			case "property_setter":
				config.Sync.Entities.PropertySetter = false
			case "client_script":
				config.Sync.Entities.ClientScript = false
			case "server_script":
				config.Sync.Entities.ServerScript = false
			case "report":
				config.Sync.Entities.Report = false
			case "print_format":
				config.Sync.Entities.PrintFormat = false
			case "workflow":
				config.Sync.Entities.Workflow = false
			case "notification":
				config.Sync.Entities.Notification = false
			case "workspace":
				config.Sync.Entities.Workspace = false
			case "letter_head":
				config.Sync.Entities.LetterHead = false
			}
		}
	}

	// Create directory
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Initialize git repo
	output.Print("Initializing git repository...")
	gitInit := exec.Command("git", "init")
	gitInit.Dir = dirName
	if err := gitInit.Run(); err != nil {
		return fmt.Errorf("failed to initialize git: %w", err)
	}

	// Save config
	if err := config.Save(dirName); err != nil {
		return wegerrors.Config("site.toml", "write", err)
	}

	// Save credentials (gitignored)
	creds := &remote.Credentials{
		Auth: remote.CredentialAuth{
			APIKey:    apiKey,
			APISecret: apiSecret,
		},
	}
	if err := creds.Save(dirName); err != nil {
		return wegerrors.Config("credentials", "write", err)
	}

	// Ensure credentials are gitignored
	if err := remote.EnsureGitignore(dirName); err != nil {
		return fmt.Errorf("failed to create gitignore: %w", err)
	}

	// Create modules.txt
	modulesFile := filepath.Join(dirName, "modules.txt")
	if err := os.WriteFile(modulesFile, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create modules.txt: %w", err)
	}

	// Fetch entities. The sync time is taken before the fetch: a site change
	// made while it runs is then picked up again by the next pull instead of
	// being missed.
	output.Print("Fetching customizations...")
	syncedAt := time.Now()
	fetcher := remote.NewFetcher(client, config)
	result, err := fetcher.FetchAll()
	if err != nil {
		return fmt.Errorf("failed to fetch customizations: %w", err)
	}

	// Update config with discovered modules and apps
	for name, info := range result.Modules {
		if _, exists := config.Modules[name]; !exists {
			config.Modules[name] = info
		}
	}
	for name, info := range result.Apps {
		config.Site.Apps[name] = info
	}

	// Write entities to disk (only if not doing history reconstruction)
	// When doing history reconstruction, entities are written incrementally per commit
	writeEntitiesUpfront := cloneNoHistory
	if writeEntitiesUpfront {
		if err := writeAllEntities(dirName, result.Entities, modulesFile); err != nil {
			return err
		}
	}

	// Update sync timestamp
	config.Sync.LastSync = syncedAt
	if err := config.Save(dirName); err != nil {
		return wegerrors.Config("site.toml", "write", err)
	}

	// Create git commits
	if cloneNoHistory {
		// Simple: single initial commit
		output.Print("Creating initial commit...")
		gitAdd := exec.Command("git", "add", "-A")
		gitAdd.Dir = dirName
		if err := gitAdd.Run(); err != nil {
			return fmt.Errorf("failed to stage files: %w", err)
		}

		commitMsg := fmt.Sprintf("Initial clone from %s\n\nFrappe version: %s\nEntities: %d",
			siteURL, frappeVersion, len(result.Entities))
		gitCommit := exec.Command("git", "commit", "-m", commitMsg)
		gitCommit.Dir = dirName
		gitCommit.Run() // Might fail if nothing to commit, that's ok
	} else {
		// Clone replays the entire history up to the sync time: zero
		// `since` = no lower bound. Later Versions belong to the next pull.
		if err := reconstructHistory(cobraCmd.Context(), historyRun{
			dir:           dirName,
			siteURL:       siteURL,
			frappeVersion: frappeVersion,
			modulesFile:   modulesFile,
			fetcher:       fetcher,
			result:        result,
			until:         syncedAt,
		}); err != nil {
			return err
		}
	}

	// Initialize workspace
	output.Print("Initializing workspace...")
	initWorkspace(dirName)

	// Everything in HEAD came from the site, so record it as the push baseline.
	// Without this, the first `weg remote push` diffs against the root commit
	// and replays the entire reconstructed history back at the site.
	saveLastPushCommit(dirName)

	// Summary
	output.Print("")
	output.Printf("Cloned to %s/", dirName)
	output.Print("")
	output.Printf("  Entities: %d", len(result.Entities))
	output.Printf("  Modules:  %d", len(modules(result.Entities)))
	output.Print("")
	output.Print("Next steps:")
	output.Printf("  cd %s", dirName)
	output.Print("  weg workspace expand    # Extract scripts for editing")
	output.Print("  weg remote status       # Check sync state")
	output.Print("  weg remote push         # Push local changes")

	return nil
}

func modules(entities []remote.Entity) map[string]bool {
	m := make(map[string]bool)
	for _, e := range entities {
		m[e.Module] = true
	}
	return m
}

// historyRun configures one run of the version-history pipeline.
type historyRun struct {
	dir           string
	siteURL       string
	frappeVersion string
	modulesFile   string
	fetcher       *remote.Fetcher
	result        *remote.FetchResult

	// since/until bound the replayed Version records: zero since replays the
	// whole history (clone), a non-zero since only what is newer (pull).
	since, until time.Time

	// fromCommitted starts each document's replay from its committed file
	// (pull); otherwise documents are replayed from nothing (clone).
	fromCommitted bool

	// recordSync runs right before the final commit is staged (pull: writes
	// the new last_sync so it lands in that commit). The returned restore
	// puts the previous value back if that commit fails. May be nil.
	recordSync func() (restore func(), err error)
}

// commitFinal writes the new sync state, stages everything under run.dir and
// commits it with the given author and message. Only run.dir is staged and
// committed: when the export is a subdirectory of a larger repository, the
// rest of that repository is left as it was. If staging or the commit fails,
// the sync state is restored, so the next run covers the same window again.
func commitFinal(run historyRun, author, msg string) error {
	restore := func() {}
	if run.recordSync != nil {
		r, err := run.recordSync()
		if err != nil {
			return err
		}
		restore = r
	}
	err := func() error {
		if err := runGit(run.dir, "add", "-A", "--", "."); err != nil {
			return err
		}
		staged, err := gitHasStaged(run.dir, ".")
		if err != nil || !staged {
			return err
		}
		return gitCommitAuthored(run.dir, author, "", msg, ".")
	}()
	if err != nil {
		restore()
	}
	return err
}

// reconstructHistory runs the streaming version-history pipeline: a parallel,
// resumable fetch to an on-disk cache, forward reconstruction into per-version
// commits (bounded memory), then a reconcile to the current site state.
//
// The trailing reconcile commit captures any current-state changes not
// attributable to a Version record, so nothing is dropped.
//
// If the version history cannot be fetched, the current state is committed
// as a single snapshot (so the working tree is usable) and an error is still
// returned: the run exits non-zero so automation notices that no per-change
// history was recorded for the window. A git failure while replaying stops
// the run before the reconcile commit and, on a pull, rolls the replayed
// commits back, so a rerun neither builds on a truncated replay nor commits
// its changes twice.
func reconstructHistory(ctx context.Context, run historyRun) error {
	// Ctrl-C or SIGTERM stops the run at the next commit boundary, so the
	// rollback below runs instead of the process dying mid-replay.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	startHead := ""
	if run.fromCommitted {
		startHead = gitHead(run.dir)
	}
	touched := make(map[string]bool)
	err := replayHistory(ctx, run, touched)
	var fallback *historyFallbackError
	if err == nil || startHead == "" || errors.As(err, &fallback) {
		return err
	}
	if len(touched) == 0 && gitHead(run.dir) == startHead {
		return err // stopped before writing anything
	}
	if rbErr := restoreExport(run.dir, startHead, touched); rbErr != nil {
		return fmt.Errorf("%w; restoring the export to %s also failed: %v", err, startHead, rbErr)
	}
	output.Warningf("Restored the export to %s (no commits kept, last_sync unchanged); rerun to replay the window", startHead)
	return err
}

// historyFallbackError reports that the version history was unavailable and
// the current state was committed as a snapshot instead. The snapshot is
// kept: only the history is missing.
type historyFallbackError struct{ err error }

func (e *historyFallbackError) Error() string {
	return "version history unavailable; committed a snapshot without per-change history: " + e.err.Error()
}
func (e *historyFallbackError) Unwrap() error { return e.err }

// replayHistory does the work of reconstructHistory, recording in touched
// every path (relative to run.dir) it writes or removes.
func replayHistory(ctx context.Context, run historyRun, touched map[string]bool) error {
	dirName, fetcher, result := run.dir, run.fetcher, run.result
	tmpDir := filepath.Join(dirName, ".weg", "tmp", "versions")
	touchExport := func() {
		for _, e := range result.Entities {
			// A refused path is never written, so there is nothing to restore.
			if remote.CheckExportPath(e.FilePath) == nil {
				touched[e.FilePath] = true
			}
		}
		if rel, err := filepath.Rel(dirName, run.modulesFile); err == nil {
			touched[filepath.ToSlash(rel)] = true
		}
		touched[".weg/site.toml"] = true
	}

	// Version.creation is naive site-local time: resolve the site's zone so
	// the window filter and the commit dates use the true offset.
	// A clone replays everything, so a wrong zone only shifts commit dates.
	// A pull's window would start hours off, replaying Versions the last pull
	// covered (false reverts, duplicates) or skipping some: stop before
	// fetching anything, leaving the export and last_sync untouched.
	loc, tzErr := fetcher.SiteLocation()
	switch {
	case tzErr != nil && !run.since.IsZero():
		return wegerrors.Operation("pull", "cannot place last_sync in site time, so the version window is unknown; nothing was replayed and last_sync is unchanged", tzErr)
	case tzErr != nil:
		output.Warningf("%v; treating site times as UTC, so commit dates may be off by the site's UTC offset", tzErr)
	default:
		output.Printf("Site time zone: %s", loc)
	}
	window := remote.HistoryWindow{Since: run.since, Until: run.until, Location: loc}

	// What has local work not yet pushed is read before the replay commits
	// anything, so the pull's own commits never count as local work.
	var local localState
	if run.fromCommitted {
		local = readLocalState(dirName)
	}

	// Phase 1: fetch versions to disk in parallel, resuming from any cursors.
	output.Print("Fetching version history...")
	var mu sync.Mutex
	counts := map[string]int{}
	err := fetcher.FetchVersionsToDisk(ctx, tmpDir, window, func(dt string, n int) {
		mu.Lock()
		counts[dt] = n
		total := 0
		for _, c := range counts {
			total += c
		}
		mu.Unlock()
		fmt.Printf("\r  fetched %d version records", total)
	})
	fmt.Println()
	if ctx.Err() != nil {
		return fmt.Errorf("interrupted while fetching version history (rerun to resume): %w", ctx.Err())
	}
	if err != nil {
		touchExport()
		return snapshotAfterHistoryFailure(run, local, err)
	}

	// Frappe writes no Version on delete; its Deleted Document rows say who
	// deleted what, and when. Without them the deletions still land, in the
	// reconcile commit.
	deletions, err := fetcher.FetchDeletions(window)
	if err != nil {
		output.Warningf("Could not read the site's Deleted Document rows (%v); deletions land in the reconcile commit instead of in commits by who deleted them", err)
		deletions = nil
	} else if len(deletions) > 0 {
		output.Printf("  fetched %d deleted documents", len(deletions))
	}

	// Phase 2: determine which DocType histories are in scope.
	customDoctypes, err := fetcher.ResolveCustomDoctypes(tmpDir, customDoctypeSet(result.Entities))
	if err != nil {
		return fmt.Errorf("failed to resolve custom doctypes: %w", err)
	}

	opts := remote.HistoryOptions{Window: window, Deletions: deletions}
	if run.fromCommitted {
		baseline, err := readCommittedFiles(dirName)
		if err != nil {
			return fmt.Errorf("failed to read the committed export: %w", err)
		}
		opts.Baseline = baseline
	}

	// Resolve author names so git blame shows who changed what.
	users := func(emails []string) map[string]remote.UserInfo {
		output.Print("Fetching user information...")
		u, err := fetcher.Client.GetUsers(emails)
		if err != nil {
			output.Warningf("Could not resolve author names (using emails): %v", err)
			return nil
		}
		return u
	}

	// Phase 3: replay history forward into per-version commits.
	output.Print("Reconstructing history...")
	commitCount := 0
	hist, err := fetcher.StreamHistory(tmpDir, customDoctypes, result.Entities, users, opts, func(c remote.ReconstructedCommit) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("interrupted: %w", err)
		}
		for _, p := range c.Removed {
			if local.changed[p] {
				output.Warningf("%s was deleted on the site (%s), so the pull removes it, but it had local changes not yet pushed: uncommitted edits are gone from the working tree, committed ones stay in git history", p, c.Author)
			}
		}
		committed, err := applyReplayedCommit(dirName, c, touched)
		if committed {
			commitCount++
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("history replay stopped after %d commits, before the reconcile commit: %w", commitCount, err)
	}
	if hist.OutsideWindow > 0 {
		output.Printf("Skipped %d version records outside the window (already replayed)", hist.OutsideWindow)
	}
	if hist.Unexported > 0 {
		output.Printf("Skipped %d version records of documents outside the export", hist.Unexported)
	}
	if hist.Deletions > 0 {
		output.Printf("Replayed %d deletions", hist.Deletions)
	}
	if hist.DeletionsUnexported > 0 {
		output.Printf("Skipped %d deletions of documents outside the export", hist.DeletionsUnexported)
	}
	if n := len(hist.Dropped); n > 0 {
		output.Warningf("Dropped %d version records that cannot be replayed:", n)
		for i, d := range hist.Dropped {
			if i == 10 {
				output.Warningf("  ... and %d more", n-i)
				break
			}
			output.Warningf("  %s", d)
		}
	}

	// Phase 4: reconcile to the authoritative current state. A pull first
	// works out which committed files the site no longer has: a deletion
	// the window's trash rows did not cover, a module move's old path, a
	// DocType that is no longer custom. A deletion the site records is
	// committed by who deleted it, dated when; the rest go in the reconcile.
	var removals []remote.Removal
	if run.fromCommitted {
		removals, err = planStaleExport(dirName, result, fetcher, local, hist.SeenPaths)
		if err != nil {
			return err
		}
		removals, err = commitEarlierDeletions(dirName, fetcher, window, removals, hist.SeenPaths, touched, &commitCount)
		if err != nil {
			return err
		}
	}
	touchExport()
	if err := writeAllEntities(dirName, result.Entities, run.modulesFile); err != nil {
		return err
	}
	if run.fromCommitted {
		if err := removeExportFiles(dirName, removals, touched); err != nil {
			return err
		}
	} else if len(result.ModuleLookupFailed) == 0 {
		// A clone holds only what its replay wrote: a document with history
		// that the fetch no longer has was deleted or renamed away. Keep its
		// history, drop it from HEAD, unless its type's fetch was incomplete.
		currentPaths := entityPathSet(result.Entities)
		for p := range hist.SeenPaths {
			if currentPaths[p] {
				continue
			}
			if et, ok := remote.EntityTypeOfPath(p); ok && len(result.Incomplete[et]) > 0 {
				continue
			}
			if err := remote.CheckExportPath(p); err != nil {
				return err
			}
			touched[p] = true
			if err := runGit(dirName, "rm", "-q", "-f", "--ignore-unmatch", "--", p); err != nil {
				return err
			}
		}
	}
	// Keep the workspace in step with the reconciled JSONs so the refreshed
	// files land in the same reconcile commit.
	refreshWorkspaceAfterPull(dirName)
	if err := commitFinal(run, "Weg <noreply@weg.io>",
		fmt.Sprintf("chore(sync): reconcile to current site state\n\nSource: %s", run.siteURL)); err != nil {
		return fmt.Errorf("reconcile commit: %w", err)
	}

	output.Printf("Reconstructed %d commits from version history", commitCount)

	// Success: drop the staging cache.
	os.RemoveAll(filepath.Join(dirName, ".weg", "tmp"))
	return nil
}

// applyReplayedCommit writes or removes a replayed commit's files and
// commits only those, with the commit's author and date: anything else
// already staged must not be credited to the site user. Every path is
// checked to stay inside the export before it is touched. It reports
// whether a commit was made (none when the files were already as replayed).
func applyReplayedCommit(dir string, c remote.ReconstructedCommit, touched map[string]bool) (bool, error) {
	paths := append([]string(nil), c.Removed...)
	if c.FilePath != "" {
		paths = append(paths, c.FilePath)
	}
	if len(paths) == 0 {
		return false, nil
	}
	for _, p := range paths {
		if err := remote.CheckExportPath(p); err != nil {
			return false, err
		}
	}
	if len(c.Removed) > 0 {
		for _, p := range c.Removed {
			touched[p] = true
		}
		if err := runGit(dir, append([]string{"rm", "-q", "-f", "--ignore-unmatch", "--"}, c.Removed...)...); err != nil {
			return false, err
		}
	}
	if c.FilePath != "" {
		touched[c.FilePath] = true
		if err := writeFileContent(dir, c.FilePath, c.Content); err != nil {
			return false, err
		}
		if err := runGit(dir, "add", "--", c.FilePath); err != nil {
			return false, err
		}
	}
	staged, err := gitHasStaged(dir, paths...)
	if err != nil || !staged {
		return false, err
	}
	if err := gitCommitAuthored(dir, c.Author, formatGitDate(c.When), c.Message, paths...); err != nil {
		return false, fmt.Errorf("commit %s (%s): %w", strings.Join(paths, ", "), c.VersionName, err)
	}
	return true, nil
}

// commitEarlierDeletions commits, one each, the removals whose deletion
// the site records in a Deleted Document row (from before the pull's
// window, since the window's own rows were replayed) by the user who deleted
// the document, dated when, oldest first. It returns the other removals,
// left for the reconcile commit. A file a Version in the window wrote was
// alive after any such row, so it is left for the reconcile too.
func commitEarlierDeletions(dir string, fetcher *remote.Fetcher, window remote.HistoryWindow, removals []remote.Removal, seen map[string]bool, touched map[string]bool, count *int) ([]remote.Removal, error) {
	var earlier, rest []remote.Removal
	owners := map[string]bool{}
	for _, r := range removals {
		if r.Deletion != nil && !seen[r.Path] {
			earlier = append(earlier, r)
			owners[r.Deletion.Owner] = true
		} else {
			rest = append(rest, r)
		}
	}
	if len(earlier) == 0 {
		return rest, nil
	}
	sort.SliceStable(earlier, func(i, j int) bool {
		a, b := earlier[i].Deletion, earlier[j].Deletion
		if a.Creation != b.Creation {
			return a.Creation < b.Creation
		}
		return earlier[i].Path < earlier[j].Path
	})
	emails := make([]string, 0, len(owners))
	for e := range owners {
		emails = append(emails, e)
	}
	sort.Strings(emails)
	users, err := fetcher.Client.GetUsers(emails)
	if err != nil {
		output.Warningf("Could not resolve author names (using emails): %v", err)
	}
	output.Printf("Committing %d deletions the site recorded before this pull's window:", len(earlier))
	for _, r := range earlier {
		output.Printf("  - %s (deleted by %s at %s)", r.Path, r.Deletion.Owner, r.Deletion.Creation)
		committed, err := applyReplayedCommit(dir, remote.DeletionCommit(r, window, users), touched)
		if err != nil {
			return nil, err
		}
		if committed {
			*count++
		}
	}
	return rest, nil
}

// removeExportFiles removes the given files from the index and the working
// tree, recording them in touched (when given), and prints them.
func removeExportFiles(dir string, removals []remote.Removal, touched map[string]bool) error {
	if len(removals) == 0 {
		return nil
	}
	paths := make([]string, 0, len(removals))
	for _, r := range removals {
		if err := remote.CheckExportPath(r.Path); err != nil {
			return err
		}
		paths = append(paths, r.Path)
	}
	for _, p := range paths {
		if touched != nil {
			touched[p] = true
		}
	}
	if err := runGitPaths(dir, paths, "rm", "-q", "-f", "--ignore-unmatch"); err != nil {
		return err
	}
	output.Printf("Removed %d files the site no longer has:", len(removals))
	for _, r := range removals {
		output.Printf("  - %s (%s)", r.Path, r.Reason)
	}
	return nil
}

// planStaleExport returns the committed export files under dir that the
// current fetch no longer produces (remote.StaleExportFiles) and the site
// confirms are gone (Fetcher.ConfirmStale), warning about what it keeps.
// Only files under dir are listed, never .weg/ or the workspace. A file
// with local work not yet pushed (uncommitted, or committed since the push
// baseline) is kept: it may be a document made locally for the next push.
// When that local work cannot be told, or the fetch was incomplete, nothing
// is listed. seen holds the files a Version in the pull's window wrote.
func planStaleExport(dir string, result *remote.FetchResult, fetcher *remote.Fetcher, local localState, seen map[string]bool) ([]remote.Removal, error) {
	for et, docs := range result.Incomplete {
		output.Warningf("Could not read %d %s documents, so no %s file is removed: %s",
			len(docs), et, et, strings.Join(docs, "; "))
	}
	if n := len(result.ModuleLookupFailed); n > 0 {
		output.Warningf("Could not read the module of %d DocTypes, so no file is removed: %s",
			n, strings.Join(result.ModuleLookupFailed, "; "))
		return nil, nil
	}
	committed, err := committedExportPaths(dir)
	if err != nil {
		return nil, err
	}
	candidates := remote.StaleExportFiles(committed, result, fetcher.Config)
	if len(candidates) == 0 {
		return nil, nil
	}
	if local.err != nil {
		output.Warningf("%v; not removing the %d files the site no longer exports", local.err, len(candidates))
		return nil, nil
	}
	var stale []remote.StaleFile
	var kept []string
	content := make(map[string][]byte, len(candidates))
	for _, c := range candidates {
		if local.changed[c.Path] {
			kept = append(kept, c.Path)
			continue
		}
		out, err := gitCmd(dir, "show", "HEAD:./"+c.Path).Output()
		if err != nil {
			return nil, wegerrors.Operation("git show", "could not read "+c.Path, err)
		}
		content[c.Path] = out
		stale = append(stale, c)
	}
	if len(kept) > 0 {
		output.Warningf("Kept %d files the site does not export, since they have local changes not yet pushed:", len(kept))
		for _, p := range kept {
			output.Warningf("  %s", p)
		}
	}
	ev := remote.StaleEvidence{Content: content, Seen: seen}
	if local.hasBaseline {
		ev.Pushed = func(p string) bool { return !local.changed[p] }
	}
	removals, notes := fetcher.ConfirmStale(stale, result, ev)
	if len(notes) > 0 {
		output.Warningf("Kept %d files the site does not export:", len(notes))
		for _, n := range notes {
			output.Warningf("  %s", n)
		}
	}
	return removals, nil
}

// localState is the export's local work a push has yet to send.
type localState struct {
	changed     map[string]bool // paths under the export dir, relative to it
	hasBaseline bool            // .weg/last_push_commit names a commit here
	err         error           // the local work cannot be told
}

// readLocalState returns the paths under dir (relative to it) with local
// work a push has yet to send: changes not committed, and changes committed
// since the push baseline (.weg/last_push_commit), when there is one. It
// fails when the baseline is not a commit of this repository, since the
// local work then cannot be told apart.
func readLocalState(dir string) localState {
	st := localState{changed: make(map[string]bool)}
	if gitHead(dir) == "" {
		return st
	}
	revs := [][]string{{"HEAD"}}
	if data, err := os.ReadFile(filepath.Join(dir, ".weg", "last_push_commit")); err == nil {
		if base := strings.TrimSpace(string(data)); base != "" {
			if gitCmd(dir, "rev-parse", "--verify", "--quiet", base+"^{commit}").Run() != nil {
				st.err = fmt.Errorf("the push baseline %s in .weg/last_push_commit is not a commit here", base)
				return st
			}
			st.hasBaseline = true
			revs = append(revs, []string{base, "HEAD"})
		}
	}
	for _, r := range revs {
		args := append(append([]string{"diff", "--name-only", "-z", "--relative"}, r...), "--", ".")
		out, err := gitCmd(dir, args...).Output()
		if err != nil {
			st.err = wegerrors.Operation("git diff", "could not list local changes", err)
			return st
		}
		for _, p := range strings.Split(string(out), "\x00") {
			if p != "" {
				st.changed[p] = true
			}
		}
	}
	return st
}

// committedExportPaths lists the files committed at HEAD under dir (relative
// to dir), outside .weg/ and the workspace. An unborn HEAD lists nothing.
func committedExportPaths(dir string) ([]string, error) {
	if gitHead(dir) == "" {
		return nil, nil
	}
	out, err := gitCmd(dir, "ls-tree", "-r", "-z", "--name-only", "HEAD").Output()
	if err != nil {
		return nil, wegerrors.Operation("git ls-tree", "could not list HEAD", err)
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" && !outsideExport(p) {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// outsideExport reports whether a path under the export directory is weg's
// own state or the workspace rather than exported site content.
func outsideExport(p string) bool {
	return strings.HasPrefix(p, ".weg/") || strings.HasPrefix(p, workspace.WorkspaceDir+"/")
}

// snapshotAfterHistoryFailure commits the current state as one snapshot when
// the version history could not be fetched, then returns an error so the run
// still exits non-zero. The staged cache is kept for a resume.
func snapshotAfterHistoryFailure(run historyRun, local localState, fetchErr error) error {
	dirName := run.dir
	output.Warningf("history-fallback: version history could not be fetched: %v", fetchErr)
	if err := writeAllEntities(dirName, run.result.Entities, run.modulesFile); err != nil {
		return err
	}
	if run.fromCommitted {
		removals, err := planStaleExport(dirName, run.result, run.fetcher, local, nil)
		if err != nil {
			return err
		}
		if err := removeExportFiles(dirName, removals, nil); err != nil {
			return err
		}
	}
	refreshWorkspaceAfterPull(dirName)

	var msg, retry string
	if !run.fromCommitted {
		msg = fmt.Sprintf("Initial clone from %s\n\nFrappe version: %s\nEntities: %d (history pending)",
			run.siteURL, run.frappeVersion, len(run.result.Entities))
		retry = "rerun clone to resume the history"
	} else {
		msg = fmt.Sprintf("chore(sync): snapshot pull from %s\n\nEntities: %d (history pending)",
			run.siteURL, len(run.result.Entities))
		retry = "the site changes in this window are committed without per-change history"
	}
	if err := commitFinal(run, "", msg); err != nil {
		return fmt.Errorf("snapshot commit after history failure: %w", err)
	}
	output.Warningf("history-fallback: committed the current state as a single snapshot; %s", retry)
	return &historyFallbackError{err: fetchErr}
}

// restoreExport undoes a failed pull: HEAD goes back to startHead and every
// path the pull wrote or removed gets its startHead content back (or is
// removed, if startHead did not have it). Staged changes to other paths are
// kept. Those paths were already overwritten with site content by the pull,
// so restoring them loses nothing that was not lost already.
func restoreExport(dir, startHead string, touched map[string]bool) error {
	if gitHead(dir) != startHead {
		if err := runGit(dir, "reset", "-q", "--soft", startHead); err != nil {
			return err
		}
	}
	out, err := gitCmd(dir, "ls-tree", "-r", "-z", "--name-only", startHead).Output()
	if err != nil {
		return wegerrors.Operation("git ls-tree", "could not list "+startHead, err)
	}
	inHead := make(map[string]bool)
	for _, p := range strings.Split(string(out), "\x00") {
		inHead[p] = true
	}
	var restore, remove []string
	for p := range touched {
		if !filepath.IsLocal(p) {
			continue // never written: paths are checked before they are touched
		}
		if inHead[p] {
			restore = append(restore, p)
		} else {
			remove = append(remove, p)
		}
	}
	sort.Strings(restore)
	sort.Strings(remove)
	if len(restore) > 0 {
		if err := runGitPaths(dir, restore, "checkout", "-q", startHead); err != nil {
			return err
		}
	}
	if len(remove) > 0 {
		if err := runGitPaths(dir, remove, "rm", "-q", "--cached", "--ignore-unmatch"); err != nil {
			return err
		}
		for _, p := range remove {
			if err := os.Remove(filepath.Join(dir, p)); err != nil && !os.IsNotExist(err) {
				output.Warningf("Could not remove %s left by the failed pull: %v", p, err)
			}
		}
	}
	return nil
}

// customDoctypeSet is the set of currently-custom DocType names (custom=1).
func customDoctypeSet(entities []remote.Entity) map[string]bool {
	set := make(map[string]bool)
	for _, e := range entities {
		if e.Type == remote.EntityDocType {
			set[e.Name] = true
		}
	}
	return set
}

// entityPathSet is the set of file paths for the current entities.
func entityPathSet(entities []remote.Entity) map[string]bool {
	set := make(map[string]bool)
	for _, e := range entities {
		set[e.FilePath] = true
	}
	return set
}

// resetForResume preserves the .weg version cache and credentials, wiping the
// rest so a resumed clone rebuilds cleanly from the cached (expensive) JSONL.
func resetForResume(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == ".weg" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	wegEntries, err := os.ReadDir(filepath.Join(dir, ".weg"))
	if err != nil {
		return err
	}
	for _, e := range wegEntries {
		if e.Name() == "tmp" || e.Name() == "credentials.toml" {
			continue
		}
		os.RemoveAll(filepath.Join(dir, ".weg", e.Name()))
	}
	return nil
}

// gitCmd builds a git command in dir. Pathspecs are literal: entity paths
// may contain glob characters ("sales_[old].json") that must not match
// other files.
func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	return cmd
}

// runGit runs a git command in dir, returning git's output in the error.
func runGit(dir string, args ...string) error {
	if out, err := gitCmd(dir, args...).CombinedOutput(); err != nil {
		return wegerrors.Operation("git "+args[0], strings.TrimSpace(string(out)), err)
	}
	return nil
}

// runGitPaths runs a git command that takes the given paths (relative to
// dir) as pathspecs, passed on stdin so any number of them fits. It runs
// from the top of the work tree with dir's prefix on each path: in a
// subdirectory, some commands (git rm) add the current directory as a
// pathspec of their own and then refuse --pathspec-from-file.
func runGitPaths(dir string, paths []string, args ...string) error {
	out, err := gitCmd(dir, "rev-parse", "--show-toplevel", "--show-prefix").Output()
	if err != nil {
		return wegerrors.Operation("git rev-parse", "could not locate the work tree of "+dir, err)
	}
	// --show-prefix prints an empty line at the top of the work tree.
	lines := strings.SplitN(strings.TrimRight(string(out), "\n"), "\n", 2)
	top, prefix := lines[0], ""
	if len(lines) == 2 {
		prefix = lines[1]
	}
	full := make([]string, len(paths))
	for i, p := range paths {
		full[i] = prefix + filepath.ToSlash(p)
	}
	args = append(args, "--pathspec-from-file=-", "--pathspec-file-nul")
	cmd := gitCmd(top, args...)
	cmd.Stdin = strings.NewReader(strings.Join(full, "\x00"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return wegerrors.Operation("git "+args[0], strings.TrimSpace(string(out)), err)
	}
	return nil
}

// gitHead returns the commit HEAD points at, or "" for an unborn branch.
func gitHead(dir string) string {
	out, err := gitCmd(dir, "rev-parse", "--verify", "--quiet", "HEAD^{commit}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitHasStaged reports whether the index has staged changes (limited to
// paths, when given).
func gitHasStaged(dir string, paths ...string) (bool, error) {
	args := []string{"diff", "--cached", "--quiet"}
	if len(paths) > 0 {
		args = append(append(args, "--"), paths...)
	}
	err := gitCmd(dir, args...).Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, wegerrors.Operation("git diff", "could not check staged changes", err)
}

// gitCommitAuthored commits staged changes (only paths, when given) with an
// optional author and date (any format git accepts; the committer date is set
// to match). Hooks are skipped: these commits record the site's state, and a
// workspace-collapse hook must not rewrite them.
func gitCommitAuthored(dir, author, date, msg string, paths ...string) error {
	args := []string{"commit", "--quiet", "--no-verify"}
	if author != "" {
		args = append(args, "--author", author)
	}
	if date != "" {
		args = append(args, "--date", date)
	}
	args = append(args, "-m", msg)
	if len(paths) > 0 {
		args = append(append(args, "--only", "--"), paths...)
	}
	cmd := gitCmd(dir, args...)
	if date != "" {
		cmd.Env = append(cmd.Env, "GIT_COMMITTER_DATE="+date)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return wegerrors.Operation("git commit", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// readCommittedFiles returns the JSON files committed at HEAD under dir (path
// relative to dir -> content), outside .weg/ and the workspace. An unborn HEAD
// yields an empty map. Reading HEAD rather than the working tree keeps
// uncommitted local edits out of the replayed site history.
//
// The export may be a subdirectory of the repository (weg -C catalog), so
// paths are listed relative to dir, the way every other git call here and
// the entity file paths see them.
func readCommittedFiles(dir string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	if gitHead(dir) == "" {
		return files, nil
	}
	ls := gitCmd(dir, "ls-tree", "-r", "-z", "HEAD")
	out, err := ls.Output()
	if err != nil {
		return nil, wegerrors.Operation("git ls-tree", "could not list HEAD", err)
	}

	var paths []string
	var batch bytes.Buffer
	for _, entry := range bytes.Split(out, []byte{0}) {
		// "<mode> <type> <sha>\t<path>"
		meta, path, ok := strings.Cut(string(entry), "\t")
		if !ok || !strings.HasSuffix(path, ".json") || !strings.Contains(meta, " blob ") || outsideExport(path) {
			continue
		}
		fields := strings.Fields(meta)
		paths = append(paths, path)
		batch.WriteString(fields[2] + "\n")
	}
	if len(paths) == 0 {
		return files, nil
	}

	cat := gitCmd(dir, "cat-file", "--batch")
	cat.Stdin = &batch
	blobs, err := cat.Output()
	if err != nil {
		return nil, wegerrors.Operation("git cat-file", "could not read HEAD blobs", err)
	}
	r := bufio.NewReader(bytes.NewReader(blobs))
	for _, path := range paths {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file: truncated output at %s", path)
		}
		var sha, kind string
		var size int
		if _, err := fmt.Sscanf(header, "%s %s %d", &sha, &kind, &size); err != nil {
			return nil, fmt.Errorf("git cat-file: unexpected header %q", strings.TrimSpace(header))
		}
		content := make([]byte, size)
		if _, err := io.ReadFull(r, content); err != nil {
			return nil, fmt.Errorf("git cat-file: short read for %s: %w", path, err)
		}
		if _, err := r.ReadByte(); err != nil { // trailing newline
			return nil, fmt.Errorf("git cat-file: truncated output at %s", path)
		}
		files[path] = content
	}
	return files, nil
}

// initWorkspace sets up the workspace directory with gitignore and pre-commit hooks
func initWorkspace(baseDir string) {
	// Create workspace directory
	workspaceDir := filepath.Join(baseDir, workspace.WorkspaceDir)
	os.MkdirAll(workspaceDir, 0755)

	// Add to gitignore
	gitignorePath := filepath.Join(baseDir, ".gitignore")
	content := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	// Add workspace to gitignore if not present
	if !strings.Contains(content, workspace.WorkspaceDir) {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			if content != "" && content[len(content)-1] != '\n' {
				f.WriteString("\n")
			}
			f.WriteString("\n# Expanded workspace for code editing\n")
			f.WriteString(workspace.WorkspaceDir + "/\n")
		}
	}

	// Create pre-commit config if it doesn't exist
	precommitPath := filepath.Join(baseDir, ".pre-commit-config.yaml")
	if _, err := os.Stat(precommitPath); os.IsNotExist(err) {
		precommitConfig := `# Pre-commit hooks for weg workspace
# Install with: pre-commit install

repos:
  # Collapse workspace before commit
  - repo: local
    hooks:
      - id: weg-workspace-collapse
        name: Collapse weg workspace
        entry: weg workspace collapse
        language: system
        pass_filenames: false
        files: ^weg_workspace/

  # Python linting with ruff
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.4.4
    hooks:
      - id: ruff
        files: ^weg_workspace/.*\.py$
        args: [--fix]
      - id: ruff-format
        files: ^weg_workspace/.*\.py$
`
		os.WriteFile(precommitPath, []byte(precommitConfig), 0644)
	}

	output.Print("Workspace initialized")
}

// writeAllEntities writes the current state of every entity to disk and updates
// modules.txt. Used for --no-history clones and as a fallback whenever version
// history is unavailable, so the working tree is never left empty.
func writeAllEntities(dirName string, entities []remote.Entity, modulesFile string) error {
	if len(entities) == 0 {
		return nil
	}

	bar := progressbar.NewOptions(len(entities),
		progressbar.OptionSetDescription("Writing files"),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionClearOnFinish(),
	)

	var failed []string
	mods := make(map[string]bool)
	for _, entity := range entities {
		if err := remote.WriteEntity(dirName, entity); err != nil {
			output.Errorf("Failed to write %s: %v", entity.Name, err)
			failed = append(failed, entity.Name)
			continue
		}
		mods[entity.Module] = true
		bar.Add(1)
	}

	// Sorted, so the same module set always writes the same file.
	moduleList := make([]string, 0, len(mods))
	for m := range mods {
		moduleList = append(moduleList, m)
	}
	sort.Strings(moduleList)
	modulesContent := strings.Join(moduleList, "\n") + "\n"
	if err := os.WriteFile(modulesFile, []byte(modulesContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", modulesFile, err)
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to write %d entities: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// writeFileContent writes JSON content to a file path through
// remote.WriteEntity, so a replayed file matches the exported one byte for
// byte and passes the same path check.
func writeFileContent(baseDir, filePath string, content map[string]any) error {
	return remote.WriteEntity(baseDir, remote.Entity{FilePath: filePath, Data: content})
}

// formatGitDate formats a replayed commit's time for git --date, keeping
// the site's UTC offset. A zero time (unparseable creation) yields "" so git
// uses the current time.
func formatGitDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
