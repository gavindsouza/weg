package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultComposeOptions(t *testing.T) {
	opts := DefaultComposeOptions("/test/bench")

	if opts.BenchPath != "/test/bench" {
		t.Errorf("BenchPath = %q, want /test/bench", opts.BenchPath)
	}
	if opts.WebPort != 8000 {
		t.Errorf("WebPort = %d, want 8000", opts.WebPort)
	}
	if opts.SocketPort != 9000 {
		t.Errorf("SocketPort = %d, want 9000", opts.SocketPort)
	}
	if opts.RedisCache != 13000 {
		t.Errorf("RedisCache = %d, want 13000", opts.RedisCache)
	}
	if opts.RedisQueue != 11000 {
		t.Errorf("RedisQueue = %d, want 11000", opts.RedisQueue)
	}
	if !opts.IncludeRedis {
		t.Error("IncludeRedis should be true by default")
	}
	if !opts.IncludeWatch {
		t.Error("IncludeWatch should be true by default")
	}
}

func TestGenerateProcessComposeBasic(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:    "/test/bench",
		WebPort:      8000,
		SocketPort:   9000,
		IncludeRedis: true,
		IncludeWatch: true,
	}

	config := GenerateProcessCompose(opts)

	if config.Version != "0.5" {
		t.Errorf("Version = %q, want 0.5", config.Version)
	}

	// Check required processes exist
	requiredProcesses := []string{"redis_cache", "redis_queue", "web", "socketio", "worker", "scheduler", "watch"}
	for _, proc := range requiredProcesses {
		if _, ok := config.Processes[proc]; !ok {
			t.Errorf("Missing process: %s", proc)
		}
	}
}

func TestGenerateProcessComposeWithoutRedis(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:    "/test/bench",
		WebPort:      8000,
		SocketPort:   9000,
		IncludeRedis: false,
		IncludeWatch: true,
	}

	config := GenerateProcessCompose(opts)

	// Redis processes should not exist
	if _, ok := config.Processes["redis_cache"]; ok {
		t.Error("redis_cache should not exist when IncludeRedis is false")
	}
	if _, ok := config.Processes["redis_queue"]; ok {
		t.Error("redis_queue should not exist when IncludeRedis is false")
	}

	// Web should not depend on redis
	web := config.Processes["web"]
	if web.DependsOn != nil {
		for dep := range web.DependsOn {
			if strings.Contains(dep, "redis") {
				t.Errorf("web should not depend on %s when IncludeRedis is false", dep)
			}
		}
	}

	// Worker should depend on web instead of redis
	worker := config.Processes["worker"]
	if worker.DependsOn == nil {
		t.Error("worker should have dependencies")
	} else if _, ok := worker.DependsOn["web"]; !ok {
		t.Error("worker should depend on web when IncludeRedis is false")
	}
}

func TestGenerateProcessComposeWithoutWatch(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:    "/test/bench",
		WebPort:      8000,
		SocketPort:   9000,
		IncludeRedis: true,
		IncludeWatch: false,
	}

	config := GenerateProcessCompose(opts)

	if _, ok := config.Processes["watch"]; ok {
		t.Error("watch process should not exist when IncludeWatch is false")
	}
}

func TestGenerateProcessComposeCustomPorts(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:    "/test/bench",
		WebPort:      8080,
		SocketPort:   9090,
		IncludeRedis: true,
	}

	config := GenerateProcessCompose(opts)

	// Web command should include custom port
	web := config.Processes["web"]
	if !strings.Contains(web.Command, "8080") {
		t.Errorf("Web command should contain port 8080: %s", web.Command)
	}

	// SocketIO should have correct port env
	socketio := config.Processes["socketio"]
	foundPort := false
	for _, env := range socketio.Environment {
		if env == "PORT=9090" {
			foundPort = true
			break
		}
	}
	if !foundPort {
		t.Errorf("SocketIO environment should contain PORT=9090: %v", socketio.Environment)
	}
}

func TestGenerateProcessComposeVenvPython(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:     "/test/bench",
		WebPort:       8000,
		SocketPort:    9000,
		UseVenvPython: true,
		IncludeRedis:  true,
	}

	config := GenerateProcessCompose(opts)

	// Commands should use .venv/bin/python instead of bench
	web := config.Processes["web"]
	if !strings.Contains(web.Command, ".venv/bin/python") {
		t.Errorf("Web command should use venv python: %s", web.Command)
	}
	if !strings.Contains(web.Command, "frappe.utils.bench_helper") {
		t.Errorf("Web command should use bench_helper: %s", web.Command)
	}

	// Should not contain plain "bench" command
	worker := config.Processes["worker"]
	if strings.HasPrefix(worker.Command, "bench ") {
		t.Errorf("Worker command should not start with 'bench ': %s", worker.Command)
	}
}

