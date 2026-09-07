package output

import (
	"strings"
	"testing"
)

func TestDiff_IdenticalContent(t *testing.T) {
	content := "line1\nline2\nline3\n"
	got := Diff(content, content)
	if got != " line1\n line2\n line3\n" {
		t.Errorf("expected all-context diff, got:\n%s", got)
	}
}

func TestDiff_AddedLines(t *testing.T) {
	oldContent := "a\nb\n"
	newContent := "a\nb\nc\n"
	got := Diff(oldContent, newContent)
	want := " a\n b\n+c\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDiff_RemovedLines(t *testing.T) {
	oldContent := "a\nb\nc\n"
	newContent := "a\nc\n"
	got := Diff(oldContent, newContent)
	want := " a\n-b\n c\n"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDiff_ReplacedLines(t *testing.T) {
	oldContent := "x\n"
	newContent := "y\n"
	got := Diff(oldContent, newContent)
	if !strings.HasPrefix(got, "-x\n+") && !strings.HasPrefix(got, "+y\n-") {
		t.Errorf("expected replace to show -x and +y, got:\n%s", got)
	}
}

func TestDiff_EmptyContent(t *testing.T) {
	if got := Diff("", ""); got != "" {
		t.Errorf("expected empty diff for empty inputs, got %q", got)
	}
	got := Diff("", "hello\n")
	if got != "+hello\n" {
		t.Errorf("expected single added line, got %q", got)
	}
}

func TestDiff_NoTrailingNewline(t *testing.T) {
	if got := Diff("a", "a"); got != " a\n" {
		t.Errorf("expected context line for no-trailing-newline match, got %q", got)
	}
}

func TestDiffEmpty(t *testing.T) {
	if !DiffEmpty("same", "same") {
		t.Error("DiffEmpty should be true for identical content")
	}
	if DiffEmpty("same", "different") {
		t.Error("DiffEmpty should be false for different content")
	}
}
