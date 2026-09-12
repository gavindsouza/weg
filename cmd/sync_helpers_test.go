package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCommonSiteConfigKeepsUserKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "common_site_config.json")
	if err := os.WriteFile(path, []byte(`{"mail_server": "localhost", "webserver_port": 1}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := mergeConfigJSON(path, map[string]any{"webserver_port": 8040}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["mail_server"] != "localhost" || got["webserver_port"] != float64(8040) {
		t.Fatalf("got %v, want user key kept and managed key overwritten", got)
	}

	// Corrupt file must not be clobbered.
	os.WriteFile(path, []byte(`{not json`), 0644)
	if err := mergeConfigJSON(path, map[string]any{"webserver_port": 8040}); err == nil {
		t.Fatal("expected error on unparseable existing config")
	}
}
