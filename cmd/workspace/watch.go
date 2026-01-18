/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package workspace

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gavindsouza/weg/internal/workspace"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch workspace and auto-collapse on changes",
	Long: `Watch the weg_workspace/ directory and automatically collapse changes.

When a file in weg_workspace/ is saved, it will automatically be packed
back into the corresponding JSON file.

Press Ctrl+C to stop watching.

Examples:
  weg workspace watch     # Watch and auto-collapse on save`,
	RunE: runWatch,
}

func init() {
	WorkspaceCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if we're in a weg clone
	if _, err := os.Stat(".weg"); os.IsNotExist(err) {
		return fmt.Errorf("not a weg remote clone (no .weg directory)")
	}

	// Check if workspace exists
	workspaceDir := filepath.Join(cwd, workspace.WorkspaceDir)
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		return fmt.Errorf("no workspace found (run 'weg workspace expand' first)")
	}

	// Create watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Add workspace directory and all subdirectories
	err = filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	fmt.Printf("Watching %s/ for changes...\n", workspace.WorkspaceDir)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Debounce timer to avoid multiple collapses for rapid saves
	var debounceTimer *time.Timer
	debounceDelay := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only watch for writes to files (not directories)
			if event.Op&fsnotify.Write == fsnotify.Write {
				// Skip non-code files
				if !isCodeFile(event.Name) {
					continue
				}

				// Debounce: reset timer on each event
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDelay, func() {
					relPath, _ := filepath.Rel(cwd, event.Name)
					fmt.Printf("Changed: %s\n", relPath)

					// Run collapse
					result, err := workspace.Collapse(workspace.CollapseOptions{
						BaseDir: cwd,
					})

					if err != nil {
						fmt.Printf("  Error: %v\n", err)
					} else if len(result.Updated) > 0 {
						for _, f := range result.Updated {
							fmt.Printf("  ✓ Collapsed: %s\n", f)
						}
					} else if len(result.Unchanged) > 0 {
						fmt.Println("  (no changes to collapse)")
					}
					fmt.Println()
				})
			}

			// Watch new directories
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("Watch error: %v\n", err)

		case <-sigChan:
			fmt.Println("\nStopping watch...")
			return nil
		}
	}
}

// isCodeFile checks if a file is a code file we should watch
func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".py", ".js", ".sql", ".html", ".css":
		return true
	default:
		return false
	}
}
