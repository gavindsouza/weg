/*
Copyright © 2025 Gavin <me@gavv.in>

Core entity types for syncing customizations from remote Frappe sites.
*/
package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EntityType represents a type of syncable entity
type EntityType string

const (
	EntityDocType        EntityType = "doctype"
	EntityCustomField    EntityType = "custom_field"
	EntityPropertySetter EntityType = "property_setter"
	EntityClientScript   EntityType = "client_script"
	EntityServerScript   EntityType = "server_script"
	EntityReport         EntityType = "report"
	EntityPrintFormat    EntityType = "print_format"
	EntityWorkflow       EntityType = "workflow"
	EntityNotification   EntityType = "notification"
	EntityWorkspace      EntityType = "workspace"
	EntityLetterHead     EntityType = "letter_head"
	EntityWebTemplate    EntityType = "web_template"
	EntityNumberCard     EntityType = "number_card"
	EntityDashboard      EntityType = "dashboard"
	EntityDashboardChart EntityType = "dashboard_chart"
)

// Entity represents a fetched entity with metadata
type Entity struct {
	Type     EntityType
	Name     string
	Module   string
	Data     map[string]interface{}
	FilePath string // Relative path where this should be saved
}

// FetchResult contains the results of fetching all entities
type FetchResult struct {
	Entities  []Entity
	Modules   map[string]ModuleInfo
	FrappeVer string
	Apps      map[string]AppInfo
}

// WriteEntity writes an entity to disk
func WriteEntity(baseDir string, entity Entity) error {
	fullPath := filepath.Join(baseDir, entity.FilePath)

	// Create directory
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(entity.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	return nil
}

// getString extracts a string value from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// toSnakeCase converts a string to snake_case
func toSnakeCase(s string) string {
	// Simple conversion: lowercase and replace spaces with underscores
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
