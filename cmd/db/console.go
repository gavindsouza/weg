package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var consoleCmd = &cobra.Command{
	Use:     "console [site]",
	Aliases: []string{"shell"},
	Short:   "Open database shell",
	Long: `Open an interactive database shell (MariaDB/MySQL/PostgreSQL).

Examples:
  weg db console                    # Default site
  weg db console test.localhost     # Specific site`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConsole,
}

func init() {
	DbCmd.AddCommand(consoleCmd)
}

func runConsole(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
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

	var site string
	if len(args) > 0 {
		site = args[0]
	} else {
		st, err := state.Load(absPath)
		if err == nil {
			site = st.GetDefaultSite()
		}
		if site == "" {
			currentSitePath := filepath.Join(benchPath, "sites", "currentsite.txt")
			data, _ := os.ReadFile(currentSitePath)
			site = strings.TrimSpace(string(data))
		}
	}

	if site == "" {
		return fmt.Errorf("no site specified and no default site found")
	}

	return runBenchHelper(benchPath, []string{"frappe", "--site", site, "db-console"})
}
