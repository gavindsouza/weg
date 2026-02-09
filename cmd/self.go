package cmd

import (
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage weg itself and its dependencies",
	Long: `Manage the weg CLI tool and its system-level dependencies.

Examples:
  weg self install-tools   # Install devbox, direnv, etc
  weg self doctor          # Check system compatibility
  weg self update          # Update weg to latest version`,
}

var installCmd = &cobra.Command{
	Use:   "install-tools",
	Short: "Install required tools (devbox, direnv, etc)",
	Long: `Install system tools required by weg.

Installs devbox, direnv, and other dependencies needed to manage
Frappe environments. Safe to run multiple times — already-installed
tools are skipped.

Examples:
  weg self install-tools`,
	Run: func(cmd *cobra.Command, args []string) {
		tools.EnsureToolsInstalled()
	},
}

var selfDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system setup and environment compatibility",
	Long: `Check that system-level prerequisites are installed and configured.

Unlike 'weg doctor' (which checks a project), this checks the host
system for Go, devbox, direnv, and other global requirements.

Examples:
  weg self doctor`,
	Run: func(cmd *cobra.Command, args []string) {
		// tools.RunDoctor()
	},
}

var selfUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update weg to the latest version",
	Long: `Update the weg binary to the latest released version.

Examples:
  weg self update`,
	Run: func(cmd *cobra.Command, args []string) {
		// tools.UpdateWeg()
	},
}

func init() {
	selfCmd.AddCommand(installCmd)
	selfCmd.AddCommand(selfDoctorCmd)
	selfCmd.AddCommand(selfUpdateCmd)
	rootCmd.AddCommand(selfCmd)
}
