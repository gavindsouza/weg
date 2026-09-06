package cmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// frappeDevelopMajor is the major version the develop branch currently
// tracks. Advance it when the frappe nightlies bump majors.
const frappeDevelopMajor = 17

// frappeDependencyRange maps a set of supported Frappe versions to a
// bounded dependency range for [tool.bench.frappe-dependencies], which
// Frappe Cloud validates on installs and bench updates. The lower bound is
// the lowest supported major; the upper bound is one past the highest one,
// with develop treated as the current development major.
func frappeDependencyRange(versions []string) string {
	major := func(v string) int {
		if v == "develop" {
			return frappeDevelopMajor
		}
		cleaned := strings.TrimPrefix(strings.TrimPrefix(v, "version-"), "v")
		n, err := strconv.Atoi(cleaned)
		if err != nil {
			return 0
		}
		return n
	}

	lowest, highest := math.MaxInt, 0
	for _, v := range versions {
		m := major(v)
		if m > highest {
			highest = m
		}
		if v != "develop" && m < lowest {
			lowest = m
		}
	}
	if lowest == math.MaxInt {
		lowest = highest
	}
	if highest == 0 {
		return ""
	}
	return fmt.Sprintf(">=%d.0.0,<%d.0.0", lowest, highest+1)
}

// benchDependencySection renders the [tool.bench.frappe-dependencies] table
// that FC requires for app version constraints.
func benchDependencySection(versions []string) string {
	return fmt.Sprintf(`# Frappe Cloud version constraints
[tool.bench.frappe-dependencies]
frappe = %q`, frappeDependencyRange(versions))
}

// hasBenchDependencies reports whether pyproject.toml already declares a
// frappe constraint, so manual overrides are never clobbered.
func hasBenchDependencies(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "[tool.bench.frappe-dependencies]" {
			return true
		}
	}
	return false
}

// buildWegSection renders the [tool.weg] block that gets merged into
// pyproject.toml. The block mirrors what 'weg new' and ParsePyproject produce.
func buildWegSection(plan *bootstrapPlan) string {
	return fmt.Sprintf(`[tool.weg]
# Compatibility - which Frappe versions does this app support?
[tool.weg.compatibility]
frappe = [%s]
databases = [%s]

# Development environment settings
[tool.weg.dev]
frappe = %q
database = %q
`, quotedList(plan.versions), quotedList(plan.databases), plan.devVersion, plan.devDB)
}

// findWegSection extracts the [tool.weg] section (including sub-tables) from
// pyproject.toml content. Returns "" when absent.
func findWegSection(content string) string {
	start, _ := wegSectionSpan(content)
	if start == -1 {
		return ""
	}
	lines := strings.Split(content, "\n")
	end := wegSectionEnd(lines, start)
	return strings.Join(lines[start:end], "\n")
}

// addWegSection appends the section to the end of the file.
func addWegSection(content, section string) string {
	if strings.TrimSpace(content) == "" {
		return strings.TrimRight(section, "\n") + "\n"
	}
	return strings.TrimRight(content, "\n") + "\n\n" + strings.TrimRight(section, "\n") + "\n"
}

// replaceWegSection swaps the existing [tool.weg] block for a new one.
func replaceWegSection(content, section string) string {
	start, end := wegSectionSpan(content)
	if start == -1 {
		return addWegSection(content, section)
	}
	lines := strings.Split(content, "\n")
	kept := append([]string{}, lines[:start]...)
	kept = append(kept, lines[end:]...)
	return addWegSection(strings.Join(kept, "\n"), section)
}

func wegSectionSpan(content string) (start, end int) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "[tool.weg]" {
			return i, wegSectionEnd(lines, i)
		}
	}
	return -1, -1
}

func wegSectionEnd(lines []string, start int) int {
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[tool.weg") {
			return i
		}
	}
	return len(lines)
}

func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, v := range items {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(quoted, ", ")
}