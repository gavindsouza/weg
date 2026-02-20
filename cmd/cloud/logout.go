package cloud

import (
	"os"
	"path/filepath"

	"github.com/gavindsouza/weg/internal/cloud"
	wegerrors "github.com/gavindsouza/weg/internal/errors"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout [cloud-name]",
	Short: "Log out from Frappe Cloud",
	Long: `Remove stored Frappe Cloud credentials.

Without arguments, removes credentials for the default cloud.
With a cloud name, removes credentials for that specific cloud.

Examples:
  weg cloud logout           # Logout from default cloud
  weg cloud logout mycloud   # Logout from specific cloud
  weg cloud logout --global  # Remove global credentials
  weg cloud logout --all     # Remove all cloud credentials`,
	RunE: runLogout,
	Args: cobra.MaximumNArgs(1),
}

var (
	logoutGlobal bool
	logoutAll    bool
)

func init() {
	logoutCmd.Flags().BoolVar(&logoutGlobal, "global", false, "Remove global credentials")
	logoutCmd.Flags().BoolVar(&logoutAll, "all", false, "Remove all cloud credentials")
}

func runLogout(cmd *cobra.Command, args []string) error {
	if logoutAll {
		// Remove all credentials
		return removeAllCredentials(logoutGlobal)
	}

	// Determine which cloud to logout from
	cloudName := ""
	if len(args) > 0 {
		cloudName = args[0]
	}

	// Load existing config and credentials
	config, _ := cloud.LoadConfig()
	creds, _ := cloud.LoadCredentials()

	if cloudName == "" && config != nil {
		cloudName = config.Default
	}
	if cloudName == "" {
		cloudName = "frappe"
	}

	// Check if we have credentials for this cloud
	if creds == nil || creds.Clouds == nil || creds.Clouds[cloudName] == nil {
		return wegerrors.NotFound("cloud login", cloudName)
	}

	// Remove from credentials
	delete(creds.Clouds, cloudName)

	// Remove from config
	if config != nil && config.Clouds != nil {
		delete(config.Clouds, cloudName)
		if config.Default == cloudName {
			config.Default = ""
		}
	}

	// Save updated files
	if err := cloud.SaveCredentials(creds, logoutGlobal); err != nil {
		return wegerrors.Config("credentials", "write", err)
	}
	if config != nil {
		if err := cloud.SaveConfig(config, logoutGlobal); err != nil {
			return wegerrors.Config("cloud config", "write", err)
		}
	}

	scope := "local"
	if logoutGlobal {
		scope = "global"
	}
	output.Printf("Logged out from cloud '%s' (%s)", cloudName, scope)
	return nil
}

func removeAllCredentials(global bool) error {
	_, localConfig, _, localCreds, err := cloud.ConfigPaths()
	if err != nil {
		return err
	}

	globalDir, err := cloud.GlobalConfigDir()
	if err != nil {
		return err
	}
	globalConfig := filepath.Join(globalDir, "config.toml")
	globalCreds := filepath.Join(globalDir, "credentials.toml")

	var removed []string

	if global {
		if err := os.Remove(globalConfig); err == nil {
			removed = append(removed, globalConfig)
		}
		if err := os.Remove(globalCreds); err == nil {
			removed = append(removed, globalCreds)
		}
	} else {
		if err := os.Remove(localConfig); err == nil {
			removed = append(removed, localConfig)
		}
		if err := os.Remove(localCreds); err == nil {
			removed = append(removed, localCreds)
		}
	}

	if len(removed) == 0 {
		output.Print("No credentials found to remove")
	} else {
		output.Print("Removed credentials:")
		for _, f := range removed {
			output.Printf("  %s", f)
		}
	}
	return nil
}