func TestGenerateProcessComposeCustomNodePath(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:    "/test/bench",
		WebPort:      8000,
		SocketPort:   9000,
		NodePath:     "/custom/node",
		IncludeRedis: true,
	}

	config := GenerateProcessCompose(opts)

	socketio := config.Processes["socketio"]
	if !strings.HasPrefix(socketio.Command, "/custom/node ") {
		t.Errorf("SocketIO should use custom node path: %s", socketio.Command)
	}
}

func TestGenerateProcessComposeRunID(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:    "/test/bench",
		WebPort:      8000,
		SocketPort:   9000,
		RunID:        "abc123",
		IncludeRedis: true,
		IncludeWatch: true,
	}

	config := GenerateProcessCompose(opts)

	// All processes should have WEG_RUNNER env var
	for name, proc := range config.Processes {
		found := false
		for _, env := range proc.Environment {
			if env == "WEG_RUNNER=abc123" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Process %s should have WEG_RUNNER=abc123 env var", name)
		}
	}
}

func TestGenerateProcessComposeDependencies(t *testing.T) {
	opts := ComposeOptions{
		BenchPath:    "/test/bench",
		WebPort:      8000,
		SocketPort:   9000,
		IncludeRedis: true,
	}

	config := GenerateProcessCompose(opts)

	// Web should depend on redis
	web := config.Processes["web"]
	if web.DependsOn == nil {
		t.Error("web should have dependencies")
	}
	if _, ok := web.DependsOn["redis_cache"]; !ok {
		t.Error("web should depend on redis_cache")
	}
	if _, ok := web.DependsOn["redis_queue"]; !ok {
		t.Error("web should depend on redis_queue")
	}

	// SocketIO should depend on web
	socketio := config.Processes["socketio"]
	if socketio.DependsOn == nil {
		t.Error("socketio should have dependencies")
	}
	if _, ok := socketio.DependsOn["web"]; !ok {
		t.Error("socketio should depend on web")
	}

	// Scheduler should depend on web
	scheduler := config.Processes["scheduler"]
	if scheduler.DependsOn == nil {
		t.Error("scheduler should have dependencies")
	}
	if _, ok := scheduler.DependsOn["web"]; !ok {
		t.Error("scheduler should depend on web")
	}
}

func TestWriteProcessCompose(t *testing.T) {
	tmpDir := t.TempDir()

	config := &ProcessComposeConfig{
		Version: "0.5",
		Processes: map[string]Process{
			"web": {
				Command: "bench serve",
			},
		},
	}

	err := WriteProcessCompose(tmpDir, config)
	if err != nil {
		t.Fatalf("WriteProcessCompose failed: %v", err)
	}

	// Verify file exists
	composePath := filepath.Join(tmpDir, "process-compose.yaml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	content := string(data)

	// Should have header comment
	if !strings.HasPrefix(content, "# Generated by weg") {
		t.Error("File should start with header comment")
	}

	// Should contain process
	if !strings.Contains(content, "web:") {
		t.Error("File should contain web process")
	}

	// Should contain command
	if !strings.Contains(content, "bench serve") {
		t.Error("File should contain bench serve command")
	}
}

func TestGenerateAndWrite(t *testing.T) {
	tmpDir := t.TempDir()

	opts := ComposeOptions{
		BenchPath:    tmpDir,
		WebPort:      8000,
		SocketPort:   9000,
		IncludeRedis: true,
		IncludeWatch: false,
	}

	err := GenerateAndWrite(opts)
	if err != nil {
		t.Fatalf("GenerateAndWrite failed: %v", err)
	}

	// Verify file exists and has content
	composePath := filepath.Join(tmpDir, "process-compose.yaml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	content := string(data)

	// Should have all required processes
	for _, proc := range []string{"redis_cache", "redis_queue", "web", "socketio", "worker", "scheduler"} {
		if !strings.Contains(content, proc+":") {
			t.Errorf("File should contain %s process", proc)
		}
	}

	// Should not have watch (disabled)
	if strings.Contains(content, "watch:") {
		t.Error("File should not contain watch process")
	}
}
