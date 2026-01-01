package site

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all sites",
	Long: `List all sites in the current project.

Shows sites from both configuration and actual directories.

Examples:
  weg site list
  weg site ls`,
	RunE: runList,
}

func runList(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath, sitesDir string
	var configuredSites []config.SiteConfig

	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
		sitesDir = filepath.Join(benchPath, "sites")
		benchConfig, err := config.ParseWegToml(absPath)
		if err != nil {
			return fmt.Errorf("failed to parse weg.toml: %w", err)
		}
		configuredSites = benchConfig.Sites

	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
		sitesDir = filepath.Join(benchPath, "sites")
		// App-centric mode may not have configured sites yet
		configuredSites = []config.SiteConfig{}

	default:
		return fmt.Errorf("not a weg-managed project")
	}

	// Load state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	// Get default site
	defaultSite := st.GetDefaultSite()

	// Scan actual sites directory
	actualSites := make(map[string]bool)
	if entries, err := os.ReadDir(sitesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "assets" {
				sitePath := filepath.Join(sitesDir, entry.Name(), "site_config.json")
				if _, err := os.Stat(sitePath); err == nil {
					actualSites[entry.Name()] = true
				}
			}
		}
	}

	// Build combined list
	allSites := make(map[string]siteInfo)

	for _, site := range configuredSites {
		allSites[site.Name] = siteInfo{
			Name:       site.Name,
			Configured: true,
			Default:    site.DefaultSite || site.Name == defaultSite,
			Apps:       site.Apps,
		}
	}

	for name := range actualSites {
		if info, ok := allSites[name]; ok {
			info.Exists = true
			allSites[name] = info
		} else {
			allSites[name] = siteInfo{
				Name:   name,
				Exists: true,
			}
		}
	}

	for name, siteState := range st.Sites {
		if info, ok := allSites[name]; ok {
			info.InState = true
			info.Apps = siteState.Apps
			allSites[name] = info
		}
	}

	if len(allSites) == 0 {
		fmt.Println("No sites found.")
		fmt.Println("Create one with: weg site new <name>")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tAPPS")

	for _, info := range allSites {
		status := ""
		if info.Default {
			status = "* "
		}

		if info.Exists {
			status += "active"
		} else if info.Configured {
			status += "configured"
		} else {
			status += "unknown"
		}

		apps := ""
		if len(info.Apps) > 0 {
			for i, a := range info.Apps {
				if i > 0 {
					apps += ", "
				}
				apps += a
				if i >= 2 && len(info.Apps) > 3 {
					apps += fmt.Sprintf(" (+%d more)", len(info.Apps)-3)
					break
				}
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", info.Name, status, apps)
	}

	w.Flush()

	if defaultSite != "" {
		fmt.Printf("\n* = default site\n")
	}

	return nil
}

type siteInfo struct {
	Name       string
	Configured bool
	Exists     bool
	InState    bool
	Default    bool
	Apps       []string
}
