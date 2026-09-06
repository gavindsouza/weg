package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gavindsouza/weg/tools"
)

var (
	// ciInlineVersionRe matches matrix include rows that pin a Frappe version
	// inline, e.g. `- {version: "15", ...}` or `- version: version-15`.
	ciInlineVersionRe = regexp.MustCompile(`(?m)^\s*-\s*\{?\s*(?:frappe[_-]?version|version|frappe[_-]?branch)\s*:\s*["']?([^,"\s}]+)`)
	// ciStandaloneVersionRe matches a bare version key on its own line,
	// e.g. `frappe_version: version-16` or `version: [version-15, develop]`.
	ciStandaloneVersionRe = regexp.MustCompile(`(?m)^\s*(?:frappe[_-]?version|version|frappe[_-]?branch)\s*:\s*(.+)`)
	// ciPythonVersionRe matches python-version declarations that carry a
	// literal runtime (matrix rows, setup-python with: blocks). Matrix variable
	// references like ${{ matrix.python-version }} don't match.
	ciPythonVersionRe = regexp.MustCompile(`python-version:\s*["']?(\d+\.\d+)`)
	// ciDBServiceRe matches database services inside a job's services block.
	ciDBServiceRe = regexp.MustCompile(`(?m)^\s{2,8}(mariadb|postgres|sqlite):`)
	// ciVersionTokenRe matches version tokens inside matrix values, covering
	// "15", "v15", "version-15", "15.0.1", and "develop".
	ciVersionTokenRe = regexp.MustCompile(`(?:version-|v)?(14|15|16)(?:\.\d+)*|develop`)

	validDetectableVersions = []string{"14", "15", "16", "develop"}
)

// pythonToFrappeVersion maps the default Python runtime for each Frappe version
// back to the version itself, letting us infer compatibility from CI matrices
// that declare python-version rows.
var pythonToFrappeVersion = map[string]string{
	"3.10": "14",
	"3.11": "15",
	"3.12": "16",
	"3.13": "develop",
}

// detectCICompatibility inspects an existing .github/workflows directory for
// Frappe version and database compatibility signals. Both return values are
// nil when nothing usable is found.
func detectCICompatibility(repoPath string) (versions []string, databases []string) {
	workflowsDir := filepath.Join(repoPath, ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return nil, nil
	}

	detected := map[string]bool{}
	dbsSeen := map[string]bool{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(workflowsDir, name))
		if err != nil {
			continue
		}
		content := string(data)

		// Inline matrix rows and bare version keys.
		for _, m := range ciInlineVersionRe.FindAllStringSubmatch(content, -1) {
			for _, v := range ciVersionsFromValue(m[1]) {
				detected[v] = true
			}
		}
		for _, m := range ciStandaloneVersionRe.FindAllStringSubmatch(content, -1) {
			for _, v := range ciVersionsFromValue(m[1]) {
				detected[v] = true
			}
		}
		// python-version rows mapped through the runtime table.
		for _, m := range ciPythonVersionRe.FindAllStringSubmatch(content, -1) {
			if fv, ok := pythonToFrappeVersion[m[1]]; ok {
				detected[fv] = true
			}
		}

		// Database services.
		for _, m := range ciDBServiceRe.FindAllStringSubmatch(content, -1) {
			dbsSeen[m[1]] = true
		}
	}

	if len(detected) == 0 && len(dbsSeen) == 0 {
		return nil, nil
	}

	versions = nil
	for _, v := range validDetectableVersions {
		if detected[v] {
			versions = append(versions, v)
		}
	}

	databases = nil
	for db := range dbsSeen {
		databases = append(databases, db)
	}
	sort.Strings(databases)

	return versions, databases
}

// ciVersionsFromValue maps a CI matrix value to Frappe version short forms.
// Handles a scalar ("15", "v15", "version-15", "15.0.1"), a list
// ("[version-15, version-16, develop]"), and comma-joined values.
func ciVersionsFromValue(value string) []string {
	var out []string
	value = strings.TrimSpace(value)
	for _, m := range ciVersionTokenRe.FindAllString(value, -1) {
		if m == "develop" {
			out = append(out, "develop")
			continue
		}
		major := strings.Split(strings.TrimPrefix(strings.TrimPrefix(m, "version-"), "v"), ".")[0]
		out = append(out, major)
	}
	return out
}

// buildMatrixInclude renders the strategy.matrix.include block for the CI
// workflow template from a list of compatible Frappe versions.
func buildMatrixInclude(versions []string) (string, error) {
	var out strings.Builder
	for _, v := range versions {
		fv, err := tools.GetFrappeVersion(tools.NormalizeFrappeVersion(v))
		if err != nil {
			return "", fmt.Errorf("unsupported Frappe version %q: %w", v, err)
		}
		branch := "version-" + v
		if v == "develop" {
			branch = "develop"
		}
		out.WriteString(fmt.Sprintf("          - {version: %q, python-version: %q, node-version: %q, frappe-branch: %q}\n",
			v, fv.PythonVersion, fv.NodeVersion, branch))
	}
	return out.String(), nil
}

// frappeBranch returns the git branch name for a Frappe version.
func frappeBranch(version string) string {
	if version == "develop" {
		return "develop"
	}
	return "version-" + version
}