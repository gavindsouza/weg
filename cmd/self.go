package cmd

import (
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage weg itself and its dependencies",
}

var installCmd = &cobra.Command{
	Use:   "install-tools",
	Short: "Install required tools (devbox, direnv, etc)",
	Run: func(cmd *cobra.Command, args []string) {
		tools.EnsureToolsInstalled()
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system setup and environment compatibility",
	Run: func(cmd *cobra.Command, args []string) {
		// tools.RunDoctor()
	},
}

var selfUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update weg to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		// tools.UpdateWeg()
	},
}

func init() {
	selfCmd.AddCommand(installCmd)
	selfCmd.AddCommand(doctorCmd)
	selfCmd.AddCommand(selfUpdateCmd)
	rootCmd.AddCommand(selfCmd)
}
