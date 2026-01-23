package services

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProcessComposeConfig represents a process-compose.yaml configuration
type ProcessComposeConfig struct {
	Version   string             `yaml:"version,omitempty"`
	Includes  []Include          `yaml:"includes,omitempty"`
	Processes map[string]Process `yaml:"processes"`
}

// Include represents a process-compose include directive
type Include struct {
	Path     string `yaml:"path"`
	Optional bool   `yaml:"optional,omitempty"`
}

// Process represents a single process in process-compose
type Process struct {
	Command     string               `yaml:"command"`
	WorkingDir  string               `yaml:"working_dir,omitempty"`
	Environment []string             `yaml:"environment,omitempty"`
	DependsOn   map[string]DependsOn `yaml:"depends_on,omitempty"`
	Disabled    bool                 `yaml:"disabled,omitempty"`
	ReadyProbe  *ReadyProbe          `yaml:"readiness_probe,omitempty"`
}

// DependsOn specifies process dependencies
type DependsOn struct {
	Condition string `yaml:"condition"` // process_started, process_healthy, process_completed_successfully
}

// ReadyProbe defines health check for process readiness
type ReadyProbe struct {
	HTTPGet       *HTTPGet `yaml:"http_get,omitempty"`
	InitialDelay  int      `yaml:"initial_delay_seconds,omitempty"`
	PeriodSeconds int      `yaml:"period_seconds,omitempty"`
}

// HTTPGet defines an HTTP health check
type HTTPGet struct {
	Host   string `yaml:"host,omitempty"`
	Port   int    `yaml:"port"`
	Path   string `yaml:"path,omitempty"`
	Scheme string `yaml:"scheme,omitempty"`
}

// ComposeOptions configures process-compose generation
type ComposeOptions struct {
	BenchPath     string
	WebPort       int
	SocketPort    int
	RedisCache    int
	RedisQueue    int
	IncludeRedis  bool
	IncludeWatch  bool
	FrappeVersion string
	NodePath      string         // Path to node binary (for devbox)
	UseVenvPython bool           // Use env/bin/python for bench commands (devbox projects)
	RunID         string         // Unique run ID for process identification
	Workers       map[string]int // Queue name -> instance count ("all" = all queues)
}

// DefaultComposeOptions returns sensible defaults
func DefaultComposeOptions(benchPath string) ComposeOptions {
	return ComposeOptions{
		BenchPath:    benchPath,
		WebPort:      8000,
		SocketPort:   9000,
		RedisCache:   13000,
		RedisQueue:   11000,
		IncludeRedis: true,
		IncludeWatch: true,
	}
}

