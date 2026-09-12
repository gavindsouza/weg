package services

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/gavindsouza/weg/internal/config"
	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/tools"
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
	Command      string               `yaml:"command"`
	WorkingDir   string               `yaml:"working_dir,omitempty"`
	Environment  []string             `yaml:"environment,omitempty"`
	DependsOn    map[string]DependsOn `yaml:"depends_on,omitempty"`
	Disabled     bool                 `yaml:"disabled,omitempty"`
	ReadyProbe   *ReadyProbe          `yaml:"readiness_probe,omitempty"`
	Availability *Availability        `yaml:"availability,omitempty"`
	Shutdown     *Shutdown            `yaml:"shutdown,omitempty"`
}

// Shutdown defines how process-compose stops a process
type Shutdown struct {
	Command        string `yaml:"command,omitempty"`
	Signal         int    `yaml:"signal,omitempty"`
	TimeoutSeconds int    `yaml:"timeout_seconds,omitempty"`
}

// Availability defines a process's restart policy
type Availability struct {
	Restart     string `yaml:"restart,omitempty"`
	MaxRestarts int    `yaml:"max_restarts,omitempty"`
}

// DependsOn specifies process dependencies
type DependsOn struct {
	Condition string `yaml:"condition"` // process_started, process_healthy, process_completed_successfully
}

// ReadyProbe defines health check for process readiness
type ReadyProbe struct {
	Exec          *Exec    `yaml:"exec,omitempty"`
	HTTPGet       *HTTPGet `yaml:"http_get,omitempty"`
	InitialDelay  int      `yaml:"initial_delay_seconds,omitempty"`
	PeriodSeconds int      `yaml:"period_seconds,omitempty"`
}

// Exec defines a command health check
type Exec struct {
	Command string `yaml:"command"`
}

// devbox drops a plugin's process when the project's process-compose.yaml
// defines one with the same name, so weg replaces two plugin processes there.
// Both are disabled: `process-compose up -f` on this file (weg start's app
// processes) never starts a second copy, while `devbox services start <name>`
// names them, which still starts them.

// DevboxRedis replaces the devbox redis plugin's process. The plugin probes
// `redis-cli -p $REDIS_PORT ping`, which always fails for weg's socket-only
// redis (port 0), so process-compose SIGTERMs redis ~20s after start.
func DevboxRedis(benchPath string) Process {
	return Process{
		Command:      `redis-server "$REDIS_CONF" --port "$REDIS_PORT"`,
		Disabled:     true,
		ReadyProbe:   &ReadyProbe{Exec: &Exec{Command: fmt.Sprintf("redis-cli -s %q ping", devboxRedisSocket(benchPath))}},
		Availability: &Availability{Restart: "on_failure", MaxRestarts: 5}, // as the plugin
	}
}

// DevboxMariaDB replaces the devbox mariadb plugin's process. The plugin
// stops mariadbd with a passwordless `mariadb-admin -u root shutdown`, which
// weg's root password refuses; process-compose then kills mariadbd (crash
// recovery on the next start, a stale socket) and its `restart: always`
// brings it straight back. SIGTERM is mariadbd's clean shutdown.
func DevboxMariaDB(benchPath string) Process {
	return Process{
		Command:  `exec mariadbd --log-error="$MYSQL_HOME/mysql.log"`,
		Disabled: true,
		// --no-defaults: a [client] socket in /etc/mysql or ~/.my.cnf would
		// win otherwise. ping exits 0 even on access denied.
		ReadyProbe:   &ReadyProbe{Exec: &Exec{Command: fmt.Sprintf("mariadb-admin --no-defaults -S %q ping", devboxMariaDBSocket(benchPath))}, InitialDelay: 2},
		Availability: &Availability{Restart: "on_failure", MaxRestarts: 5},
		Shutdown:     &Shutdown{Signal: 15, TimeoutSeconds: 30},
	}
}

func devboxServiceOverrides(benchPath string) map[string]Process {
	return map[string]Process{"redis": DevboxRedis(benchPath), "mariadb": DevboxMariaDB(benchPath)}
}

// devboxMariaDBSocket is the absolute path of the bench's devbox MariaDB socket.
func devboxMariaDBSocket(benchPath string) string {
	if abs, err := filepath.Abs(benchPath); err == nil {
		benchPath = abs
	}
	return filepath.Join(benchPath, ".devbox", "virtenv", "mariadb", "run", "mysql.sock")
}

