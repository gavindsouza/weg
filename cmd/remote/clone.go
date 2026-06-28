/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

	// Fetch entities
	output.Print("Fetching customizations...")
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
		writeAllEntities(dirName, result.Entities, modulesFile)
	}

	// Update sync timestamp
	config.Sync.LastSync = time.Now()
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
		if err := reconstructHistory(dirName, siteURL, frappeVersion, modulesFile, fetcher, result); err != nil {
			return err
		}
	}

	// Initialize workspace
	output.Print("Initializing workspace...")
	initWorkspace(dirName)

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

// reconstructHistory runs the streaming version-history pipeline: a parallel,
// resumable fetch to an on-disk cache, forward reconstruction into per-version
// commits (bounded memory), then a reconcile to the current site state.
func reconstructHistory(dirName, siteURL, frappeVersion, modulesFile string, fetcher *remote.Fetcher, result *remote.FetchResult) error {
	tmpDir := filepath.Join(dirName, ".weg", "tmp", "versions")

	// Phase 1: fetch versions to disk in parallel, resuming from any cursors.
	output.Print("Fetching version history...")
	var mu sync.Mutex
	counts := map[string]int{}
	err := fetcher.FetchVersionsToDisk(tmpDir, func(dt string, n int) {
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
	if err != nil {
		// The cache is preserved for resume; make the clone usable now.
		output.Warningf("Version history incomplete (rerun clone to resume): %v", err)
		writeAllEntities(dirName, result.Entities, modulesFile)
		gitCommitAll(dirName, fmt.Sprintf("Initial clone from %s\n\nFrappe version: %s\nEntities: %d (history pending)",
			siteURL, frappeVersion, len(result.Entities)))
		return nil
	}

	// Phase 2: determine which DocType histories are in scope.
	customDoctypes, err := fetcher.ResolveCustomDoctypes(tmpDir, customDoctypeSet(result.Entities))
	if err != nil {
		return fmt.Errorf("failed to resolve custom doctypes: %w", err)
	}

	// Resolve author names so git blame shows who changed what.
	output.Print("Fetching user information...")
	users := map[string]remote.UserInfo{}
	if owners, err := remote.CollectVersionOwners(tmpDir); err == nil {
		if u, err := fetcher.Client.GetUsers(owners); err == nil {
			users = u
		}
	}

	// Phase 3: replay history forward into per-version commits.
	output.Print("Reconstructing history...")
	commitCount := 0
	seenPaths, err := fetcher.StreamHistory(tmpDir, customDoctypes, result.Entities, users, func(c remote.ReconstructedCommit) error {
		if err := writeFileContent(dirName, c.FilePath, c.Content); err != nil {
			output.Errorf("Failed to write %s: %v", c.FilePath, err)
			return nil
		}
		runGit(dirName, "add", c.FilePath)
		if gitNothingStaged(dirName) {
			return nil
		}
		gitCommitAuthored(dirName, c.Author, formatGitTimestamp(c.Timestamp), c.Message)
		commitCount++
		return nil
	})
	if err != nil {
		output.Warningf("History reconstruction stopped early: %v", err)
	}

	// Phase 4: reconcile to the authoritative current state.
	writeAllEntities(dirName, result.Entities, modulesFile)
	currentPaths := entityPathSet(result.Entities)
	for p := range seenPaths {
		if !currentPaths[p] {
			// Entity deleted/renamed away: keep its history, drop it from HEAD.
			runGit(dirName, "rm", "-f", "--ignore-unmatch", p)
		}
	}
	runGit(dirName, "add", "-A")
	if !gitNothingStaged(dirName) {
		gitCommitAuthored(dirName, "Weg <noreply@weg.io>", "",
			fmt.Sprintf("chore(sync): reconcile to current site state\n\nSource: %s", siteURL))
	}

	output.Printf("Reconstructed %d commits from version history", commitCount)

	// Success: drop the staging cache.
	os.RemoveAll(filepath.Join(dirName, ".weg", "tmp"))
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

func runGit(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Run()
}

// gitNothingStaged reports whether the index has no staged changes.
func gitNothingStaged(dir string) bool {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// gitCommitAuthored commits staged changes with an optional author and date.
func gitCommitAuthored(dir, author, date, msg string) {
	args := []string{"commit"}
	if author != "" {
		args = append(args, "--author", author)
	}
	if date != "" {
		args = append(args, "--date", date)
	}
	args = append(args, "-m", msg)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if date != "" {
		cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+date)
	}
	cmd.Run()
}

func gitCommitAll(dir, msg string) {
	runGit(dir, "add", "-A")
	gitCommitAuthored(dir, "", "", msg)
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

// writeFileContent writes JSON content to a file path
// writeAllEntities writes the current state of every entity to disk and updates
// modules.txt. Used for --no-history clones and as a fallback whenever version
// history is unavailable, so the working tree is never left empty.
func writeAllEntities(dirName string, entities []remote.Entity, modulesFile string) {
	if len(entities) == 0 {
		return
	}

	bar := progressbar.NewOptions(len(entities),
		progressbar.OptionSetDescription("Writing files"),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionClearOnFinish(),
	)

	mods := make(map[string]bool)
	for _, entity := range entities {
		if err := remote.WriteEntity(dirName, entity); err != nil {
			output.Errorf("Failed to write %s: %v", entity.Name, err)
			continue
		}
		mods[entity.Module] = true
		bar.Add(1)
	}

	var moduleList []string
	for m := range mods {
		moduleList = append(moduleList, m)
	}
	modulesContent := strings.Join(moduleList, "\n") + "\n"
	os.WriteFile(modulesFile, []byte(modulesContent), 0644)
}

func writeFileContent(baseDir, filePath string, content map[string]any) error {
	fullPath := filepath.Join(baseDir, filePath)

	// Create directory
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	return nil
}

// formatGitTimestamp converts Frappe timestamp to git-compatible format
func formatGitTimestamp(ts string) string {
	// Frappe format: "2025-01-21 14:31:36.892644"
	// Git format: "2025-01-21T14:31:36"

	// Parse the timestamp
	layouts := []string{
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.Parse(layout, ts)
		if err == nil {
			break
		}
	}

	if err != nil {
		// Return as-is if parsing fails
		return ts
	}

	return t.Format(time.RFC3339)
}
