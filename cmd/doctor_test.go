package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/testutil"
)

// doctorReport mirrors the JSON shape emitted by doctorJSON.
type doctorReport struct {
	Checks []struct {
		Name    string `json:"name"`
		OK      bool   `json:"ok"`
		Message string `json:"message,omitempty"`
	} `json:"checks"`
	Issues int `json:"issues"`
}

// In a directory that is not a weg project, JSON mode must still emit a
// machine-readable report (one failing check) and return a non-zero error.
func TestRunDoctor_JSON_NotAWegProject(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	buf := output.CaptureForTest(t)
	output.CurrentFormat = output.FormatJSON

	err := runDoctor(doctorCmd, nil)
	if err == nil {
		t.Fatal("expected error outside a weg project")
	}

	var report doctorReport
	if jsonErr := json.Unmarshal(buf.Bytes(), &report); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", jsonErr, buf.String())
	}
	if len(report.Checks) != 1 || report.Checks[0].OK {
		t.Fatalf("expected a single failing check, got %+v", report.Checks)
	}
	if report.Issues != 1 {
		t.Errorf("issues = %d, want 1", report.Issues)
	}
}

// In a weg bench, JSON mode emits the full check list with a consistent shape:
// every check has a name, the issue count matches the failing checks, and the
// returned error carries the count so scripts get a non-zero exit code.
func TestRunDoctor_JSON_BenchShapeAndExitError(t *testing.T) {
	bench := testutil.NewBench(t).
		WithApp("frappe").
		WithSite("test.localhost").
		Build()
	t.Chdir(bench)

	buf := output.CaptureForTest(t)
	output.CurrentFormat = output.FormatJSON

	err := runDoctor(doctorCmd, nil)
	// A bare test bench has no devbox/venv/services, so checks must fail.
	if err == nil {
		t.Fatal("expected non-nil error when checks fail")
	}
	if !strings.Contains(err.Error(), "issue(s) found") {
		t.Errorf("error should report issue count, got: %v", err)
	}

	var report doctorReport
	if jsonErr := json.Unmarshal(buf.Bytes(), &report); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", jsonErr, buf.String())
	}

	if len(report.Checks) != 11 {
		t.Errorf("expected 11 checks, got %d", len(report.Checks))
	}

	failing := 0
	names := make(map[string]bool)
	for _, c := range report.Checks {
		if c.Name == "" {
			t.Error("check with empty name")
		}
		names[c.Name] = true
		if !c.OK {
			failing++
		}
	}
	if report.Issues != failing {
		t.Errorf("issues = %d, but %d checks failed", report.Issues, failing)
	}
	if report.Issues == 0 {
		t.Error("expected at least one failing check on a bare bench")
	}

	// weg.toml exists in the test bench, so that check must pass.
	for _, c := range report.Checks {
		if c.Name == "weg.toml" && !c.OK {
			t.Errorf("weg.toml check failed unexpectedly: %s", c.Message)
		}
	}
	for _, want := range []string{"Devbox", "weg.toml", "Directories", "Apps", "Sites", "Services"} {
		if !names[want] {
			t.Errorf("check %q missing from report", want)
		}
	}
}

// Text mode prints checkbox lines and still returns the exit-code error.
func TestRunDoctor_TextMode(t *testing.T) {
	bench := testutil.NewBench(t).WithApp("frappe").Build()
	t.Chdir(bench)

	buf := output.CaptureForTest(t)

	err := runDoctor(doctorCmd, nil)
	if err == nil {
		t.Fatal("expected non-nil error when checks fail")
	}

	out := buf.String()
	if !strings.Contains(out, "Weg Doctor") {
		t.Errorf("missing header, got: %s", out)
	}
	if !strings.Contains(out, "[x] weg.toml") {
		t.Errorf("expected passing weg.toml check, got: %s", out)
	}
	if !strings.Contains(out, "issue(s) found.") {
		t.Errorf("expected issue summary, got: %s", out)
	}
}
