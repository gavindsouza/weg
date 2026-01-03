package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRunID(t *testing.T) {
	id1 := GenerateRunID()
	id2 := GenerateRunID()

	// Should be 16 hex characters (8 bytes = 16 hex chars)
	if len(id1) != 16 {
		t.Errorf("expected 16 char run ID, got %d chars: %s", len(id1), id1)
	}

	// Should be different each time
	if id1 == id2 {
		t.Error("expected unique run IDs")
	}

	// Should be valid hex
	for _, c := range id1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex character in run ID: %c", c)
		}
	}
}

func TestDefaultPorts(t *testing.T) {
	ports := DefaultPorts()

	if ports.Web != 8000 {
		t.Errorf("expected Web port 8000, got %d", ports.Web)
	}
	if ports.SocketIO != 9000 {
		t.Errorf("expected SocketIO port 9000, got %d", ports.SocketIO)
	}
	if ports.Redis != 6379 {
		t.Errorf("expected Redis port 6379, got %d", ports.Redis)
	}
	if ports.ProcessCompose != 8080 {
		t.Errorf("expected ProcessCompose port 8080, got %d", ports.ProcessCompose)
	}
}

func TestRuntimePath(t *testing.T) {
	path := RuntimePath("/my/bench")
	expected := filepath.Join("/my/bench", "runtime.json")

	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Ports: Ports{
			Web:            8001,
			SocketIO:       9001,
			Redis:          6380,
			ProcessCompose: 8081,
		},
		PID:   12345,
		RunID: "abc123def456",
	}

	// Save
	err := cfg.Save(tmpDir)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	runtimePath := RuntimePath(tmpDir)
	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		t.Fatal("runtime.json not created")
	}

	// Load
	loaded, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values
	if loaded.Ports.Web != 8001 {
		t.Errorf("expected Web 8001, got %d", loaded.Ports.Web)
	}
	if loaded.Ports.SocketIO != 9001 {
		t.Errorf("expected SocketIO 9001, got %d", loaded.Ports.SocketIO)
	}
	if loaded.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", loaded.PID)
	}
	if loaded.RunID != "abc123def456" {
		t.Errorf("expected RunID abc123def456, got %s", loaded.RunID)
	}
}

func TestLoadNotExist(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := Load(tmpDir)
	if err == nil {
		t.Error("expected error for non-existent runtime.json")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	runtimePath := RuntimePath(tmpDir)
	err := os.WriteFile(runtimePath, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}

	_, err = Load(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()

	// Create runtime.json
	cfg := &Config{Ports: DefaultPorts()}
	cfg.Save(tmpDir)

	// Verify it exists
	runtimePath := RuntimePath(tmpDir)
	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		t.Fatal("runtime.json should exist before removal")
	}

	// Remove
	err := Remove(tmpDir)
	if err != nil {
		t.Fatalf("failed to remove: %v", err)
	}

	// Verify removed
	if _, err := os.Stat(runtimePath); !os.IsNotExist(err) {
		t.Error("runtime.json should be removed")
	}
}

func TestRemoveNotExist(t *testing.T) {
	tmpDir := t.TempDir()

	// Should not error when file doesn't exist
	err := Remove(tmpDir)
	if err != nil {
		t.Errorf("expected no error for non-existent file, got: %v", err)
	}
}

func TestIsPortAvailable(t *testing.T) {
	// Test with a very high port that's unlikely to be in use
	// Note: This test might be flaky depending on system state
	highPort := 59999

	// This might fail if port is actually in use, but unlikely
	available := IsPortAvailable(highPort)
	// We just test it doesn't panic - result depends on system state
	_ = available
}

func TestFindAvailablePort(t *testing.T) {
	// Find a port starting from a high number
	port, err := FindAvailablePort(55000)
	if err != nil {
		// This could fail if all 100 ports are in use, but very unlikely
		t.Logf("could not find available port: %v", err)
		return
	}

	if port < 55000 || port >= 55100 {
		t.Errorf("expected port in range [55000, 55100), got %d", port)
	}
}

func TestGetWebURL(t *testing.T) {
	cfg := &Config{
		Ports: Ports{
			Web: 8080,
		},
	}

	url := cfg.GetWebURL("mysite.localhost")
	expected := "http://mysite.localhost:8080"

	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Ports: Ports{
			Web:            8000,
			SocketIO:       9000,
			Redis:          6379,
			ProcessCompose: 8080,
		},
		PID:   99999,
		RunID: "testrun123",
	}

	if cfg.PID != 99999 {
		t.Errorf("unexpected PID: %d", cfg.PID)
	}
	if cfg.RunID != "testrun123" {
		t.Errorf("unexpected RunID: %s", cfg.RunID)
	}
}

func TestPortsStruct(t *testing.T) {
	ports := Ports{
		Web:            8080,
		SocketIO:       9090,
		Redis:          6380,
		ProcessCompose: 8081,
	}

	if ports.Web != 8080 {
		t.Errorf("unexpected Web: %d", ports.Web)
	}
	if ports.ProcessCompose != 8081 {
		t.Errorf("unexpected ProcessCompose: %d", ports.ProcessCompose)
	}
}

func TestLoadIfRunningNoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadIfRunning(tmpDir)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config for non-existent file")
	}
}

func TestAllocatePorts(t *testing.T) {
	preferred := Ports{
		Web:            55000,
		SocketIO:       55001,
		Redis:          6379,
		ProcessCompose: 55002,
	}

	ports, err := AllocatePorts(preferred)
	if err != nil {
		t.Logf("could not allocate ports (system may have them in use): %v", err)
		return
	}

	// Web port should be >= preferred
	if ports.Web < preferred.Web {
		t.Errorf("Web port %d should be >= %d", ports.Web, preferred.Web)
	}

	// SocketIO port should be >= preferred
	if ports.SocketIO < preferred.SocketIO {
		t.Errorf("SocketIO port %d should be >= %d", ports.SocketIO, preferred.SocketIO)
	}

	// Redis should be unchanged (managed externally)
	if ports.Redis != preferred.Redis {
		t.Errorf("Redis port should be %d, got %d", preferred.Redis, ports.Redis)
	}
}
