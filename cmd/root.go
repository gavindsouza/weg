/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package cmd

import (
	"fmt"
	"os"

	internalconfig "github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"

	"github.com/gavindsouza/weg/cmd/api"
	"github.com/gavindsouza/weg/cmd/app"
	"github.com/gavindsouza/weg/cmd/build"
	"github.com/gavindsouza/weg/cmd/cache"
	"github.com/gavindsouza/weg/cmd/cloud"
	"github.com/gavindsouza/weg/cmd/config"
	"github.com/gavindsouza/weg/cmd/db"
	"github.com/gavindsouza/weg/cmd/doc"
	"github.com/gavindsouza/weg/cmd/docker"
	"github.com/gavindsouza/weg/cmd/doctype"
	"github.com/gavindsouza/weg/cmd/fixtures"
	"github.com/gavindsouza/weg/cmd/image"
	"github.com/gavindsouza/weg/cmd/log"
	"github.com/gavindsouza/weg/cmd/remote"
	"github.com/gavindsouza/weg/cmd/scheduler"
	"github.com/gavindsouza/weg/cmd/site"
	"github.com/gavindsouza/weg/cmd/user"
	"github.com/gavindsouza/weg/cmd/workspace"
	"github.com/spf13/cobra"
)

// Global flags
var (
	verboseCount    int    // Track -v flags (supports -v, -vv, -vvv)
	logLevel        string // --log-level flag
	quiet           bool
	yes             bool
	configPath      string
	chdir           string
	outputFormat    string // --output flag
	debugCategories string // --debug-categories flag

	// Backward compatibility: verbose is derived from verboseCount
	verbose bool
)

// Detected paths (set in PersistentPreRunE)
var (
	projectRoot string // The detected project root (or empty if not in a project)
	originalDir string // The directory weg was invoked from
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
		// Configure output package first
		if err := configureOutput(); err != nil {
			return err
		}

		// Store original directory
		var err error
		originalDir, err = os.Getwd()
		if err != nil {
			originalDir = "."
		}

		// Handle explicit --chdir flag first
		if chdir != "" {
			if err := os.Chdir(chdir); err != nil {
				return fmt.Errorf("failed to change directory to %s: %w", chdir, err)
			}
		}

		// Commands that should work without being in a project
		skipAutoChdir := map[string]bool{
			"new":        true,
			"create":     true,
			"init":       true,
			"help":       true,
			"version":    true,
			"completion": true,
			"self":       true,
			"run":        true, // weg run clones fresh
			"clone":      true, // weg remote clone works outside projects
			"remote":     true, // weg remote subcommands
			"workspace":  true, // weg workspace works in remote clones
		}

		// Skip auto-detection for root command (no subcommand) or skipped commands
		cmdName := cmd.Name()
		if cmdName == "weg" || skipAutoChdir[cmdName] {
			return nil
		}

		// Find project root by walking up the directory tree
		cwd, _ := os.Getwd()
		if root, found := internalconfig.FindBenchRoot(cwd); found {
			projectRoot = root
			// Only chdir if we're not already at the root
			if cwd != root {
				if err := os.Chdir(root); err != nil {
					return fmt.Errorf("failed to change to project root %s: %w", root, err)
				}
				if verbose {
					fmt.Printf("Changed to project root: %s\n", root)
				}
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
	rootCmd.PersistentFlags().CountVarP(&verboseCount, "verbose", "v", "Increase verbosity (-v, -vv, -vvv)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Set log level: quiet, normal, verbose, debug, trace")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "Assume yes for all prompts")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: auto-detect)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "auto", "Output format: auto, json, table, plain, quiet")
	rootCmd.PersistentFlags().StringVar(&debugCategories, "debug-categories", "", "Filter debug output: all,config,state,net,git,fs,exec")

	// Mark quiet and log-level as mutually exclusive with each other
	// (verbose count is handled differently)

	// Add subcommand groups
	rootCmd.AddCommand(api.ApiCmd)
	rootCmd.AddCommand(app.AppCmd)
	// Note: benchCmd is registered in cmd/bench.go (passthrough to bench CLI)
	rootCmd.AddCommand(build.BuildCmd)
	rootCmd.AddCommand(cache.CacheCmd)
	rootCmd.AddCommand(cloud.CloudCmd)
	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(db.DbCmd)
	rootCmd.AddCommand(doc.DocCmd)
	rootCmd.AddCommand(docker.DockerCmd)
	rootCmd.AddCommand(doctype.DoctypeCmd)
	rootCmd.AddCommand(fixtures.FixturesCmd)
	rootCmd.AddCommand(image.ImageCmd)
	rootCmd.AddCommand(log.LogCmd)
	rootCmd.AddCommand(remote.RemoteCmd)
	rootCmd.AddCommand(scheduler.SchedulerCmd)
	rootCmd.AddCommand(site.SiteCmd)
	rootCmd.AddCommand(user.UserCmd)
	rootCmd.AddCommand(workspace.WorkspaceCmd)
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	// Config initialization happens here
	// Will be expanded as needed
}

// configureOutput sets up the output package based on flags and environment.
func configureOutput() error {
	// Parse output format
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return err
	}
	output.CurrentFormat = format

	// Determine verbosity level
	// Precedence: --log-level > -q > -v count > env var
	if logLevel != "" {
		level, err := output.ParseVerbosity(logLevel)
		if err != nil {
			return err
		}
		output.Level = level
	} else if quiet {
		output.Level = output.VerbosityQuiet
	} else if verboseCount >= 3 {
		output.Level = output.VerbosityTrace
	} else if verboseCount >= 2 {
		output.Level = output.VerbosityDebug
	} else if verboseCount >= 1 {
		output.Level = output.VerbosityVerbose
	} else {
		output.Level = output.VerbosityNormal
	}

	// Parse debug categories
	if debugCategories != "" {
		output.ParseDebugCategories(debugCategories)
	}

	// Load from environment variables (lowest precedence, won't override flags)
	output.LoadFromEnv()

	// Set backward-compatible verbose flag
	verbose = output.Level >= output.VerbosityVerbose

	return nil
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

// GetProjectRoot returns the detected project root (may be empty if not in a project)
func GetProjectRoot() string {
	return projectRoot
}

// GetOriginalDir returns the directory weg was invoked from
func GetOriginalDir() string {
	return originalDir
}
