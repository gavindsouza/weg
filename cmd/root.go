/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/gavindsouza/weg/cmd/api"
	"github.com/gavindsouza/weg/cmd/app"
	"github.com/gavindsouza/weg/cmd/bench"
	"github.com/gavindsouza/weg/cmd/cache"
	"github.com/gavindsouza/weg/cmd/cloud"
	"github.com/gavindsouza/weg/cmd/config"
	"github.com/gavindsouza/weg/cmd/docker"
	"github.com/gavindsouza/weg/cmd/image"
	"github.com/gavindsouza/weg/cmd/site"
	"github.com/spf13/cobra"
)

// Global flags
var (
	verbose    bool
	quiet      bool
	yes        bool
	configPath string
	chdir      string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "weg",
	Short:        "Manage Frappe Deployments",
	SilenceUsage: true,
	Long: `Weg is a modern CLI for managing Frappe development environments.

It provides fast, declarative configuration for Frappe apps and benches,
with support for multiple Frappe versions and databases.

Quick start:
  weg new myapp          Create a new Frappe app
  weg init               Initialize weg in existing project
  weg start              Start development servers
  weg sync               Apply configuration changes

Learn more at https://github.com/gavindsouza/weg`,
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if chdir != "" {
			if err := os.Chdir(chdir); err != nil {
				return fmt.Errorf("failed to change directory to %s: %w", chdir, err)
			}
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global persistent flags
	rootCmd.PersistentFlags().StringVarP(&chdir, "chdir", "C", "", "Run as if weg was started in <path>")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "Assume yes for all prompts")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: auto-detect)")

	// Mark quiet and verbose as mutually exclusive
	rootCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")

	// Add subcommand groups
	rootCmd.AddCommand(api.ApiCmd)
	rootCmd.AddCommand(app.AppCmd)
	rootCmd.AddCommand(bench.BenchCmd)
	rootCmd.AddCommand(cache.CacheCmd)
	rootCmd.AddCommand(cloud.CloudCmd)
	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(docker.DockerCmd)
	rootCmd.AddCommand(image.ImageCmd)
	rootCmd.AddCommand(site.SiteCmd)
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	// Config initialization happens here
	// Will be expanded as needed
}

// IsVerbose returns true if verbose mode is enabled
func IsVerbose() bool {
	return verbose
}

// IsQuiet returns true if quiet mode is enabled
func IsQuiet() bool {
	return quiet
}

// AssumeYes returns true if --yes flag was passed
func AssumeYes() bool {
	return yes
}

// GetConfigPath returns the custom config path if set
func GetConfigPath() string {
	return configPath
}

// PrintVerbose prints a message only in verbose mode
func PrintVerbose(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args...)
	}
}

// PrintInfo prints a message unless quiet mode is enabled
func PrintInfo(format string, args ...interface{}) {
	if !quiet {
		fmt.Printf(format+"\n", args...)
	}
}

// PrintError prints an error message (always shown)
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
