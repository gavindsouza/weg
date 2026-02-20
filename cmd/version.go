package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

// Version information - set during build with ldflags
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Show version information for weg and installed apps.

Examples:
  weg version           # Show weg version
  weg version --apps    # Also show installed app versions`,
	RunE: runVersionCmd,
}

var showApps bool

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVar(&showApps, "apps", false, "Show installed app versions")
}

func runVersionCmd(cmd *cobra.Command, args []string) error {
	fmt.Printf("weg %s\n", Version)
	if Commit != "unknown" {
		fmt.Printf("  commit: %s\n", Commit)
	}
	if BuildDate != "unknown" {
		fmt.Printf("  built:  %s\n", BuildDate)
	}

	if !showApps {
		return nil
	}

	// Try to show app versions
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return nil
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return nil
	}

	// Load state
	st, err := state.Load(absPath)
	if err != nil || st.IsEmpty() {
		return nil
	}

	fmt.Println()
	fmt.Println("Installed apps:")
	for name, app := range st.Apps {
		version := getAppVersion(benchPath, name)
		if version != "" {
			fmt.Printf("  %s: %s\n", name, version)
		} else if app.Branch != "" {
			fmt.Printf("  %s: %s\n", name, app.Branch)
		} else {
			fmt.Printf("  %s: (unknown)\n", name)
		}
	}

	return nil
}

func getAppVersion(benchPath, appName string) string {
	// Try to read from hooks.py or __init__.py
	appDir := filepath.Join(benchPath, "apps", appName, appName)

	// Check __init__.py for __version__
	initPath := filepath.Join(appDir, "__init__.py")
	if data, err := os.ReadFile(initPath); err == nil {
		// Simple parsing - look for __version__ = "x.x.x"
		content := string(data)
		for _, line := range []string{
			`__version__ = "`,
			`__version__ = '`,
			`__version__="`,
			`__version__='`,
		} {
			if idx := strings.Index(content, line); idx != -1 {
				start := idx + len(line)
				end := start
				for end < len(content) && content[end] != '"' && content[end] != '\'' {
					end++
				}
				if end > start {
					return content[start:end]
				}
			}
		}
	}

	// Try package.json for JS version
	pkgPath := filepath.Join(benchPath, "apps", appName, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Version != "" {
			return pkg.Version
		}
	}

	return ""
}
