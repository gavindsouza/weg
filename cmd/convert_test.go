package cmd

import (
	"strings"
	"testing"

	"github.com/gavindsouza/weg/internal/output"
	"github.com/gavindsouza/weg/internal/testutil"
)

func TestConvertCommand_ArgValidation(t *testing.T) {
	// Exactly one positional arg is required.
	if err := convertCmd.Args(convertCmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := convertCmd.Args(convertCmd, []string{"app", "bench"}); err == nil {
		t.Error("expected error for two args")
	}
	if err := convertCmd.Args(convertCmd, []string{"app"}); err != nil {
		t.Errorf("one arg should be valid: %v", err)
	}
}

func TestRunConvert_RejectsUnknownMode(t *testing.T) {
	output.CaptureForTest(t)

	err := runConvert(convertCmd, []string{"sideways"})
	if err == nil {
		t.Fatal("expected validation error for unknown mode")
	}
	if !strings.Contains(err.Error(), "must be 'app' or 'bench'") {
		t.Errorf("expected mode validation message, got: %v", err)
	}
}

// Mode is case-insensitive; "BENCH" in a fresh dir must fail on context, not
// on mode validation.
func TestRunConvert_ModeCaseInsensitive(t *testing.T) {
	t.Chdir(t.TempDir())
	output.CaptureForTest(t)

	err := runConvert(convertCmd, []string{"BENCH"})
	if err == nil {
		t.Fatal("expected error in fresh directory")
	}
	if strings.Contains(err.Error(), "must be 'app' or 'bench'") {
		t.Errorf("uppercase mode should pass validation, got: %v", err)
	}
}

// Converting in a directory that is neither app- nor bench-centric must fail
// with a usage error naming the required source mode.
func TestRunConvert_FreshDirRejected(t *testing.T) {
	cases := []struct {
		mode    string
		wantMsg string
	}{
		{"bench", "can only convert from app-centric mode"},
		{"app", "can only convert from bench-centric mode"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Chdir(t.TempDir())
			output.CaptureForTest(t)

			err := runConvert(convertCmd, []string{tc.mode})
			if err == nil {
				t.Fatal("expected error in fresh directory")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantMsg)
			}
		})
	}
}

// Converting to the mode the project is already in is a friendly no-op.
func TestRunConvert_AlreadyBenchCentric(t *testing.T) {
	bench := testutil.NewBench(t).WithApp("frappe").Build()
	t.Chdir(bench)

	buf := output.CaptureForTest(t)

	if err := runConvert(convertCmd, []string{"bench"}); err != nil {
		t.Fatalf("expected no-op, got error: %v", err)
	}
	if !strings.Contains(buf.String(), "Already in bench-centric mode") {
		t.Errorf("expected no-op message, got: %s", buf.String())
	}
}
