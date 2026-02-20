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

	// Check if directory already exists
	if _, err := os.Stat(dirName); err == nil {
		return wegerrors.Validation("path", fmt.Sprintf("directory %s already exists", dirName))
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
	if writeEntitiesUpfront && len(result.Entities) > 0 {
		bar := progressbar.NewOptions(len(result.Entities),
			progressbar.OptionSetDescription("Writing files"),
			progressbar.OptionSetWriter(os.Stdout),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(40),
			progressbar.OptionClearOnFinish(),
		)

		mods := make(map[string]bool)
		for _, entity := range result.Entities {
			if err := remote.WriteEntity(dirName, entity); err != nil {
				output.Errorf("Failed to write %s: %v", entity.Name, err)
				continue
			}
			mods[entity.Module] = true
			bar.Add(1)
		}

		// Update modules.txt
		var moduleList []string
		for m := range mods {
			moduleList = append(moduleList, m)
		}
		modulesContent := strings.Join(moduleList, "\n") + "\n"
		os.WriteFile(modulesFile, []byte(modulesContent), 0644)
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
		// Full history reconstruction from Version DocType
		output.Print("Fetching version history...")
		history, entitiesWithoutHistory, err := fetcher.FetchHistoryWithDocs(result.Entities)
		if err != nil {
			// Fall back to simple commit if history fetch fails
			output.Warningf("Could not fetch version history: %v", err)
			output.Print("Creating simple initial commit...")
			gitAdd := exec.Command("git", "add", "-A")
			gitAdd.Dir = dirName
			gitAdd.Run()
			commitMsg := fmt.Sprintf("Initial clone from %s\n\nFrappe version: %s\nEntities: %d",
				siteURL, frappeVersion, len(result.Entities))
			gitCommit := exec.Command("git", "commit", "-m", commitMsg)
			gitCommit.Dir = dirName
			gitCommit.Run()
		} else {
			// Fetch user information for author names
			output.Print("Fetching user information...")
			users, err := fetcher.FetchUsers(history)
			if err != nil {
				// Non-fatal: we'll fall back to email-derived names
				users = make(map[string]remote.UserInfo)
			}

			commitPlan := remote.BuildCommitPlan(history, result.Entities, entitiesWithoutHistory, users)

			if len(commitPlan) == 0 {
				// No history found, create a single commit
				output.Print("No version history found, creating initial commit...")
				gitAdd := exec.Command("git", "add", "-A")
				gitAdd.Dir = dirName
				gitAdd.Run()
				commitMsg := fmt.Sprintf("Initial clone from %s\n\nFrappe version: %s\nEntities: %d",
					siteURL, frappeVersion, len(result.Entities))
				gitCommit := exec.Command("git", "commit", "-m", commitMsg)
				gitCommit.Dir = dirName
				gitCommit.Run()
			} else {
				output.Infof("Reconstructing %d commits from version history...\n", len(commitPlan))

				bar := progressbar.NewOptions(len(commitPlan),
					progressbar.OptionSetDescription("Creating commits"),
					progressbar.OptionSetWriter(os.Stdout),
					progressbar.OptionShowCount(),
					progressbar.OptionSetWidth(40),
					progressbar.OptionClearOnFinish(),
				)

				// Track modules for modules.txt
				mods := make(map[string]bool)

				for _, commit := range commitPlan {
					// Write file contents for this commit (historical state)
					for filePath, content := range commit.FileContents {
						if err := writeFileContent(dirName, filePath, content); err != nil {
							output.Errorf("Failed to write %s: %v", filePath, err)
							continue
						}
						// Track module from path
						parts := strings.Split(filePath, "/")
						if len(parts) > 0 {
							mods[parts[0]] = true
						}
					}

					// Stage files for this commit
					for _, file := range commit.Files {
						gitAdd := exec.Command("git", "add", file)
						gitAdd.Dir = dirName
						gitAdd.Run()
					}

					// Check if there's anything staged
					gitDiff := exec.Command("git", "diff", "--cached", "--quiet")
					gitDiff.Dir = dirName
					if err := gitDiff.Run(); err == nil {
						// No changes staged, skip this commit
						bar.Add(1)
						continue
					}

					// Format timestamp for git (RFC3339 or ISO 8601)
					timestamp := formatGitTimestamp(commit.Timestamp)

					// Create commit with historical timestamp from version.creation
					// Both author date and committer date are set to preserve timeline
					gitCommit := exec.Command("git", "commit",
						"--date", timestamp,
						"--author", commit.Author,
						"-m", commit.Message,
					)
					gitCommit.Dir = dirName
					gitCommit.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+timestamp)
					gitCommit.Run()

					bar.Add(1)
				}

				// Write modules.txt after all commits
				if len(mods) > 0 {
					var moduleList []string
					for m := range mods {
						moduleList = append(moduleList, m)
					}
					modulesContent := strings.Join(moduleList, "\n") + "\n"
					os.WriteFile(modulesFile, []byte(modulesContent), 0644)
				}

				// Final commit for config files
				gitAdd := exec.Command("git", "add", "-A")
				gitAdd.Dir = dirName
				gitAdd.Run()

				// Check if there are uncommitted changes
				gitStatus := exec.Command("git", "status", "--porcelain")
				gitStatus.Dir = dirName
				statusOutput, _ := gitStatus.Output()
				if len(strings.TrimSpace(string(statusOutput))) > 0 {
					commitMsg := fmt.Sprintf("chore(config): initialize weg config\n\nSource: %s", siteURL)
					gitCommit := exec.Command("git", "commit",
						"--author", "Weg <noreply@weg.io>",
						"-m", commitMsg,
					)
					gitCommit.Dir = dirName
					gitCommit.Run()
				}
			}
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
