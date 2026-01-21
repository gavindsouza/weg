package site

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
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

	result, err := config.DetectContext(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect context: %w", err)
	}

	siteName := args[0]

	var benchPath string
	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
	default:
		return fmt.Errorf("not a weg-managed project")
	}

	sitesDir := filepath.Join(benchPath, "sites")
	sitePath := filepath.Join(sitesDir, siteName)

	// Check if site already exists
	if _, err := os.Stat(sitePath); err == nil {
		return fmt.Errorf("site %s already exists", siteName)
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

	fmt.Printf("Creating site %s...\n", siteName)

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
		fmt.Printf("Installing %s on %s...\n", appName, siteName)

		installCmd := exec.Command("bench", "--site", siteName, "install-app", appName)
		installCmd.Dir = benchPath
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		if err := installCmd.Run(); err != nil {
			fmt.Printf("Warning: failed to install %s: %v\n", appName, err)
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

	fmt.Printf("Successfully created site %s\n", siteName)

	if setDefault {
		fmt.Printf("Set %s as default site\n", siteName)
	}

	return nil
}
