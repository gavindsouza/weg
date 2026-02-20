package site

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/state"
	"github.com/spf13/cobra"
)

var (
	adminPassword string
	dbRootPass    string
	installApps   []string
	setDefault    bool
)

var newCmd = &cobra.Command{
	Use:   "new <site-name>",
	Short: "Create a new site",
	Long: `Create a new Frappe site.

Examples:
  weg site new mysite.localhost
  weg site new mysite.localhost --admin-password secret
  weg site new mysite.localhost --install-app erpnext`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	newCmd.Flags().StringVar(&adminPassword, "admin-password", "", "Administrator password")
	newCmd.Flags().StringVar(&dbRootPass, "db-root-password", "", "Database root password")
	newCmd.Flags().StringSliceVar(&installApps, "install-app", nil, "Apps to install on the site")
	newCmd.Flags().BoolVar(&setDefault, "set-default", false, "Set as default site")
}

func runNew(cmd *cobra.Command, args []string) error {
	path := "."
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	result, err := config.DetectProjectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	siteName := args[0]

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = result.BenchPath
	case config.ContextWegApp:
		benchPath = result.BenchPath
	default:
		return wegerrors.NotInProject(absPath)
	}

	sitesDir := filepath.Join(benchPath, "sites")
	sitePath := filepath.Join(sitesDir, siteName)

	// Check if site already exists
	if _, err := os.Stat(sitePath); err == nil {
		return wegerrors.Validation("site", fmt.Sprintf("%s already exists", siteName))
	}

	// Get admin password if not provided
	if adminPassword == "" {
		fmt.Print("Administrator password: ")
		reader := bufio.NewReader(os.Stdin)
		adminPassword, _ = reader.ReadString('\n')
		adminPassword = strings.TrimSpace(adminPassword)
		if adminPassword == "" {
			adminPassword = "admin" // Default for dev
		}
	}

	output.Infof("Creating site %s...", siteName)

	// Build bench new-site command
	cmdArgs := []string{"new-site", siteName, "--admin-password", adminPassword}

	if dbRootPass != "" {
		cmdArgs = append(cmdArgs, "--db-root-password", dbRootPass)
	}

	// Run bench new-site
	benchCmd := exec.Command("bench", cmdArgs...)
	benchCmd.Dir = benchPath
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	if err := benchCmd.Run(); err != nil {
		return fmt.Errorf("failed to create site: %w", err)
	}

	// Install apps
	for _, appName := range installApps {
		output.Infof("Installing %s on %s...", appName, siteName)

		installCmd := exec.Command("bench", "--site", siteName, "install-app", appName)
		installCmd.Dir = benchPath
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		if err := installCmd.Run(); err != nil {
			output.Warningf("failed to install %s: %v", appName, err)
		}
	}

	// Update state
	st, err := state.Load(absPath)
	if err != nil {
		st = state.NewState()
	}

	st.AddSite(state.SiteState{
		Name:        siteName,
		Apps:        installApps,
		DefaultSite: setDefault,
	})

	if setDefault {
		st.SetDefaultSite(siteName)
	}

	if err := st.Save(absPath); err != nil {
		output.Warningf("Failed to save state: %v", err)
	}

	output.Successf("Successfully created site %s", siteName)

	if setDefault {
		output.Printf("Set %s as default site", siteName)
	}

	return nil
}