// GenerateProcessCompose creates the process-compose.yaml content
func GenerateProcessCompose(opts ComposeOptions) *ProcessComposeConfig {
	config := &ProcessComposeConfig{
		Version:   "0.5",
		Processes: make(map[string]Process),
	}

	// Add optional override file include
	config.Includes = []Include{
		{Path: "process-compose.override.yaml", Optional: true},
	}

	// Helper to generate bench command with correct Python
	benchCmd := func(args string) string {
		if opts.UseVenvPython {
			// Use explicit Python path for devbox projects
			// Run from sites directory using bench_helper
			// bench_helper wraps frappe commands, so we need "frappe <cmd>"
			return fmt.Sprintf("cd sites && ../env/bin/python -m frappe.utils.bench_helper frappe %s", args)
		}
		return fmt.Sprintf("bench %s", args)
	}

	// Helper to create worker dependencies
	workerDeps := func() map[string]DependsOn {
		if opts.IncludeRedis {
			return map[string]DependsOn{"redis_queue": {Condition: "process_started"}}
		}
		return map[string]DependsOn{"web": {Condition: "process_started"}}
	}

	// Redis cache (only if not using external redis like devbox)
	if opts.IncludeRedis {
		config.Processes["redis_cache"] = Process{
			Command: fmt.Sprintf("redis-server config/redis_cache.conf"),
		}
		config.Processes["redis_queue"] = Process{
			Command: fmt.Sprintf("redis-server config/redis_queue.conf"),
		}
	}

	// Web server (gunicorn via bench serve)
	webProcess := Process{
		Command: benchCmd(fmt.Sprintf("serve --port %d", opts.WebPort)),
	}
	if opts.IncludeRedis {
		webProcess.DependsOn = map[string]DependsOn{
			"redis_cache": {Condition: "process_started"},
			"redis_queue": {Condition: "process_started"},
		}
	}
	config.Processes["web"] = webProcess

	// Socket.io server
	nodePath := opts.NodePath
	if nodePath == "" {
		nodePath = "node"
	}
	config.Processes["socketio"] = Process{
		Command: fmt.Sprintf("%s apps/frappe/socketio.js", nodePath),
		Environment: []string{
			fmt.Sprintf("PORT=%d", opts.SocketPort),
		},
		DependsOn: map[string]DependsOn{
			"web": {Condition: "process_started"},
		},
	}

	// Generate workers based on Workers map
	workers := opts.Workers

	// If no config specified, default to 1 worker for all queues
	if len(workers) == 0 {
		workers = map[string]int{"all": 1}
	}

	// Generate workers for each queue
	for queue, count := range workers {
		if count <= 0 {
			continue
		}

		// Determine the queue flag
		var queueFlag string
		if queue == "all" {
			// "all" is a special case - consume all standard queues
			queueFlag = "short,default,long"
		} else {
			queueFlag = queue
		}

		for i := 0; i < count; i++ {
			var name string
			if queue == "all" {
				name = "worker"
				if count > 1 {
					name = fmt.Sprintf("worker_%d", i+1)
				}
			} else {
				name = fmt.Sprintf("worker_%s", queue)
				if count > 1 {
					name = fmt.Sprintf("worker_%s_%d", queue, i+1)
				}
			}
			config.Processes[name] = Process{
				Command:   benchCmd(fmt.Sprintf("worker --queue %s", queueFlag)),
				DependsOn: workerDeps(),
			}
		}
	}

	// Scheduler
	config.Processes["scheduler"] = Process{
		Command: benchCmd("schedule"),
		DependsOn: map[string]DependsOn{
			"web": {Condition: "process_started"},
		},
	}

	// Watch mode for development
	if opts.IncludeWatch {
		config.Processes["watch"] = Process{
			Command: benchCmd("watch"),
			DependsOn: map[string]DependsOn{
				"web": {Condition: "process_started"},
			},
		}
	}

	// Add DEV_SERVER env var to all processes for development mode detection
	for name, proc := range config.Processes {
		proc.Environment = append(proc.Environment, "DEV_SERVER=1")
		config.Processes[name] = proc
	}

	// Add WEG_RUNNER env var to all processes for identification
	if opts.RunID != "" {
		wegEnv := fmt.Sprintf("WEG_RUNNER=%s", opts.RunID)
		for name, proc := range config.Processes {
			proc.Environment = append(proc.Environment, wegEnv)
			config.Processes[name] = proc
		}
	}

	return config
}

// WriteProcessCompose writes the process-compose.yaml file
func WriteProcessCompose(benchPath string, config *ProcessComposeConfig) error {
	composePath := filepath.Join(benchPath, "process-compose.yaml")

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal process-compose config: %w", err)
	}

	// Add header comment
	header := "# Generated by weg - do not edit manually\n# Run 'weg start' to use this configuration\n\n"
	content := header + string(data)

	if err := os.WriteFile(composePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write process-compose.yaml: %w", err)
	}

	return nil
}

// GenerateAndWrite is a convenience function that generates and writes in one step
func GenerateAndWrite(opts ComposeOptions) error {
	config := GenerateProcessCompose(opts)
	return WriteProcessCompose(opts.BenchPath, config)
}
