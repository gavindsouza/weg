package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
