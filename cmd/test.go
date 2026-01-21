package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/tools"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test [test-args...]",
	Short: "Run tests for the current app",
	Long: `Run tests for the current Frappe app.

By default, runs pytest on the current app. Supports multi-version testing
when the app specifies multiple compatible Frappe versions in pyproject.toml.

Examples:
  weg test                     # Run tests with default Frappe version
  weg test --all-versions      # Run tests against all compatible versions
  weg test --version 14        # Run tests with Frappe 14
  weg test --parallel          # Run multi-version tests in parallel
  weg test -- -v -x            # Pass arguments to pytest
  weg test --module myapp.tests.test_api`,
	RunE:               runTest,
	DisableFlagParsing: false,
}

var (
	testAllVersions bool
	testVersion     string
	testParallel    bool
	testModule      string
	testApp         string
	testSite        string
)

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Flags().BoolVar(&testAllVersions, "all-versions", false, "Run tests against all compatible Frappe versions")
	testCmd.Flags().StringVar(&testVersion, "version", "", "Specific Frappe version to test against")
	testCmd.Flags().BoolVar(&testParallel, "parallel", false, "Run multi-version tests in parallel")
	testCmd.Flags().StringVar(&testModule, "module", "", "Specific test module to run")
	testCmd.Flags().StringVar(&testApp, "app", "", "App to test (default: current app)")
	testCmd.Flags().StringVar(&testSite, "site", "", "Site to use for tests")
}

type testResult struct {
	Version string
	Success bool
	Output  string
	Error   error
}

func runTest(cmd *cobra.Command, args []string) error {
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
	var appName string

	switch result.Context {
	case config.ContextWegBench:
		benchPath = absPath
		if testApp == "" {
			return fmt.Errorf("please specify --app when running from a bench")
		}
		appName = testApp
	case config.ContextWegApp:
		benchPath = filepath.Join(absPath, ".weg")
		appName = filepath.Base(absPath)
	case config.ContextApp:
		// App without weg setup
		benchPath = filepath.Join(absPath, ".weg")
		appName = filepath.Base(absPath)
		if _, err := os.Stat(benchPath); os.IsNotExist(err) {
			return fmt.Errorf("no .weg environment found. Run 'weg init' first")
		}
	default:
		return fmt.Errorf("not in a Frappe app or bench directory")
	}

	if testApp != "" {
		appName = testApp
	}

	// Get compatible versions from pyproject.toml
	var compatibleVersions []string
	pyprojectPath := filepath.Join(absPath, "pyproject.toml")
	if appConfig, err := config.ParsePyproject(pyprojectPath); err == nil && appConfig != nil {
		compatibleVersions = appConfig.Compatibility.Frappe
	}

	// Determine which versions to test
	var versionsToTest []string

	if testAllVersions {
		if len(compatibleVersions) == 0 {
			return fmt.Errorf("no compatible versions found in pyproject.toml")
		}
		versionsToTest = compatibleVersions
		PrintInfo("Running tests against %d Frappe versions: %s", len(versionsToTest), strings.Join(versionsToTest, ", "))
	} else if testVersion != "" {
		// Validate version
		if !isValidVersion(testVersion) {
			return fmt.Errorf("invalid version: %s", testVersion)
		}
		versionsToTest = []string{testVersion}
	} else {
		// Use current environment version
		versionsToTest = []string{""}
	}

	// Determine site
	site := testSite
	if site == "" {
		site = fmt.Sprintf("%s.localhost", appName)
	}

	// Build test arguments
	testArgs := args
	if testModule != "" {
		testArgs = append([]string{testModule}, testArgs...)
	}

	// Single version test
	if len(versionsToTest) == 1 {
		return runSingleTest(benchPath, appName, site, versionsToTest[0], testArgs)
	}

	// Multi-version testing
	results := make([]testResult, len(versionsToTest))

	if testParallel {
		PrintInfo("Running tests in parallel...")
		var wg sync.WaitGroup
		for i, version := range versionsToTest {
			wg.Add(1)
			go func(idx int, ver string) {
				defer wg.Done()
				results[idx] = runVersionTest(benchPath, appName, site, ver, testArgs)
			}(i, version)
		}
		wg.Wait()
	} else {
		for i, version := range versionsToTest {
			results[i] = runVersionTest(benchPath, appName, site, version, testArgs)
		}
	}

	// Print summary
	PrintInfo("")
	PrintInfo("Test Results Summary")
	PrintInfo("====================")

	allPassed := true
	for _, r := range results {
		status := "PASS"
		if !r.Success {
			status = "FAIL"
			allPassed = false
		}
		PrintInfo("Frappe %s: %s", r.Version, status)
		if r.Error != nil {
			PrintInfo("  Error: %v", r.Error)
		}
	}

	if !allPassed {
		return fmt.Errorf("some tests failed")
	}

	return nil
}

func runSingleTest(benchPath, appName, site, version string, args []string) error {
	if version != "" {
		PrintInfo("Testing %s with Frappe %s...", appName, version)
	} else {
		PrintInfo("Testing %s...", appName)
	}

	// Build bench test command
	cmdArgs := []string{"--site", site, "run-tests", "--app", appName}
	cmdArgs = append(cmdArgs, args...)

	testCommand := exec.Command("bench", cmdArgs...)
	testCommand.Dir = benchPath
	testCommand.Stdout = os.Stdout
	testCommand.Stderr = os.Stderr
	testCommand.Stdin = os.Stdin

	// Set environment for specific version if needed
	if version != "" {
		env := os.Environ()
		env = append(env, fmt.Sprintf("FRAPPE_VERSION=%s", version))
		testCommand.Env = env
	}

	return testCommand.Run()
}

func runVersionTest(benchPath, appName, site, version string, args []string) testResult {
	result := testResult{Version: version}

	if !testParallel {
		PrintInfo("")
		PrintInfo("Testing with Frappe %s...", version)
		PrintInfo("---")
	}

	// For multi-version testing, we need a separate environment per version
	// This is a simplified implementation - full implementation would create
	// isolated environments per version

	versionBenchPath := benchPath
	if version != "" {
		// Check if we have a version-specific environment
		versionPath := filepath.Join(filepath.Dir(benchPath), fmt.Sprintf(".weg-%s", version))
		if _, err := os.Stat(versionPath); err == nil {
			versionBenchPath = versionPath
		} else {
			// For now, warn that we're using the default environment
			if !testParallel {
				PrintVerbose("Note: Using default environment for version %s testing", version)
				PrintVerbose("For isolated testing, create .weg-%s environment", version)
			}
		}
	}

	// Build and run test command
	cmdArgs := []string{"--site", site, "run-tests", "--app", appName}
	cmdArgs = append(cmdArgs, args...)

	testCommand := exec.Command("bench", cmdArgs...)
	testCommand.Dir = versionBenchPath

	// Set environment
	env := os.Environ()
	if version != "" {
		env = append(env, fmt.Sprintf("FRAPPE_VERSION=%s", version))
	}
	testCommand.Env = env

	if testParallel {
		output, err := testCommand.CombinedOutput()
		result.Output = string(output)
		result.Error = err
		result.Success = err == nil
	} else {
		testCommand.Stdout = os.Stdout
		testCommand.Stderr = os.Stderr
		err := testCommand.Run()
		result.Error = err
		result.Success = err == nil
	}

	return result
}

func isValidVersion(version string) bool {
	return version == "14" || version == "15" || version == "16" ||
		tools.NormalizeFrappeVersion(version) != ""
}
