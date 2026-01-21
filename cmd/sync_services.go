package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/services"
	"gopkg.in/yaml.v3"
)

// regenerateProcessCompose regenerates process-compose.yaml from config
func regenerateProcessCompose(benchPath string, cfg *config.BenchConfig) error {
	opts := services.ComposeOptions{
		BenchPath:     benchPath,
		WebPort:       cfg.Services.Web.Port,
		SocketPort:    cfg.Services.Web.SocketPort,
		IncludeRedis:  false, // Devbox manages redis
		IncludeWatch:  true,
		UseVenvPython: true,
		Workers:       cfg.Services.Workers,
	}

	// Set defaults
	if opts.WebPort == 0 {
		opts.WebPort = 8000
	}
	if opts.SocketPort == 0 {
		opts.SocketPort = 9000
	}

	return services.GenerateAndWrite(opts)
}

// syncAppServices collects services from all apps and applies them
func syncAppServices(benchPath string) error {
	appsDir := filepath.Join(benchPath, "apps")

	// Collect services from all apps
	packages, processes, warnings, err := config.CollectAppServices(appsDir)
	if err != nil {
		PrintVerbose("Warning: failed to collect app services: %v", err)
		return nil // Non-fatal
	}

	// Log any parse warnings
	for _, warn := range warnings {
		PrintVerbose("Warning: %s", warn)
	}

	// Add packages to devbox.json
	if len(packages) > 0 {
		if err := addDevboxPackages(benchPath, packages); err != nil {
			return fmt.Errorf("failed to add devbox packages: %w", err)
		}
	}

	// Merge processes into process-compose.yaml
	if len(processes) > 0 {
		if err := mergeAppProcesses(benchPath, processes); err != nil {
			return fmt.Errorf("failed to merge app processes: %w", err)
		}
	}

	return nil
}

// devboxConfig represents devbox.json structure
type devboxConfig struct {
	Schema   string                 `json:"$schema,omitempty"`
	Packages []string               `json:"packages"`
	Shell    map[string]interface{} `json:"shell,omitempty"`
}

// addDevboxPackages adds packages to devbox.json if not already present
func addDevboxPackages(benchPath string, packages []string) error {
	devboxPath := filepath.Join(benchPath, "devbox.json")

	data, err := os.ReadFile(devboxPath)
	if err != nil {
		return fmt.Errorf("failed to read devbox.json: %w", err)
	}

	var cfg devboxConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse devbox.json: %w", err)
	}

	// Build set of existing packages (without version suffix for comparison)
	existing := make(map[string]bool)
	for _, pkg := range cfg.Packages {
		// Extract package name without version (e.g., "tor@latest" -> "tor")
		name := pkg
		if idx := strings.Index(pkg, "@"); idx > 0 {
			name = pkg[:idx]
		}
		existing[name] = true
	}

	// Add new packages
	added := false
	for _, pkg := range packages {
		name := pkg
		if idx := strings.Index(pkg, "@"); idx > 0 {
			name = pkg[:idx]
		}
		if !existing[name] {
			cfg.Packages = append(cfg.Packages, pkg)
			existing[name] = true
			added = true
			PrintInfo("  Adding devbox package: %s", pkg)
		}
	}

	if !added {
		return nil // No changes needed
	}

	// Write updated devbox.json
	data, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal devbox.json: %w", err)
	}

	if err := os.WriteFile(devboxPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write devbox.json: %w", err)
	}

	// Run devbox install to apply changes
	PrintInfo("  Installing new devbox packages...")
	if err := runCmdInDir(benchPath, "devbox", "install"); err != nil {
		PrintVerbose("Warning: devbox install failed: %v", err)
	}

	return nil
}

// mergeAppProcesses adds app-defined processes to process-compose.yaml
func mergeAppProcesses(benchPath string, processes map[string]config.AppProcessConfig) error {
	composePath := filepath.Join(benchPath, "process-compose.yaml")

	// Read existing process-compose.yaml
	data, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("failed to read process-compose.yaml: %w", err)
	}

	var composeConfig services.ProcessComposeConfig
	if err := yaml.Unmarshal(data, &composeConfig); err != nil {
		return fmt.Errorf("failed to parse process-compose.yaml: %w", err)
	}

	// Find WEG_RUNNER from existing processes to add to new ones
	var wegRunnerEnv string
	for _, proc := range composeConfig.Processes {
		for _, env := range proc.Environment {
			if strings.HasPrefix(env, "WEG_RUNNER=") {
				wegRunnerEnv = env
				break
			}
		}
		if wegRunnerEnv != "" {
			break
		}
	}

	// Add app processes
	for name, proc := range processes {
		if _, exists := composeConfig.Processes[name]; exists {
			continue // Don't override existing processes
		}

		// Convert environment map to slice
		var envSlice []string
		for k, v := range proc.Environment {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
		}

		// Add WEG_RUNNER env var for process identification
		if wegRunnerEnv != "" {
			envSlice = append(envSlice, wegRunnerEnv)
		}

		// Convert depends_on list to map
		var depsMap map[string]services.DependsOn
		if len(proc.DependsOn) > 0 {
			depsMap = make(map[string]services.DependsOn)
			for _, dep := range proc.DependsOn {
				depsMap[dep] = services.DependsOn{Condition: "process_started"}
			}
		}

		composeConfig.Processes[name] = services.Process{
			Command:     proc.Command,
			WorkingDir:  proc.WorkingDir,
			Environment: envSlice,
			DependsOn:   depsMap,
		}
		PrintInfo("  Adding process: %s", name)
	}

	// Write updated process-compose.yaml
	return services.WriteProcessCompose(benchPath, &composeConfig)
}
