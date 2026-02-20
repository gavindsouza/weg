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

	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
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
		return wegerrors.NotFound("remote clone", ".weg/site.toml")
	}

	// Load config
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return wegerrors.Config("site.toml", "read", err)
	}

	output.Printf("Remote site: %s", config.Site.URL)
	output.Printf("Last sync:   %s", config.Sync.LastSync.Format("2006-01-02 15:04:05"))
	output.Print("")

	// Check git status for local changes
	gitStatus := exec.Command("git", "status", "--porcelain")
	gitOut, err := gitStatus.Output()
	if err != nil {
		return fmt.Errorf("failed to get git status: %w", err)
	}

	localChanges := strings.TrimSpace(string(gitOut))
	if localChanges == "" {
		output.Print("Local:  No changes")
	} else {
		lines := strings.Split(localChanges, "\n")
		output.Printf("Local:  %d file(s) changed", len(lines))
		for _, line := range lines {
			if len(line) > 3 {
				status := line[:2]
				file := line[3:]
				switch {
				case strings.HasPrefix(status, "M"):
					output.Printf("  modified:   %s", file)
				case strings.HasPrefix(status, "A"):
					output.Printf("  added:      %s", file)
				case strings.HasPrefix(status, "D"):
					output.Printf("  deleted:    %s", file)
				case strings.HasPrefix(status, "?"):
					output.Printf("  untracked:  %s", file)
				default:
					output.Printf("  %s %s", status, file)
				}
			}
		}
	}

	// Check for unpushed commits (only if upstream is configured)
	gitLog := exec.Command("git", "log", "--oneline", "@{u}..", "--")
	unpushed, err := gitLog.Output()
	if err == nil && len(strings.TrimSpace(string(unpushed))) > 0 {
		lines := strings.Split(strings.TrimSpace(string(unpushed)), "\n")
		output.Printf("\nUnpushed commits: %d", len(lines))
		for _, line := range lines {
			output.Printf("  %s", line)
		}
	}

	// Check remote if requested
	if statusRemote {
		output.Print("")
		output.Print("Checking remote...")

		creds, err := remote.LoadCredentials(".")
		if err != nil {
			return wegerrors.Config("credentials", "read", err)
		}

		client := remote.NewClientFromConfig(config, creds)
		if err := client.Ping(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		output.Print("Connected")

		// Fetch remote entities and compare with local
		remoteChanges, err := detectRemoteChanges(client, config, ".")
		if err != nil {
			output.Warningf("Could not detect remote changes: %v", err)
		} else if len(remoteChanges) == 0 {
			output.Print("Remote: No changes detected")
		} else {
			output.Printf("Remote: %d entity change(s) detected", len(remoteChanges))
			for _, change := range remoteChanges {
				output.Printf("  %s: %s", change.Status, change.Name)
			}
			output.Print("\nRun 'weg remote pull' to fetch remote changes.")
		}
	}

	// Show sync instructions
	if localChanges != "" {
		output.Print("")
		output.Print("To sync changes:")
		output.Print("  weg sync -m \"description\"")
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
		var doc map[string]any
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
		dataCopy := make(map[string]any)
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
