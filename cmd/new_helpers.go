package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// toModuleName converts app-name to app_name (Python module name)
func toModuleName(name string) string {
	// Replace hyphens with underscores
	name = strings.ReplaceAll(name, "-", "_")
	// Remove any non-alphanumeric characters except underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = reg.ReplaceAllString(name, "")
	// Ensure it doesn't start with a number
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	return strings.ToLower(name)
}

// toTitle converts app-name to App Name
func toTitle(name string) string {
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// getGitUser gets the git user name from config
func getGitUser() string {
	// Try to get from git config
	if home, err := os.UserHomeDir(); err == nil {
		gitconfig := filepath.Join(home, ".gitconfig")
		if data, err := os.ReadFile(gitconfig); err == nil {
			lines := strings.Split(string(data), "\n")
			inUser := false
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "[user]" {
					inUser = true
					continue
				}
				if strings.HasPrefix(line, "[") {
					inUser = false
					continue
				}
				if inUser && strings.HasPrefix(line, "name") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}
	return ""
}

// getGitEmail gets the git email from config
func getGitEmail() string {
	if home, err := os.UserHomeDir(); err == nil {
		gitconfig := filepath.Join(home, ".gitconfig")
		if data, err := os.ReadFile(gitconfig); err == nil {
			lines := strings.Split(string(data), "\n")
			inUser := false
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "[user]" {
					inUser = true
					continue
				}
				if strings.HasPrefix(line, "[") {
					inUser = false
					continue
				}
				if inUser && strings.HasPrefix(line, "email") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}
	return ""
}

// runCmdInDir runs a command in the given directory
func runCmdInDir(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
