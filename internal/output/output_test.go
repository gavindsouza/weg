package output

import (
	"bytes"
	"net/http"
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
		{"short", "***"},          // < 8 chars
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

func TestTraceEnabled(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldCategories := DebugCategories
	defer func() {
		Level = oldLevel
		DebugCategories = oldCategories
	}()

	// Test: trace disabled at debug level
	Level = VerbosityDebug
	DebugCategories = nil
	if TraceEnabled(DebugConfig) {
		t.Error("expected trace disabled at debug level")
	}

	// Test: trace enabled at trace level
	Level = VerbosityTrace
	DebugCategories = nil
	if !TraceEnabled(DebugConfig) {
		t.Error("expected trace enabled at trace level")
	}

	// Test: specific category
	Level = VerbosityTrace
	DebugCategories = map[DebugCategory]bool{DebugConfig: true}
	if !TraceEnabled(DebugConfig) {
		t.Error("expected config category enabled")
	}
	if TraceEnabled(DebugNet) {
		t.Error("expected net category disabled")
	}
}

func TestDebugOutput(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldCategories := DebugCategories
	oldErrWriter := ErrWriter
	oldShowTimestamps := ShowTimestamps
	defer func() {
		Level = oldLevel
		DebugCategories = oldCategories
		ErrWriter = oldErrWriter
		ShowTimestamps = oldShowTimestamps
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityDebug
	DebugCategories = nil
	ShowTimestamps = false

	Debug(DebugConfig, "test message")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("expected [DEBUG] prefix, got %q", output)
	}
	if !strings.Contains(output, "config:") {
		t.Errorf("expected category in output, got %q", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected message in output, got %q", output)
	}
}

func TestDebugf(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldCategories := DebugCategories
	oldErrWriter := ErrWriter
	oldShowTimestamps := ShowTimestamps
	defer func() {
		Level = oldLevel
		DebugCategories = oldCategories
		ErrWriter = oldErrWriter
		ShowTimestamps = oldShowTimestamps
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityDebug
	DebugCategories = nil
	ShowTimestamps = false

	Debugf(DebugNet, "request to %s", "example.com")

	output := buf.String()
	if !strings.Contains(output, "request to example.com") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestTraceOutput(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldCategories := DebugCategories
	oldErrWriter := ErrWriter
	oldShowTimestamps := ShowTimestamps
	defer func() {
		Level = oldLevel
		DebugCategories = oldCategories
		ErrWriter = oldErrWriter
		ShowTimestamps = oldShowTimestamps
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityTrace
	DebugCategories = nil
	ShowTimestamps = false

	Trace(DebugFS, "reading file")

	output := buf.String()
	if !strings.Contains(output, "[TRACE]") {
		t.Errorf("expected [TRACE] prefix, got %q", output)
	}
	if !strings.Contains(output, "fs:") {
		t.Errorf("expected category in output, got %q", output)
	}
}

func TestTracef(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldCategories := DebugCategories
	oldErrWriter := ErrWriter
	oldShowTimestamps := ShowTimestamps
	defer func() {
		Level = oldLevel
		DebugCategories = oldCategories
		ErrWriter = oldErrWriter
		ShowTimestamps = oldShowTimestamps
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityTrace
	DebugCategories = nil
	ShowTimestamps = false

	Tracef(DebugExec, "running %s", "git status")

	output := buf.String()
	if !strings.Contains(output, "running git status") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestDebugWithTimestamps(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldCategories := DebugCategories
	oldErrWriter := ErrWriter
	oldShowTimestamps := ShowTimestamps
	defer func() {
		Level = oldLevel
		DebugCategories = oldCategories
		ErrWriter = oldErrWriter
		ShowTimestamps = oldShowTimestamps
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityDebug
	DebugCategories = nil
	ShowTimestamps = true

	Debug(DebugConfig, "test")

	output := buf.String()
	// Timestamp format: [DEBUG 14:23:01.123]
	if !strings.Contains(output, "[DEBUG ") {
		t.Errorf("expected timestamp in output, got %q", output)
	}
}

func TestDebugDisabled(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldErrWriter := ErrWriter
	defer func() {
		Level = oldLevel
		ErrWriter = oldErrWriter
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityNormal

	Debug(DebugConfig, "should not appear")

	if buf.Len() > 0 {
		t.Errorf("expected no output at normal level, got %q", buf.String())
	}
}

func TestItem(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	type TestItem struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Count  int    `json:"count"`
	}

	item := TestItem{Name: "test", Status: "active", Count: 42}

	// Test table format
	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatTable

	err := Item(item)
	if err != nil {
		t.Fatalf("Item() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FIELD") {
		t.Error("expected FIELD header in table output")
	}
	if !strings.Contains(output, "test") {
		t.Error("expected name value in output")
	}
}

func TestItem_JSON(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	type TestItem struct {
		Name string `json:"name"`
	}

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatJSON

	err := Item(TestItem{Name: "test"})
	if err != nil {
		t.Fatalf("Item() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"name"`) {
		t.Error("expected JSON output")
	}
}

func TestItem_Plain(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	type TestItem struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatPlain

	err := Item(TestItem{Name: "mysite", Status: "running"})
	if err != nil {
		t.Fatalf("Item() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "mysite") {
		t.Error("expected name in plain output")
	}
	if !strings.Contains(output, "status=running") {
		t.Error("expected status=running in plain output")
	}
}

func TestItem_Quiet(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	type TestItem struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatQuiet

	err := Item(TestItem{Name: "mysite", Status: "running"})
	if err != nil {
		t.Fatalf("Item() error = %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "mysite" {
		t.Errorf("expected only name in quiet output, got %q", output)
	}
}

func TestItem_NonStruct(t *testing.T) {
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

	err := Item("just a string")
	if err != nil {
		t.Fatalf("Item() error = %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "just a string" {
		t.Errorf("expected string value, got %q", output)
	}
}

func TestList_Plain(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	type Item struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatPlain

	items := []Item{
		{Name: "site1", Status: "running"},
		{Name: "site2", Status: "stopped"},
	}

	err := List(items)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "site1") {
		t.Error("expected site1 in plain output")
	}
	if !strings.Contains(output, "status=running") {
		t.Error("expected status=running in plain output")
	}
}

func TestList_Empty(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	type Item struct {
		Name string `json:"name"`
	}

	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatTable

	err := List([]Item{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if buf.Len() > 0 {
		t.Errorf("expected no output for empty list, got %q", buf.String())
	}
}

func TestList_InvalidType(t *testing.T) {
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

	err := List("not a slice")
	if err == nil {
		t.Error("expected error for non-slice input")
	}
}

func TestPrint(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityNormal

	Print("hello world")

	output := strings.TrimSpace(buf.String())
	if output != "hello world" {
		t.Errorf("Print() = %q, want %q", output, "hello world")
	}
}

func TestPrint_Quiet(t *testing.T) {
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

	Print("should not appear")

	if buf.Len() > 0 {
		t.Errorf("expected no output in quiet mode, got %q", buf.String())
	}
}

func TestPrintf(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityNormal

	Printf("hello %s", "world")

	output := strings.TrimSpace(buf.String())
	if output != "hello world" {
		t.Errorf("Printf() = %q, want %q", output, "hello world")
	}
}

func TestVerbose(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf

	// Test: not shown at normal level
	Level = VerbosityNormal
	Verbose("should not appear")
	if buf.Len() > 0 {
		t.Errorf("expected no output at normal level")
	}

	// Test: shown at verbose level
	buf.Reset()
	Level = VerbosityVerbose
	Verbose("visible message")
	if !strings.Contains(buf.String(), "visible message") {
		t.Errorf("expected message at verbose level")
	}
}

func TestVerbosef(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityVerbose

	Verbosef("count: %d", 42)

	if !strings.Contains(buf.String(), "count: 42") {
		t.Errorf("expected formatted message, got %q", buf.String())
	}
}

func TestEffectiveFormat(t *testing.T) {
	// Save and restore state
	oldFormat := CurrentFormat
	oldWriter := Writer
	defer func() {
		CurrentFormat = oldFormat
		Writer = oldWriter
	}()

	// Non-auto formats return themselves
	CurrentFormat = FormatJSON
	if EffectiveFormat() != FormatJSON {
		t.Error("expected JSON format")
	}

	CurrentFormat = FormatTable
	if EffectiveFormat() != FormatTable {
		t.Error("expected table format")
	}

	// Auto format with non-TTY returns JSON
	var buf bytes.Buffer
	Writer = &buf
	CurrentFormat = FormatAuto
	// buf is not a TTY, so should return JSON
	if EffectiveFormat() != FormatJSON {
		t.Errorf("expected JSON for non-TTY, got %v", EffectiveFormat())
	}
}

func TestWarning(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldFormat := CurrentFormat
	oldErrWriter := ErrWriter
	defer func() {
		Level = oldLevel
		CurrentFormat = oldFormat
		ErrWriter = oldErrWriter
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityNormal
	CurrentFormat = FormatTable

	Warning("caution needed")

	output := buf.String()
	if !strings.Contains(output, SymbolWarning) {
		t.Errorf("expected warning symbol, got %q", output)
	}
	if !strings.Contains(output, "caution needed") {
		t.Errorf("expected message, got %q", output)
	}
}

func TestError(t *testing.T) {
	// Save and restore state
	oldLevel := Level
	oldFormat := CurrentFormat
	oldErrWriter := ErrWriter
	defer func() {
		Level = oldLevel
		CurrentFormat = oldFormat
		ErrWriter = oldErrWriter
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityNormal
	CurrentFormat = FormatTable

	Error("something failed")

	output := buf.String()
	if !strings.Contains(output, SymbolError) {
		t.Errorf("expected error symbol, got %q", output)
	}
	if !strings.Contains(output, "something failed") {
		t.Errorf("expected message, got %q", output)
	}
}

func TestInfo(t *testing.T) {
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

	Info("informational message")

	output := buf.String()
	if !strings.Contains(output, SymbolInfo) {
		t.Errorf("expected info symbol, got %q", output)
	}
	if !strings.Contains(output, "informational message") {
		t.Errorf("expected message, got %q", output)
	}
}

func TestRedactString(t *testing.T) {
	// RedactString matches specific patterns:
	// - Bearer tokens: bearer [token]
	// - Basic auth: basic [base64]
	// - API key patterns: 16+ char key : 16+ char secret
	tests := []struct {
		input    string
		contains string
		desc     string
	}{
		{
			input:    `Authorization: Bearer abc123_token-test.value`,
			contains: "***",
			desc:     "bearer token should be redacted",
		},
		{
			input:    `Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=`,
			contains: "***",
			desc:     "basic auth should be redacted",
		},
		{
			input:    `abc1234567890123456:secret1234567890123`,
			contains: "***",
			desc:     "key:secret format (16+ chars each) should be redacted",
		},
		{
			input:    `no secrets here`,
			contains: "no secrets here",
			desc:     "non-matching strings should pass through",
		},
	}

	for _, tt := range tests {
		got := RedactString(tt.input)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("%s: RedactString(%q) = %q, want to contain %q", tt.desc, tt.input, got, tt.contains)
		}
	}
}

func TestSuccessf(t *testing.T) {
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

	Successf("completed %d items", 5)

	output := buf.String()
	if !strings.Contains(output, SymbolSuccess) {
		t.Errorf("expected success symbol, got %q", output)
	}
	if !strings.Contains(output, "completed 5 items") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestErrorf(t *testing.T) {
	oldErrWriter := ErrWriter
	defer func() {
		ErrWriter = oldErrWriter
	}()

	var buf bytes.Buffer
	ErrWriter = &buf

	Errorf("failed with code %d", 500)

	output := buf.String()
	if !strings.Contains(output, SymbolError) {
		t.Errorf("expected error symbol, got %q", output)
	}
	if !strings.Contains(output, "failed with code 500") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestWarningf(t *testing.T) {
	oldLevel := Level
	oldErrWriter := ErrWriter
	defer func() {
		Level = oldLevel
		ErrWriter = oldErrWriter
	}()

	var buf bytes.Buffer
	ErrWriter = &buf
	Level = VerbosityNormal

	Warningf("retrying in %d seconds", 30)

	output := buf.String()
	if !strings.Contains(output, SymbolWarning) {
		t.Errorf("expected warning symbol, got %q", output)
	}
	if !strings.Contains(output, "retrying in 30 seconds") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestInfof(t *testing.T) {
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

	Infof("using config from %s", "/home/user/.weg")

	output := buf.String()
	if !strings.Contains(output, SymbolInfo) {
		t.Errorf("expected info symbol, got %q", output)
	}
	if !strings.Contains(output, "using config from /home/user/.weg") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestStatusf(t *testing.T) {
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

	Statusf(SymbolActive, "processing %d of %d", 3, 10)

	output := buf.String()
	if !strings.Contains(output, SymbolActive) {
		t.Errorf("expected active symbol, got %q", output)
	}
	if !strings.Contains(output, "processing 3 of 10") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestStep(t *testing.T) {
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

	Step(2, 5, "Installing dependencies")

	output := buf.String()
	if !strings.Contains(output, "[2/5]") {
		t.Errorf("expected step number, got %q", output)
	}
	if !strings.Contains(output, "Installing dependencies") {
		t.Errorf("expected message, got %q", output)
	}
}

func TestStepf(t *testing.T) {
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

	Stepf(1, 3, "Building %s", "app.go")

	output := buf.String()
	if !strings.Contains(output, "[1/3]") {
		t.Errorf("expected step number, got %q", output)
	}
	if !strings.Contains(output, "Building app.go") {
		t.Errorf("expected formatted message, got %q", output)
	}
}

func TestRedactHeaders(t *testing.T) {
	headers := http.Header{
		"Authorization": []string{"Bearer token123456789012345"},
		"Content-Type":  []string{"application/json"},
		"X-Api-Key":     []string{"secret-api-key-12345678901"},
		"X-Request-Id":  []string{"abc123"},
		"Cookie":        []string{"session=secretsessionvalue12345"},
	}

	redacted := RedactHeaders(headers)

	// Non-sensitive headers should be unchanged
	if redacted.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type should be unchanged")
	}
	if redacted.Get("X-Request-Id") != "abc123" {
		t.Errorf("X-Request-Id should be unchanged")
	}

	// Sensitive headers should be redacted (contain ***)
	if !strings.Contains(redacted.Get("Authorization"), "***") {
		t.Errorf("Authorization should be redacted, got %q", redacted.Get("Authorization"))
	}
	if !strings.Contains(redacted.Get("X-Api-Key"), "***") {
		t.Errorf("X-Api-Key should be redacted, got %q", redacted.Get("X-Api-Key"))
	}
	if !strings.Contains(redacted.Get("Cookie"), "***") {
		t.Errorf("Cookie should be redacted, got %q", redacted.Get("Cookie"))
	}
}

func TestJSONCompact(t *testing.T) {
	oldWriter := Writer
	defer func() {
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf

	data := map[string]string{"name": "test", "value": "123"}
	JSONCompact(data)

	output := buf.String()
	// Compact JSON should not have newlines or extra spaces
	if strings.Contains(output, "  ") {
		t.Errorf("expected compact JSON without extra spaces, got %q", output)
	}
	if !strings.Contains(output, `"name":"test"`) && !strings.Contains(output, `"name": "test"`) {
		t.Errorf("expected JSON content, got %q", output)
	}
}

func TestJSONCompactTo(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]int{"count": 42}
	JSONCompactTo(&buf, data)

	output := buf.String()
	if !strings.Contains(output, "42") {
		t.Errorf("expected JSON content, got %q", output)
	}
}

func TestWithTiming(t *testing.T) {
	oldLevel := Level
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityDebug

	done := WithTiming(DebugNet, "test operation")
	done()

	// WithTiming uses Debug internally, just verify it doesn't panic
}

func TestStatus_QuietMode(t *testing.T) {
	oldLevel := Level
	oldWriter := Writer
	defer func() {
		Level = oldLevel
		Writer = oldWriter
	}()

	var buf bytes.Buffer
	Writer = &buf
	Level = VerbosityQuiet

	Status(SymbolSuccess, "test message")

	// Should be empty in quiet mode
	if buf.String() != "" {
		t.Errorf("expected no output in quiet mode, got %q", buf.String())
	}
}

func TestStatus_JSONMode(t *testing.T) {
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

	Status(SymbolSuccess, "test message")

	// Should be empty in JSON mode (status doesn't go to JSON output)
	if buf.String() != "" {
		t.Errorf("expected no output in JSON mode, got %q", buf.String())
	}
}
