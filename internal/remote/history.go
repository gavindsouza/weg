/*
Copyright © 2025 Gavin <me@gavv.in>

History fetching for tracking entity version history from Frappe sites.
*/
package remote

import (
	"path/filepath"
)

// HistoryEntry represents a single change in the version history
type HistoryEntry struct {
	Timestamp   string // Creation timestamp from Version
	Author      string // Who made the change (owner field)
	DocType     string // The doctype that was changed
	DocName     string // The document name
	Action      string // "create", "update", "delete"
	VersionData string // Raw version data JSON for generating commit messages
	VersionName string // Name of the Version document (for traceability)
	EntityType  EntityType
	Module      string
	FilePath    string
	EntityData  map[string]any // Current entity data for metadata extraction
}

// DoctypeToEntityType maps Frappe doctype names to our entity types
var DoctypeToEntityType = map[string]EntityType{
	"DocType":         EntityDocType,
	"Custom Field":    EntityCustomField,
	"Property Setter": EntityPropertySetter,
	"Client Script":   EntityClientScript,
	"Server Script":   EntityServerScript,
	"Report":          EntityReport,
	"Print Format":    EntityPrintFormat,
	"Workflow":        EntityWorkflow,
	"Notification":    EntityNotification,
	"Letter Head":     EntityLetterHead,
}

// VersionedDoctypes returns the list of doctypes we track versions for
func VersionedDoctypes() []string {
	return []string{
		"DocType",
		"Custom Field",
		"Property Setter",
		"Client Script",
		"Server Script",
		"Report",
		"Print Format",
		"Workflow",
		"Notification",
		"Letter Head",
	}
}

// buildFilePath constructs the file path for an entity
func buildFilePath(entityType EntityType, name, module string) string {
	switch entityType {
	case EntityDocType:
		return filepath.Join(toSnakeCase(module), "doctype", toSnakeCase(name), toSnakeCase(name)+".json")
	case EntityCustomField:
		return filepath.Join(toSnakeCase(module), "custom_field", toSnakeCase(name)+".json")
	case EntityPropertySetter:
		return filepath.Join(toSnakeCase(module), "property_setter", toSnakeCase(name)+".json")
	case EntityReport:
		return filepath.Join(toSnakeCase(module), "report", toSnakeCase(name), toSnakeCase(name)+".json")
	default:
		return filepath.Join(toSnakeCase(module), string(entityType), toSnakeCase(name)+".json")
	}
}
