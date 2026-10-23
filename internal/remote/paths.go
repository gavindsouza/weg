/*
Copyright © 2025 Gavin <me@gavv.in>

Export file paths: built from site-controlled names, so every path is made of
safe segments and checked to stay inside the export directory before weg
writes or removes it.
*/
package remote

import (
	"fmt"
	"path/filepath"
	"strings"
)

// pathSegment turns a module or document name into a single path segment:
// snake_cased, with path separators replaced and leading dots escaped, so a
// name such as "../../x" or ".git" cannot climb out of its directory or land
// in a hidden one. An empty name becomes "_", the catch-all module.
func pathSegment(name string) string {
	s := strings.NewReplacer("/", "_", "\\", "_", "\x00", "_").Replace(toSnakeCase(name))
	if trimmed := strings.TrimLeft(s, "."); len(trimmed) != len(s) {
		s = strings.Repeat("_", len(s)-len(trimmed)) + trimmed
	}
	if s == "" {
		return "_"
	}
	return s
}

// buildFilePath constructs the file path for an entity: <module>/<type>/<name>.json,
// or <module>/<type>/<name>/<name>.json for DocTypes and Reports.
func buildFilePath(entityType EntityType, name, module string) string {
	mod, file := pathSegment(module), pathSegment(name)
	switch entityType {
	case EntityDocType, EntityReport:
		return filepath.Join(mod, string(entityType), file, file+".json")
	default:
		return filepath.Join(mod, string(entityType), file+".json")
	}
}

// CheckExportPath returns an error unless rel is a clean relative path that
// stays inside the export directory and does not start in a hidden directory
// (.git, .weg). Every export file weg writes or removes passes this check.
func CheckExportPath(rel string) error {
	if !filepath.IsLocal(rel) || filepath.Clean(rel) != rel {
		return fmt.Errorf("refusing export path %q: it resolves outside the export directory", rel)
	}
	if first, _, _ := strings.Cut(filepath.ToSlash(rel), "/"); strings.HasPrefix(first, ".") {
		return fmt.Errorf("refusing export path %q: it is inside a hidden directory", rel)
	}
	return nil
}

// ExportFilePath joins rel to baseDir after CheckExportPath accepts it.
func ExportFilePath(baseDir, rel string) (string, error) {
	if err := CheckExportPath(rel); err != nil {
		return "", err
	}
	return filepath.Join(baseDir, rel), nil
}

// CheckEntityPaths returns an error naming every entity whose file path
// CheckExportPath refuses, so a fetch with an unsafe path fails before
// anything is written.
func CheckEntityPaths(entities []Entity) error {
	var bad []string
	for _, e := range entities {
		if err := CheckExportPath(e.FilePath); err != nil {
			bad = append(bad, fmt.Sprintf("%s %q: %v", e.Type, e.Name, err))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%d entities have unsafe file paths:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
	return nil
}
