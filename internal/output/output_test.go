package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    Format
		wantErr bool
	}{
		{"auto", FormatAuto, false},
		{"", FormatAuto, false},
		{"json", FormatJSON, false},
		{"table", FormatTable, false},
		{"plain", FormatPlain, false},
		{"quiet", FormatQuiet, false},
		{"invalid", FormatAuto, true},
		{"JSON", FormatAuto, true}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFormat(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVerbosity(t *testing.T) {
	tests := []struct {
		input   string
		want    Verbosity
		wantErr bool
	}{
		{"quiet", VerbosityQuiet, false},
		{"normal", VerbosityNormal, false},
		{"", VerbosityNormal, false},
		{"verbose", VerbosityVerbose, false},
		{"debug", VerbosityDebug, false},
		{"trace", VerbosityTrace, false},
		{"invalid", VerbosityNormal, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVerbosity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVerbosity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseVerbosity(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDebugCategories(t *testing.T) {
	// Reset state
	DebugCategories = nil

	ParseDebugCategories("config,net,state")

	if !DebugCategories[DebugConfig] {
		t.Error("expected config category to be enabled")
	}
	if !DebugCategories[DebugNet] {
		t.Error("expected net category to be enabled")
	}
	if !DebugCategories[DebugState] {
		t.Error("expected state category to be enabled")
	}
	if DebugCategories[DebugGit] {
		t.Error("expected git category to be disabled")
	}

	// Cleanup
	DebugCategories = nil
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	table := NewTableWriter(&buf, "Name", "Status")

	table.Row("site1", "running")
	table.Row("site2", "stopped")
	table.Flush()

	output := buf.String()

	if !strings.Contains(output, "NAME") {
		t.Error("expected uppercase headers")
	}
	if !strings.Contains(output, "STATUS") {
		t.Error("expected uppercase headers")
	}
	if !strings.Contains(output, "site1") {
		t.Error("expected site1 in output")
	}
	if !strings.Contains(output, "running") {
		t.Error("expected running in output")
	}
}

func TestTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	table := NewTableWriter(&buf, "Name", "Status")

	if !table.Empty() {
		t.Error("expected table to be empty")
	}

	table.Row("site1", "running")

	if table.Empty() {
		t.Error("expected table to not be empty")
	}
}

func TestRedact(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"short", "***"},        // < 8 chars
		{"exactly8", "exa***ly8"}, // exactly 8 chars
		{"longsecretvalue", "lon***lue"},
		{"abcdefghij", "abc***hij"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsSecretField(t *testing.T) {
	secrets := []string{
		"password", "PASSWORD", "Password",
		"api_key", "API_KEY", "ApiKey",
		"secret", "token", "authorization",
	}

	for _, name := range secrets {
		if !IsSecretField(name) {
			t.Errorf("IsSecretField(%q) = false, want true", name)
		}
	}

	nonSecrets := []string{
		"name", "email", "status", "url", "path",
	}

	for _, name := range nonSecrets {
		if IsSecretField(name) {
			t.Errorf("IsSecretField(%q) = true, want false", name)
		}
	}
}

func TestRedactMap(t *testing.T) {
	input := map[string]any{
		"name":     "test",
		"password": "supersecret123",
		"api_key":  "abc123xyz789",
		"nested": map[string]any{
			"token": "nestedtoken",
		},
	}

	result := RedactMap(input)

	if result["name"] != "test" {
		t.Errorf("name should not be redacted, got %v", result["name"])
	}

	if result["password"] == "supersecret123" {
		t.Error("password should be redacted")
	}

	if result["api_key"] == "abc123xyz789" {
		t.Error("api_key should be redacted")
	}

	nested := result["nested"].(map[string]any)
	if nested["token"] == "nestedtoken" {
		t.Error("nested token should be redacted")
	}
}

func TestList_JSON(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatJSON

	type Item struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	items := []Item{
		{Name: "site1", Status: "running"},
		{Name: "site2", Status: "stopped"},
	}

	err := List(items)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"name"`) {
		t.Error("expected JSON output with name field")
	}
	if !strings.Contains(output, `"site1"`) {
		t.Error("expected site1 in JSON output")
	}
}

func TestList_Table(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatTable

	type Item struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	items := []Item{
		{Name: "site1", Status: "running"},
		{Name: "site2", Status: "stopped"},
	}

	err := List(items)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("expected table headers")
	}
	if !strings.Contains(output, "site1") {
		t.Error("expected site1 in table output")
	}
}

func TestList_Quiet(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatQuiet

	type Item struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	items := []Item{
		{Name: "site1", Status: "running"},
		{Name: "site2", Status: "stopped"},
	}

	err := List(items)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "site1" {
		t.Errorf("expected 'site1', got %q", lines[0])
	}
	if lines[1] != "site2" {
		t.Errorf("expected 'site2', got %q", lines[1])
	}
}

func TestDebugEnabled(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldCategories := DebugCategories
	defer func() {
		Level = oldLevel
		DebugCategories = oldCategories
	}()

	// Test: debug disabled at normal level
	Level = VerbosityNormal
	DebugCategories = nil
	if DebugEnabled(DebugConfig) {
		t.Error("expected debug disabled at normal level")
	}

	// Test: debug enabled at debug level (no categories = all enabled)
	Level = VerbosityDebug
	DebugCategories = nil
	if !DebugEnabled(DebugConfig) {
		t.Error("expected debug enabled at debug level")
	}

	// Test: specific category enabled
	Level = VerbosityDebug
	DebugCategories = map[DebugCategory]bool{DebugConfig: true}
	if !DebugEnabled(DebugConfig) {
		t.Error("expected config category enabled")
	}
	if DebugEnabled(DebugNet) {
		t.Error("expected net category disabled")
	}

	// Test: "all" enables everything
	DebugCategories = map[DebugCategory]bool{DebugAll: true}
	if !DebugEnabled(DebugConfig) {
		t.Error("expected all categories enabled with 'all'")
	}
	if !DebugEnabled(DebugNet) {
		t.Error("expected all categories enabled with 'all'")
	}
}

func TestSymbols(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityNormal
	CurrentFormat = FormatTable

	Success("test message")

	output := buf.String()
	if !strings.Contains(output, SymbolSuccess) {
		t.Errorf("expected success symbol in output, got %q", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected message in output, got %q", output)
	}
}

func TestSymbols_QuietMode(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityQuiet

	Success("test message")

	output := buf.String()
	if output != "" {
		t.Errorf("expected no output in quiet mode, got %q", output)
	}
}

func TestSymbols_JSONMode(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityNormal
	CurrentFormat = FormatJSON

	Success("test message")

	output := buf.String()
	if output != "" {
		t.Errorf("expected no status output in JSON mode, got %q", output)
	}
}
