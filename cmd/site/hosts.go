package site

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var hostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "Manage /etc/hosts entries for sites",
	Long: `Add or remove site entries from /etc/hosts.

This allows accessing sites via their hostname (e.g., mysite.localhost)
instead of having to use localhost:8000.

Requires sudo/root access to modify /etc/hosts.

Examples:
  weg site hosts add                    # Add all sites to /etc/hosts
  weg site hosts add mysite.localhost   # Add specific site
  weg site hosts remove                 # Remove all sites from /etc/hosts
  weg site hosts list                   # Show current site entries`,
}

var hostsAddCmd = &cobra.Command{
	Use:   "add [site]",
	Short: "Add site(s) to /etc/hosts",
	Long: `Add site entries to /etc/hosts.

If no site is specified, adds all sites in the current project.
Requires sudo/root access.

Examples:
  weg site hosts add                    # Add all sites
  weg site hosts add mysite.localhost   # Add specific site`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runHostsAdd,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

var hostsRemoveCmd = &cobra.Command{
	Use:   "remove [site]",
	Short: "Remove site(s) from /etc/hosts",
	Long: `Remove site entries from /etc/hosts.

If no site is specified, removes all sites in the current project.
Requires sudo/root access.

Examples:
  weg site hosts remove                    # Remove all sites
  weg site hosts remove mysite.localhost   # Remove specific site`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runHostsRemove,
	ValidArgsFunction: completion.CompleteSiteNamesForArg(0),
}

var hostsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List site entries in /etc/hosts",
	Long: `List all site-related entries in /etc/hosts.

Shows entries for sites managed by weg and any *.localhost entries.

Examples:
  weg site hosts list`,
	RunE: runHostsList,
}

func init() {
	SiteCmd.AddCommand(hostsCmd)
	hostsCmd.AddCommand(hostsAddCmd)
	hostsCmd.AddCommand(hostsRemoveCmd)
	hostsCmd.AddCommand(hostsListCmd)
}

func runHostsAdd(cmd *cobra.Command, args []string) error {
	sites, err := getSitesToManage(args)
	if err != nil {
		return err
	}

	if len(sites) == 0 {
		return fmt.Errorf("no sites found")
	}

	// Read current hosts file
	hostsPath := "/etc/hosts"
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w (try with sudo)", hostsPath, err)
	}

	lines := strings.Split(string(content), "\n")
	existingHosts := make(map[string]bool)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && !strings.HasPrefix(line, "#") {
			for _, host := range fields[1:] {
				existingHosts[host] = true
			}
		}
	}

	// Add missing sites
	var toAdd []string
	for _, site := range sites {
		if !existingHosts[site] {
			toAdd = append(toAdd, site)
		}
	}

	if len(toAdd) == 0 {
		fmt.Println("All sites already in /etc/hosts")
		return nil
	}

	// Append new entries
	newContent := string(content)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += "\n# Added by weg\n"
	for _, site := range toAdd {
		newContent += fmt.Sprintf("127.0.0.1\t%s\n", site)
	}

	// Write back
	if err := os.WriteFile(hostsPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w (try with sudo)", hostsPath, err)
	}

	for _, site := range toAdd {
		fmt.Printf("Added: %s\n", site)
	}
	return nil
}

func runHostsRemove(cmd *cobra.Command, args []string) error {
	sites, err := getSitesToManage(args)
	if err != nil {
		return err
	}

	sitesMap := make(map[string]bool)
	for _, s := range sites {
		sitesMap[s] = true
	}

	// Read current hosts file
	hostsPath := "/etc/hosts"
	file, err := os.Open(hostsPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", hostsPath, err)
	}
	defer file.Close()

	var newLines []string
	var removed []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// Check if this line contains any of our sites
		shouldRemove := false
		if len(fields) >= 2 && !strings.HasPrefix(line, "#") {
			for _, host := range fields[1:] {
				if sitesMap[host] {
					shouldRemove = true
					removed = append(removed, host)
					break
				}
			}
		}

		if !shouldRemove {
			newLines = append(newLines, line)
		}
	}

	if len(removed) == 0 {
		fmt.Println("No matching sites found in /etc/hosts")
		return nil
	}

	// Write back
	newContent := strings.Join(newLines, "\n")
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	if err := os.WriteFile(hostsPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w (try with sudo)", hostsPath, err)
	}

	for _, site := range removed {
		fmt.Printf("Removed: %s\n", site)
	}
	return nil
}

func runHostsList(cmd *cobra.Command, args []string) error {
	sites, err := getSitesToManage(nil)
	if err != nil {
		// Just list all .localhost entries
		sites = nil
	}

	sitesMap := make(map[string]bool)
	for _, s := range sites {
		sitesMap[s] = true
	}

	// Read hosts file
	hostsPath := "/etc/hosts"
	file, err := os.Open(hostsPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", hostsPath, err)
	}
	defer file.Close()

	fmt.Println("Site entries in /etc/hosts:")
	fmt.Println()

	found := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) >= 2 && !strings.HasPrefix(line, "#") {
			for _, host := range fields[1:] {
				// Show if it's one of our sites or ends in .localhost
				if sitesMap[host] || strings.HasSuffix(host, ".localhost") {
					fmt.Printf("  %s -> %s\n", host, fields[0])
					found = true
				}
			}
		}
	}

	if !found {
		fmt.Println("  (no site entries found)")
	}

	return nil
}

func getSitesToManage(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	// Get sites from current project
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return nil, err
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return nil, wegerrors.NotInProject(absPath)
	}

	st, err := state.Load(absPath)
	if err != nil {
		// Try reading sites directory
		sitesDir := filepath.Join(benchPath, "sites")
		entries, _ := os.ReadDir(sitesDir)
		var sites []string
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "assets" {
				sites = append(sites, e.Name())
			}
		}
		return sites, nil
	}

	return st.SiteNames(), nil
}
