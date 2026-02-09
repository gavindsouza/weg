package output

import (
	"bytes"
	"testing"
)

// CaptureForTest redirects output to a buffer and restores all globals on test cleanup.
// Returns the capture buffer for assertions.
func CaptureForTest(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}

	// Save current state
	savedFormat := CurrentFormat
	savedLevel := Level
	savedCategories := DebugCategories
	savedNoColor := NoColor
	savedTimestamps := ShowTimestamps
	savedWriter := Writer
	savedErrWriter := ErrWriter

	// Set test state
	Writer = buf
	ErrWriter = buf
	CurrentFormat = FormatPlain
	Level = VerbosityNormal

	t.Cleanup(func() {
		CurrentFormat = savedFormat
		Level = savedLevel
		DebugCategories = savedCategories
		NoColor = savedNoColor
		ShowTimestamps = savedTimestamps
		Writer = savedWriter
		ErrWriter = savedErrWriter
	})

	return buf
}

// SaveForTest saves all output globals and restores them on test cleanup.
// Unlike CaptureForTest, this does not redirect output — use it when you
// need to modify globals yourself (e.g. testing configureOutput).
func SaveForTest(t *testing.T) {
	t.Helper()

	savedFormat := CurrentFormat
	savedLevel := Level
	savedCategories := DebugCategories
	savedNoColor := NoColor
	savedTimestamps := ShowTimestamps
	savedWriter := Writer
	savedErrWriter := ErrWriter

	t.Cleanup(func() {
		CurrentFormat = savedFormat
		Level = savedLevel
		DebugCategories = savedCategories
		NoColor = savedNoColor
		ShowTimestamps = savedTimestamps
		Writer = savedWriter
		ErrWriter = savedErrWriter
	})
}
