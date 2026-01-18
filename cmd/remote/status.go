/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		fmt.Println("✓ Connected")

		// Fetch remote entities and compare with local
		remoteChanges, err := detectRemoteChanges(client, config, ".")
		if err != nil {
			fmt.Printf("Warning: Could not detect remote changes: %v\n", err)
		} else if len(remoteChanges) == 0 {
			fmt.Println("Remote: No changes detected")
		} else {
			fmt.Printf("Remote: %d entity change(s) detected\n", len(remoteChanges))
			for _, change := range remoteChanges {
				fmt.Printf("  %s: %s\n", change.Status, change.Name)
			}
			fmt.Println("\nRun 'weg remote pull' to fetch remote changes.")
		}
	}

	// Show sync instructions
	if localChanges != "" {
		fmt.Println()
		fmt.Println("To sync changes:")
		fmt.Println("  weg sync -m \"description\"")
	}

	return nil
}

// RemoteChange represents a detected change on the remote
type RemoteChange struct {
	Name   string
	Type   string
	Status string // "modified", "added", "deleted"
}

// detectRemoteChanges compares local files with remote entities
func detectRemoteChanges(client *remote.Client, config *remote.SiteConfig, baseDir string) ([]RemoteChange, error) {
	var changes []RemoteChange

	// Fetch current remote entities
	fetcher := remote.NewFetcher(client, config)
	result, err := fetcher.FetchAll()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote entities: %w", err)
	}

	// Build map of local entity hashes
	localHashes := make(map[string]string)
	localEntities := make(map[string]bool)

	err = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}

		// Skip .weg directory
		if strings.HasPrefix(relPath, ".weg") || strings.HasPrefix(relPath, "weg_workspace") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Normalize JSON for comparison (parse and re-marshal)
		var doc map[string]interface{}
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil
		}

		// Remove fields that change on every save
		delete(doc, "modified")
		delete(doc, "modified_by")

		normalized, _ := json.Marshal(doc)
		hash := md5.Sum(normalized)
		localHashes[relPath] = hex.EncodeToString(hash[:])
		localEntities[relPath] = true

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Compare remote entities with local
	for _, entity := range result.Entities {
		// Normalize remote entity
		dataCopy := make(map[string]interface{})
		for k, v := range entity.Data {
			dataCopy[k] = v
		}
		delete(dataCopy, "modified")
		delete(dataCopy, "modified_by")

		normalized, _ := json.Marshal(dataCopy)
		remoteHash := md5.Sum(normalized)
		remoteHashStr := hex.EncodeToString(remoteHash[:])

		localHash, exists := localHashes[entity.FilePath]
		if !exists {
			// New entity on remote
			changes = append(changes, RemoteChange{
				Name:   entity.Name,
				Type:   string(entity.Type),
				Status: "added",
			})
		} else if localHash != remoteHashStr {
			// Modified on remote
			changes = append(changes, RemoteChange{
				Name:   entity.Name,
				Type:   string(entity.Type),
				Status: "modified",
			})
		}

		// Mark as seen
		delete(localEntities, entity.FilePath)
	}

	// Remaining local entities might be deleted on remote
	// (but could also be local additions - skip for now to avoid false positives)

	return changes, nil
}
