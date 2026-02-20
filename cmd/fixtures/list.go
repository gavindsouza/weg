package fixtures

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/completion"
	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list <app>",
	Short: "List fixture files for an app",
	Long: `List all fixture files in an app's fixtures directory.

Examples:
  weg fixtures list myapp`,
	Args:              cobra.ExactArgs(1),
	RunE:              runList,
	ValidArgsFunction: completion.CompleteAppNamesForArg(0),
}

func init() {
	FixturesCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	appName := args[0]

	// Detect context to find bench path
	absPath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return wegerrors.NotInProject(absPath)
	}

	// Verify app exists
	appPath := filepath.Join(benchPath, "apps", appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return fmt.Errorf("app %s not found", appName)
	}

	fixturesPath := filepath.Join(appPath, appName, "fixtures")
	if _, err := os.Stat(fixturesPath); os.IsNotExist(err) {
		fmt.Printf("No fixtures directory found for %s\n", appName)
		fmt.Printf("Expected location: %s\n", fixturesPath)
		return nil
	}

	entries, err := os.ReadDir(fixturesPath)
	if err != nil {
		return fmt.Errorf("failed to read fixtures directory: %w", err)
	}

	var jsonFiles []os.DirEntry
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e)
		}
	}

	if len(jsonFiles) == 0 {
		fmt.Printf("No fixture files found in %s\n", fixturesPath)
		return nil
	}

	fmt.Printf("Fixtures for %s (%s):\n\n", appName, fixturesPath)
	fmt.Printf("%-40s %s\n", "FILE", "SIZE")
	fmt.Println(strings.Repeat("-", 55))

	for _, e := range jsonFiles {
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := formatSize(info.Size())
		fmt.Printf("%-40s %s\n", e.Name(), size)
	}

	return nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