// devboxRedisSocket is the absolute path of weg's socket-only devbox redis.
func devboxRedisSocket(benchPath string) string {
	if abs, err := filepath.Abs(benchPath); err == nil {
		benchPath = abs
	}
	return filepath.Join(benchPath, ".devbox", "virtenv", "redis", "redis.sock")
}

// StartDatabase makes sure the bench's devbox MariaDB and redis are running
// and returns the MariaDB socket path. Running services are left alone
// (devbox would print "already running" errors).
func StartDatabase(benchPath string) (string, error) {
	mysqlSocket := filepath.Join(benchPath, ".devbox", "virtenv", "mariadb", "run", "mysql.sock")

	// A running services instance keeps the config it started with, so
	// restart it when an override was just added.
	changed, err := EnsureDevboxServices(benchPath)
	if err != nil {
		return "", err
	}
	// Connect rather than stat: a killed server leaves its socket file behind.
	dbUp := listening(mysqlSocket)
	if !changed && dbUp && listening(devboxRedisSocket(benchPath)) {
		return mysqlSocket, nil
	}
	start := []string{"services", "start", "redis"}
	if changed || !dbUp {
		stopDevboxServices(benchPath) // also clears a manager whose mariadb died
		start = append(start, "mariadb")
	}

	cmd := exec.Command("devbox", start...)
	cmd.Dir = benchPath
	out, startErr := cmd.CombinedOutput()

	for i := 0; i < 60; i++ {
		if listening(mysqlSocket) {
			return mysqlSocket, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("MariaDB isn't accepting connections on %s after 30 seconds (devbox services start: %v)\n%s\nSee .devbox/virtenv/mariadb/run/mysql.log; try 'devbox services stop' and run the command again", mysqlSocket, startErr, out)
}

// MariaDBRoot runs a statement as MariaDB root over the bench's socket. The
// client comes straight from devbox's profile and the password through
// MYSQL_PWD (`devbox run` would eval the arguments and mangle a `$`);
// --no-defaults keeps a ~/.my.cnf password from overriding it. `mysql`, not
// `mariadb`: the latter only exists from MariaDB 10.4.6.
func MariaDBRoot(benchPath, mysqlSocket, password, stmt string) error {
	client := filepath.Join(benchPath, ".devbox", "nix", "profile", "default", "bin", "mysql")
	cmd := exec.Command(client, "--no-defaults", "-S", mysqlSocket, "-uroot", "-e", stmt)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DropSite drops a site's database and archives its directory with frappe's
// drop-site. A site whose directory is already gone counts as dropped. If
// drop-site fails (e.g. the database went with a reset .devbox), the database
// and user named in site_config.json are dropped directly and the directory
// archived the way drop-site would, so removal can't get stuck.
func DropSite(benchPath, name, rootPassword string, dropArgs ...string) error {
	siteDir := filepath.Join(benchPath, "sites", name)
	if _, err := os.Stat(siteDir); os.IsNotExist(err) {
		output.Warningf("site directory %s is gone; any database it had is left in place", siteDir)
		return nil
	}
	mysqlSocket, err := StartDatabase(benchPath)
	if err != nil {
		return err
	}

	args := append([]string{"frappe", "drop-site", name, "--force", "--db-root-password", rootPassword}, dropArgs...)
	cmd := tools.FrappeCmd(benchPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dropErr := cmd.Run(); dropErr != nil {
		var conf struct {
			DBName string `json:"db_name"`
			DBUser string `json:"db_user"`
		}
		data, err := os.ReadFile(filepath.Join(siteDir, "site_config.json"))
		if err != nil || json.Unmarshal(data, &conf) != nil || conf.DBName == "" {
			return fmt.Errorf("drop-site %s failed (%v) and its site_config.json has no db_name to drop by hand", name, dropErr)
		}
		if conf.DBUser == "" {
			conf.DBUser = conf.DBName // frappe's default
		}
		output.Warningf("drop-site %s failed (%v); dropping database %s directly", name, dropErr, conf.DBName)
		// db names are frappe-generated identifiers; backtick-quote anyway.
		stmt := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`; DROP USER IF EXISTS '%s'@'localhost'; DROP USER IF EXISTS '%s'@'%%';",
			strings.ReplaceAll(conf.DBName, "`", "``"), strings.ReplaceAll(conf.DBUser, "'", "''"), strings.ReplaceAll(conf.DBUser, "'", "''"))
		if err := MariaDBRoot(benchPath, mysqlSocket, rootPassword, stmt); err != nil {
			return fmt.Errorf("drop-site %s failed (%v), and dropping its database directly failed: %w", name, dropErr, err)
		}
		return archiveSite(benchPath, siteDir, dropArgs)
	}
	return nil
}

// archiveSite moves a site directory where frappe's drop-site would:
// --archived-sites-path or <bench>/archived/sites, as <name>, <name>1, ...
// Its files and backups may be all that's left of the site.
func archiveSite(benchPath, siteDir string, dropArgs []string) error {
	dest := filepath.Join(benchPath, "archived", "sites")
	if i := slices.Index(dropArgs, "--archived-sites-path"); i >= 0 && i+1 < len(dropArgs) {
		dest = dropArgs[i+1]
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	target := filepath.Join(dest, filepath.Base(siteDir))
	for n := 1; fileExists(target); n++ {
		target = filepath.Join(dest, fmt.Sprintf("%s%d", filepath.Base(siteDir), n))
	}
	if err := os.Rename(siteDir, target); err != nil {
		return err
	}
	output.Infof("Moved site to %s", target)
	return nil
}

// ShutdownMariaDB shuts the bench's MariaDB down cleanly and waits for it to
// finish. `devbox services stop` kills the whole process group outright, so
// without this mariadbd crash-recovers on every start.
func ShutdownMariaDB(benchPath string) {
	run := filepath.Join(benchPath, ".devbox", "virtenv", "mariadb", "run")
	sock := filepath.Join(run, "mysql.sock")
	if !listening(sock) {
		return
	}
	if MariaDBRoot(benchPath, sock, config.DBRootPassword(benchPath), "SHUTDOWN") != nil &&
		MariaDBRoot(benchPath, sock, "", "SHUTDOWN") != nil { // fresh, passwordless root
		return // can't log in; the kill it is
	}
	// The socket closes early in shutdown; the pid file goes last.
	for i := 0; i < 60 && fileExists(filepath.Join(run, "mysql.pid")); i++ {
		time.Sleep(500 * time.Millisecond)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// listening reports whether something accepts connections on a unix socket.
func listening(socket string) bool {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// stopDevboxServices stops the bench's devbox services, quietly: when none
// are running devbox reports it as an error, which is noise here.
func stopDevboxServices(benchPath string) {
	ShutdownMariaDB(benchPath)
	cmd := exec.Command("devbox", "services", "stop")
	cmd.Dir = benchPath
	cmd.Run()
}

// EnsureDevboxServices adds (or refreshes stale) DevboxRedis/DevboxMariaDB
// overrides in the bench's process-compose.yaml, creating the file if
// needed, and reports whether it changed anything.
func EnsureDevboxServices(benchPath string) (bool, error) {
	cfg := &ProcessComposeConfig{Version: "0.5"}
	data, err := os.ReadFile(filepath.Join(benchPath, "process-compose.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return false, fmt.Errorf("failed to parse process-compose.yaml: %w", err)
	}
	if cfg.Processes == nil {
		cfg.Processes = map[string]Process{}
	}
	changed := false
	for name, want := range devboxServiceOverrides(benchPath) {
		have, ok := cfg.Processes[name]
		have.Environment = nil // weg start tags every process with WEG_RUNNER
		if ok && reflect.DeepEqual(have, want) {
			continue
		}
		cfg.Processes[name] = want
		changed = true
	}
	if !changed {
		return false, nil
	}
	return true, WriteProcessCompose(benchPath, cfg)
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

	if !opts.IncludeRedis {
		for name, proc := range devboxServiceOverrides(opts.BenchPath) {
			config.Processes[name] = proc
		}
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
	socketioEnv := []string{fmt.Sprintf("PORT=%d", opts.SocketPort)}
	if !opts.IncludeRedis {
		// frappe's node client takes everything after unix:// as the socket
		// path, so redis_queue's "?db=1" breaks it. Pub/sub ignores the db;
		// FRAPPE_REDIS_QUEUE overrides the config for node only.
		socketioEnv = append(socketioEnv, "FRAPPE_REDIS_QUEUE=unix://"+devboxRedisSocket(opts.BenchPath))
	}
	config.Processes["socketio"] = Process{
		Command:     fmt.Sprintf("%s apps/frappe/socketio.js", nodePath),
		Environment: socketioEnv,
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
