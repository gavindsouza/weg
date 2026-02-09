package cmd

import (
	"os"
	"testing"

	"github.com/gavindsouza/weg/internal/output"
)

// saveOutputState captures global state that configureOutput modifies.
func saveOutputState(t *testing.T) {
	t.Helper()
	// Save output package globals via shared helper
	output.SaveForTest(t)

	// Save cmd-local globals
	origVerboseCount := verboseCount
	origLogLevel := logLevel
	origQuiet := quiet
	origVerbose := verbose
	origOutputFormat := outputFormat
	origDebugCategories := debugCategories

	t.Cleanup(func() {
		verboseCount = origVerboseCount
		logLevel = origLogLevel
		quiet = origQuiet
		verbose = origVerbose
		outputFormat = origOutputFormat
		debugCategories = origDebugCategories
	})
}

func TestConfigureOutput_DefaultNoFlags(t *testing.T) {
	saveOutputState(t)

	verboseCount = 0
	logLevel = ""
	quiet = false
	outputFormat = "auto"
	debugCategories = ""
	os.Unsetenv("WEG_LOG_LEVEL")

	if err := configureOutput(); err != nil {
		t.Fatalf("configureOutput() error: %v", err)
	}

	if output.Level != output.VerbosityNormal {
		t.Errorf("Level = %v, want VerbosityNormal", output.Level)
	}
	if output.CurrentFormat != output.FormatAuto {
		t.Errorf("CurrentFormat = %v, want FormatAuto", output.CurrentFormat)
	}
	if verbose {
		t.Error("verbose should be false")
	}
}

func TestConfigureOutput_VerboseEscalation(t *testing.T) {
	tests := []struct {
		name        string
		count       int
		wantLevel   output.Verbosity
		wantVerbose bool
	}{
		{"v", 1, output.VerbosityVerbose, true},
		{"vv", 2, output.VerbosityDebug, true},
		{"vvv", 3, output.VerbosityTrace, true},
		{"vvvv caps at trace", 4, output.VerbosityTrace, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveOutputState(t)

			verboseCount = tt.count
			logLevel = ""
			quiet = false
			outputFormat = "auto"
			debugCategories = ""
			os.Unsetenv("WEG_LOG_LEVEL")

			if err := configureOutput(); err != nil {
				t.Fatalf("configureOutput() error: %v", err)
			}

			if output.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", output.Level, tt.wantLevel)
			}
			if verbose != tt.wantVerbose {
				t.Errorf("verbose = %v, want %v", verbose, tt.wantVerbose)
			}
		})
	}
}

func TestConfigureOutput_QuietOverridesVerbose(t *testing.T) {
	saveOutputState(t)

	verboseCount = 3
	logLevel = ""
	quiet = true
	outputFormat = "auto"
	debugCategories = ""

	if err := configureOutput(); err != nil {
		t.Fatalf("configureOutput() error: %v", err)
	}

	if output.Level != output.VerbosityQuiet {
		t.Errorf("Level = %v, want VerbosityQuiet (quiet should win over -vvv)", output.Level)
	}
}

func TestConfigureOutput_LogLevelOverridesAll(t *testing.T) {
	saveOutputState(t)

	verboseCount = 3 // -vvv
	logLevel = "debug"
	quiet = true // --quiet
	outputFormat = "auto"
	debugCategories = ""

	if err := configureOutput(); err != nil {
		t.Fatalf("configureOutput() error: %v", err)
	}

	// --log-level has highest precedence
	if output.Level != output.VerbosityDebug {
		t.Errorf("Level = %v, want VerbosityDebug (--log-level should win)", output.Level)
	}
}

func TestConfigureOutput_LogLevelTrace(t *testing.T) {
	saveOutputState(t)

	verboseCount = 0
	logLevel = "trace"
	quiet = false
	outputFormat = "auto"
	debugCategories = ""

	if err := configureOutput(); err != nil {
		t.Fatalf("configureOutput() error: %v", err)
	}

	if output.Level != output.VerbosityTrace {
		t.Errorf("Level = %v, want VerbosityTrace", output.Level)
	}
}

func TestConfigureOutput_EnvVarLowestPrecedence(t *testing.T) {
	saveOutputState(t)

	verboseCount = 0
	logLevel = ""
	quiet = false
	outputFormat = "auto"
	debugCategories = ""

	t.Setenv("WEG_LOG_LEVEL", "debug")

	if err := configureOutput(); err != nil {
		t.Fatalf("configureOutput() error: %v", err)
	}

	// Env var should apply when no flags set
	if output.Level != output.VerbosityDebug {
		t.Errorf("Level = %v, want VerbosityDebug (env var should apply)", output.Level)
	}
}

func TestConfigureOutput_FlagOverridesEnvVar(t *testing.T) {
	saveOutputState(t)

	verboseCount = 0
	logLevel = ""
	quiet = true // flag set
	outputFormat = "auto"
	debugCategories = ""

	t.Setenv("WEG_LOG_LEVEL", "debug")

	if err := configureOutput(); err != nil {
		t.Fatalf("configureOutput() error: %v", err)
	}

	// -q flag should override env var
	if output.Level != output.VerbosityQuiet {
		t.Errorf("Level = %v, want VerbosityQuiet (flag should override env)", output.Level)
	}
}

func TestConfigureOutput_InvalidFormat(t *testing.T) {
	saveOutputState(t)

	outputFormat = "invalid"
	logLevel = ""
	quiet = false
	verboseCount = 0

	if err := configureOutput(); err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestConfigureOutput_InvalidLogLevel(t *testing.T) {
	saveOutputState(t)

	outputFormat = "auto"
	logLevel = "invalid"
	quiet = false
	verboseCount = 0

	if err := configureOutput(); err == nil {
		t.Error("expected error for invalid log level")
	}
}
