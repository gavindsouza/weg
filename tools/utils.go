package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// DebugLog prints debug messages only when WEG_NO_PROGRESS is set
func DebugLog(format string, args ...interface{}) {
	if os.Getenv("WEG_NO_PROGRESS") != "" {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

func RunCmdWithError(binary, dir string, args ...string) error {
	DebugLog("Running command: %s %v in %s", binary, args, dir)
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir

	// For bench commands, capture stderr for debugging
	if binary == "bench" {
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
		}
		DebugLog("Command completed successfully: %s %v", binary, args)
		return nil
	}

	// For other commands, suppress output only if progress bars are enabled
	if os.Getenv("WEG_NO_PROGRESS") == "" {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	DebugLog("Command completed successfully: %s %v", binary, args)
	return nil
}

// RunAsync runs a command asynchronously and handles errors
func RunAsync(wg *sync.WaitGroup, errChan chan<- error, pm *ProgressManager, binary, dir string, args []string, progressBar string) {
	DebugLog("Starting async command: %s %v in %s", binary, args, dir)
	wg.Add(1)
	go func() {
		defer func() {
			DebugLog("Async command completed: %s %v", binary, args)
			wg.Done()
		}()

		if err := RunCmdWithError(binary, dir, args...); err != nil {
			DebugLog("Async command failed: %s %v: %v", binary, args, err)
			errChan <- fmt.Errorf("failed to run %s: %w", binary, err)
			return
		}

		if progressBar != "" {
			DebugLog("Incrementing progress bar: %s", progressBar)
			pm.Increment(progressBar)
		}
		errChan <- nil
	}()
}

// WaitForErrors waits for all goroutines to complete and checks for errors
func WaitForErrors(wg *sync.WaitGroup, errChan chan error) error {
	DebugLog("Starting WaitForErrors")
	wg.Wait()
	DebugLog("All goroutines completed")
	close(errChan)
	DebugLog("Error channel closed")

	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		DebugLog("Found %d errors", len(errors))
		for _, err := range errors {
			DebugLog("Error: %v", err)
		}
		return fmt.Errorf("encountered %d errors: %v", len(errors), errors)
	}

	DebugLog("No errors found")
	return nil
}

func extractAppName(url string) string {
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Handle SSH URLs (git@github.com:user/repo.git)
	if strings.Contains(url, "@") {
		parts := strings.Split(url, ":")
		if len(parts) > 1 {
			url = parts[1]
		}
	}

	// Handle HTTP/HTTPS URLs (https://github.com/user/repo.git)
	if strings.Contains(url, "://") {
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			url = parts[len(parts)-1]
		}
	}

	// Handle Azure DevOps URLs (https://dev.azure.com/org/project/_git/repo)
	if strings.Contains(url, "dev.azure.com") {
		parts := strings.Split(url, "/_git/")
		if len(parts) > 1 {
			url = parts[1]
		}
	}

	return url
}

func ParseAppsJSON(jsonStr string) []FrappeApp {
	config := []FrappeApp{}

	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return config
	}

	apps := make([]FrappeApp, len(config))
	for i, app := range config {
		apps[i] = FrappeApp{
			Url:    app.Url,
			Branch: app.Branch,
			Name: func() string {
				if app.Name != "" {
					return app.Name
				}
				return extractAppName(app.Url)
			}(),
		}
	}
	return apps
}
