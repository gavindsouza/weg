/*
Copyright © 2025 Gavin <me@gavv.in>

History fetching for tracking entity version history from Frappe sites.
*/
package remote

import (
	"encoding/json"
	"fmt"
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
	EntityData  map[string]interface{} // Current entity data for metadata extraction
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

// FetchHistory fetches version history for all entities
func (f *Fetcher) FetchHistory() ([]HistoryEntry, error) {
	versions, err := f.Client.GetAllVersions(VersionedDoctypes())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}

	var history []HistoryEntry

	for _, v := range versions {
		entityType, ok := DoctypeToEntityType[v.RefDoctype]
		if !ok {
			continue
		}

		// Parse the version data to get the document state
		var versionData map[string]interface{}
		if v.Data != "" {
			if err := json.Unmarshal([]byte(v.Data), &versionData); err != nil {
				continue // Skip malformed version data
			}
		}

		// Determine action based on version data
		action := "update"
		if _, hasChanged := versionData["changed"]; !hasChanged {
			// If no "changed" field, this might be a creation
			action = "create"
		}

		// Get module info - we'll need to look this up from the current state
		module := "_"

		// Build file path based on entity type
		filePath := buildFilePath(entityType, v.Docname, module)

		entry := HistoryEntry{
			Timestamp:   v.Creation,
			Author:      v.Owner,
			DocType:     v.RefDoctype,
			DocName:     v.Docname,
			Action:      action,
			VersionData: v.Data,
			VersionName: v.Name,
			EntityType:  entityType,
			Module:      module,
			FilePath:    filePath,
		}

		history = append(history, entry)
	}

	return history, nil
}

// FetchHistoryWithDocs fetches version history and includes current document state
// This is used during clone to reconstruct the full history
func (f *Fetcher) FetchHistoryWithDocs(entities []Entity) ([]HistoryEntry, map[string]Entity, error) {
	// Create a map of entities by doctype:name for quick lookup
	entityMap := make(map[string]Entity)
	for _, e := range entities {
		key := string(e.Type) + ":" + e.Name
		entityMap[key] = e
	}

	versions, err := f.Client.GetAllVersions(VersionedDoctypes())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch versions: %w", err)
	}

	var history []HistoryEntry
	entitiesWithHistory := make(map[string]bool)

	for _, v := range versions {
		entityType, ok := DoctypeToEntityType[v.RefDoctype]
		if !ok {
			continue
		}

		// Mark this entity as having history
		key := string(entityType) + ":" + v.Docname
		entitiesWithHistory[key] = true

		// Get module and entity data from entity map if available
		module := "_"
		filePath := ""
		var entityData map[string]interface{}
		if e, exists := entityMap[key]; exists {
			module = e.Module
			filePath = e.FilePath
			entityData = e.Data
		} else {
			filePath = buildFilePath(entityType, v.Docname, module)
		}

		// Determine action
		action := determineAction(v.Data)

		entry := HistoryEntry{
			Timestamp:   v.Creation,
			Author:      v.Owner,
			DocType:     v.RefDoctype,
			DocName:     v.Docname,
			Action:      action,
			VersionData: v.Data, // Store raw data for commit message generation
			VersionName: v.Name,
			EntityType:  entityType,
			Module:      module,
			FilePath:    filePath,
			EntityData:  entityData,
		}

		history = append(history, entry)
	}

	// Return entities that don't have version history (for fallback commit)
	entitiesWithoutHistory := make(map[string]Entity)
	for key, e := range entityMap {
		if !entitiesWithHistory[key] {
			entitiesWithoutHistory[key] = e
		}
	}

	return history, entitiesWithoutHistory, nil
}

// FetchUsers fetches user information for all authors in history
func (f *Fetcher) FetchUsers(history []HistoryEntry) (map[string]UserInfo, error) {
	// Collect unique author emails
	var emails []string
	seen := make(map[string]bool)
	for _, h := range history {
		if !seen[h.Author] && h.Author != "" {
			seen[h.Author] = true
			emails = append(emails, h.Author)
		}
	}

	return f.Client.GetUsers(emails)
}

// determineAction determines the action type from version data
func determineAction(data string) string {
	if data == "" {
		return "create"
	}

	var versionData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &versionData); err != nil {
		return "update"
	}

	// Check for creation indicators
	if changed, ok := versionData["changed"].([]interface{}); ok && len(changed) == 0 {
		if added, ok := versionData["added"].([]interface{}); ok && len(added) > 0 {
			return "create"
		}
	}

	return "update"
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
