/*
Copyright © 2025 Gavin <me@gavv.in>

Entity fetchers for syncing customizations from remote Frappe sites.
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

// Fetcher handles fetching entities from a remote site
type Fetcher struct {
	Client         *Client
	Config         *SiteConfig
	docTypeModules map[string]string // Cache of doctype -> module mappings
}

// NewFetcher creates a new entity fetcher
func NewFetcher(client *Client, config *SiteConfig) *Fetcher {
	return &Fetcher{
		Client:         client,
		Config:         config,
		docTypeModules: make(map[string]string),
	}
}

// getDocTypeModule returns the module for a given doctype, fetching if needed
func (f *Fetcher) getDocTypeModule(doctype string) string {
	if doctype == "" {
		return "_"
	}

	// Check cache first
	if module, exists := f.docTypeModules[doctype]; exists {
		return module
	}

	// Fetch from server
	modules, err := f.Client.GetDocTypeModules([]string{doctype})
	if err != nil {
		f.docTypeModules[doctype] = "_"
		return "_"
	}

	if module, exists := modules[doctype]; exists {
		f.docTypeModules[doctype] = module
		return module
	}

	f.docTypeModules[doctype] = "_"
	return "_"
}

// prefetchDocTypeModules fetches modules for all given doctypes in one request
func (f *Fetcher) prefetchDocTypeModules(doctypes []string) {
	if len(doctypes) == 0 {
		return
	}

	modules, err := f.Client.GetDocTypeModules(doctypes)
	if err != nil {
		return
	}

	for dt, mod := range modules {
		f.docTypeModules[dt] = mod
	}
}

// FetchAll fetches all enabled entity types
func (f *Fetcher) FetchAll() (*FetchResult, error) {
	result := &FetchResult{
		Entities: []Entity{},
		Modules:  make(map[string]ModuleInfo),
		Apps:     make(map[string]AppInfo),
	}

	// Get Frappe version
	version, err := f.Client.GetFrappeVersion()
	if err != nil {
		// Non-fatal, continue without version
		version = "unknown"
	}
	result.FrappeVer = version

	// Get modules
	modules, err := f.Client.GetModules()
	if err == nil {
		for _, mod := range modules {
			name := getString(mod, "name")
			appName := getString(mod, "app_name")
			if name != "" {
				result.Modules[name] = ModuleInfo{
					App:  appName,
					Sync: f.shouldSyncModule(name),
				}
			}
		}
	}

	// Get apps
	apps, err := f.Client.GetInstalledApps()
	if err == nil {
		for _, app := range apps {
			name := getString(app, "app_name")
			version := getString(app, "app_version")
			if name != "" {
				result.Apps[name] = AppInfo{Version: version}
			}
		}
	}

	// Fetch each entity type
	if f.Config.Sync.Entities.DocType {
		entities, err := f.fetchCustomDocTypes()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch custom doctypes: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.CustomField {
		entities, err := f.fetchCustomFields()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch custom fields: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.PropertySetter {
		entities, err := f.fetchPropertySetters()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch property setters: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.ClientScript {
		entities, err := f.fetchClientScripts()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch client scripts: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.ServerScript {
		entities, err := f.fetchServerScripts()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch server scripts: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.Report {
		entities, err := f.fetchReports()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch reports: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.PrintFormat {
		entities, err := f.fetchPrintFormats()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch print formats: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.Workflow {
		entities, err := f.fetchWorkflows()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch workflows: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.Notification {
		entities, err := f.fetchNotifications()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch notifications: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	if f.Config.Sync.Entities.LetterHead {
		entities, err := f.fetchLetterHeads()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch letter heads: %w", err)
		}
		result.Entities = append(result.Entities, entities...)
	}

	return result, nil
}

// fetchCustomDocTypes fetches custom DocTypes (custom=1)
func (f *Fetcher) fetchCustomDocTypes() ([]Entity, error) {
	docs, err := f.Client.GetAll("DocType", map[string]interface{}{
		"custom": 1,
	}, []string{"name", "module"})
	if err != nil {
		return nil, err
	}

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		// Get full document
		fullDoc, err := f.Client.GetDoc("DocType", name)
		if err != nil {
			continue // Skip on error
		}

		module := getString(fullDoc, "module")
		if module == "" {
			module = "_" // Catch-all
		}

		entities = append(entities, Entity{
			Type:     EntityDocType,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "doctype", toSnakeCase(name), toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// fetchCustomFields fetches Custom Fields (is_system_generated=0)
func (f *Fetcher) fetchCustomFields() ([]Entity, error) {
	docs, err := f.Client.GetAll("Custom Field", map[string]interface{}{
		"is_system_generated": 0,
	}, []string{"*"})
	if err != nil {
		return nil, err
	}

	// Collect target doctypes and prefetch their modules
	var targetDocTypes []string
	for _, doc := range docs {
		if dt := getString(doc, "dt"); dt != "" {
			targetDocTypes = append(targetDocTypes, dt)
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	// Group by target doctype
	byDocType := make(map[string][]map[string]interface{})

	for _, doc := range docs {
		dt := getString(doc, "dt")
		if dt == "" {
			continue
		}
		byDocType[dt] = append(byDocType[dt], doc)
	}

	var entities []Entity
	for dt, fields := range byDocType {
		// Use the target DocType's module
		module := f.getDocTypeModule(dt)

		// Create combined document
		combined := map[string]interface{}{
			"doctype":       dt,
			"custom_fields": fields,
		}

		entities = append(entities, Entity{
			Type:     EntityCustomField,
			Name:     dt,
			Module:   module,
			Data:     combined,
			FilePath: filepath.Join(toSnakeCase(module), "custom_field", toSnakeCase(dt)+".json"),
		})
	}

	return entities, nil
}

// fetchPropertySetters fetches Property Setters
func (f *Fetcher) fetchPropertySetters() ([]Entity, error) {
	docs, err := f.Client.GetAll("Property Setter", nil, []string{"*"})
	if err != nil {
		return nil, err
	}

	// Collect target doctypes and prefetch their modules
	var targetDocTypes []string
	for _, doc := range docs {
		if dt := getString(doc, "doc_type"); dt != "" {
			targetDocTypes = append(targetDocTypes, dt)
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	// Group by target doctype
	byDocType := make(map[string][]map[string]interface{})

	for _, doc := range docs {
		dt := getString(doc, "doc_type")
		if dt == "" {
			continue
		}
		byDocType[dt] = append(byDocType[dt], doc)
	}

	var entities []Entity
	for dt, setters := range byDocType {
		// Use the target DocType's module
		module := f.getDocTypeModule(dt)

		combined := map[string]interface{}{
			"doctype":          dt,
			"property_setters": setters,
		}

		entities = append(entities, Entity{
			Type:     EntityPropertySetter,
			Name:     dt,
			Module:   module,
			Data:     combined,
			FilePath: filepath.Join(toSnakeCase(module), "property_setter", toSnakeCase(dt)+".json"),
		})
	}

	return entities, nil
}

// fetchClientScripts fetches Client Scripts
func (f *Fetcher) fetchClientScripts() ([]Entity, error) {
	docs, err := f.Client.GetAll("Client Script", nil, []string{"*"})
	if err != nil {
		return nil, err
	}

	// Collect target doctypes (dt field) and prefetch their modules
	var targetDocTypes []string
	for _, doc := range docs {
		if dt := getString(doc, "dt"); dt != "" {
			targetDocTypes = append(targetDocTypes, dt)
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		// Get full document
		fullDoc, err := f.Client.GetDoc("Client Script", name)
		if err != nil {
			continue
		}

		// Use the target DocType's module (from dt field)
		targetDocType := getString(fullDoc, "dt")
		module := f.getDocTypeModule(targetDocType)

		entities = append(entities, Entity{
			Type:     EntityClientScript,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "client_script", toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// fetchServerScripts fetches Server Scripts
func (f *Fetcher) fetchServerScripts() ([]Entity, error) {
	docs, err := f.Client.GetAll("Server Script", nil, []string{"*"})
	if err != nil {
		return nil, err
	}

	// Collect reference doctypes for DocType Event scripts
	var targetDocTypes []string
	for _, doc := range docs {
		scriptType := getString(doc, "script_type")
		if scriptType == "DocType Event" {
			if dt := getString(doc, "reference_doctype"); dt != "" {
				targetDocTypes = append(targetDocTypes, dt)
			}
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		fullDoc, err := f.Client.GetDoc("Server Script", name)
		if err != nil {
			continue
		}

		// Determine module based on script type
		var module string
		scriptType := getString(fullDoc, "script_type")

		if scriptType == "DocType Event" {
			// Use the reference doctype's module
			refDoctype := getString(fullDoc, "reference_doctype")
			module = f.getDocTypeModule(refDoctype)
		} else {
			// For other types (Scheduler Event, API, Permission Query), use script's own module
			module = getString(fullDoc, "module")
			if module == "" {
				module = "_"
			}
		}

		entities = append(entities, Entity{
			Type:     EntityServerScript,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "server_script", toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// fetchReports fetches custom Reports (is_standard="No")
func (f *Fetcher) fetchReports() ([]Entity, error) {
	docs, err := f.Client.GetAll("Report", map[string]interface{}{
		"is_standard": "No",
	}, []string{"name", "module", "ref_doctype"})
	if err != nil {
		return nil, err
	}

	// Collect target doctypes and prefetch their modules
	var targetDocTypes []string
	for _, doc := range docs {
		if dt := getString(doc, "ref_doctype"); dt != "" {
			targetDocTypes = append(targetDocTypes, dt)
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		fullDoc, err := f.Client.GetDoc("Report", name)
		if err != nil {
			continue
		}

		// Use the target DocType's module (ref_doctype field)
		targetDocType := getString(fullDoc, "ref_doctype")
		module := f.getDocTypeModule(targetDocType)

		entities = append(entities, Entity{
			Type:     EntityReport,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "report", toSnakeCase(name), toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// fetchPrintFormats fetches custom Print Formats (standard="No")
func (f *Fetcher) fetchPrintFormats() ([]Entity, error) {
	docs, err := f.Client.GetAll("Print Format", map[string]interface{}{
		"standard": "No",
	}, []string{"name", "module", "doc_type"})
	if err != nil {
		return nil, err
	}

	// Collect target doctypes and prefetch their modules
	var targetDocTypes []string
	for _, doc := range docs {
		if dt := getString(doc, "doc_type"); dt != "" {
			targetDocTypes = append(targetDocTypes, dt)
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		fullDoc, err := f.Client.GetDoc("Print Format", name)
		if err != nil {
			continue
		}

		// Use the target DocType's module
		targetDocType := getString(fullDoc, "doc_type")
		module := f.getDocTypeModule(targetDocType)

		entities = append(entities, Entity{
			Type:     EntityPrintFormat,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "print_format", toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// fetchWorkflows fetches Workflows
func (f *Fetcher) fetchWorkflows() ([]Entity, error) {
	docs, err := f.Client.GetAll("Workflow", nil, []string{"name", "document_type"})
	if err != nil {
		return nil, err
	}

	// Collect target doctypes and prefetch their modules
	var targetDocTypes []string
	for _, doc := range docs {
		if dt := getString(doc, "document_type"); dt != "" {
			targetDocTypes = append(targetDocTypes, dt)
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		fullDoc, err := f.Client.GetDoc("Workflow", name)
		if err != nil {
			continue
		}

		// Use the target DocType's module
		targetDocType := getString(fullDoc, "document_type")
		module := f.getDocTypeModule(targetDocType)

		entities = append(entities, Entity{
			Type:     EntityWorkflow,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "workflow", toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// fetchNotifications fetches Notifications
func (f *Fetcher) fetchNotifications() ([]Entity, error) {
	docs, err := f.Client.GetAll("Notification", nil, []string{"name", "document_type"})
	if err != nil {
		return nil, err
	}

	// Collect target doctypes and prefetch their modules
	var targetDocTypes []string
	for _, doc := range docs {
		if dt := getString(doc, "document_type"); dt != "" {
			targetDocTypes = append(targetDocTypes, dt)
		}
	}
	f.prefetchDocTypeModules(targetDocTypes)

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		fullDoc, err := f.Client.GetDoc("Notification", name)
		if err != nil {
			continue
		}

		// Use the target DocType's module
		targetDocType := getString(fullDoc, "document_type")
		module := f.getDocTypeModule(targetDocType)

		entities = append(entities, Entity{
			Type:     EntityNotification,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "notification", toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// fetchLetterHeads fetches Letter Heads
func (f *Fetcher) fetchLetterHeads() ([]Entity, error) {
	docs, err := f.Client.GetAll("Letter Head", nil, []string{"name"})
	if err != nil {
		return nil, err
	}

	var entities []Entity
	for _, doc := range docs {
		name := getString(doc, "name")
		if name == "" {
			continue
		}

		fullDoc, err := f.Client.GetDoc("Letter Head", name)
		if err != nil {
			continue
		}

		// Letter Heads don't have module, use Custom
		module := "Custom"

		entities = append(entities, Entity{
			Type:     EntityLetterHead,
			Name:     name,
			Module:   module,
			Data:     fullDoc,
			FilePath: filepath.Join(toSnakeCase(module), "letter_head", toSnakeCase(name)+".json"),
		})
	}

	return entities, nil
}

// shouldSyncModule checks if a module should be synced
func (f *Fetcher) shouldSyncModule(module string) bool {
	if info, exists := f.Config.Modules[module]; exists {
		return info.Sync
	}
	// Default: sync if it's Custom or catch-all
	return module == "Custom" || module == "_"
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

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func toSnakeCase(s string) string {
	// Simple conversion: lowercase and replace spaces with underscores
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

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

// applyVersionReverse applies version changes in reverse to get previous state
// Takes current data and version data, returns what the data looked like BEFORE this version
func applyVersionReverse(currentData map[string]interface{}, versionDataJSON string) map[string]interface{} {
	if versionDataJSON == "" {
		return currentData
	}

	var versionData map[string]interface{}
	if err := json.Unmarshal([]byte(versionDataJSON), &versionData); err != nil {
		return currentData
	}

	// Deep copy current data
	result := deepCopyMap(currentData)

	// Reverse field changes: replace new values with old values
	if changed, ok := versionData["changed"].([]interface{}); ok {
		for _, c := range changed {
			if arr, ok := c.([]interface{}); ok && len(arr) >= 3 {
				fieldName, ok := arr[0].(string)
				if !ok {
					continue
				}
				oldValue := arr[1]
				// Set field to its old value
				result[fieldName] = oldValue
			}
		}
	}

	// Handle child table additions (reverse = remove these rows)
	if added, ok := versionData["added"].([]interface{}); ok {
		for _, item := range added {
			if arr, ok := item.([]interface{}); ok && len(arr) >= 2 {
				tableName, ok := arr[0].(string)
				if !ok {
					continue
				}
				rowData, ok := arr[1].(map[string]interface{})
				if !ok {
					continue
				}
				// Remove this row from the child table
				if table, exists := result[tableName].([]interface{}); exists {
					result[tableName] = removeRowFromTable(table, rowData)
				}
			}
		}
	}

	// Handle child table removals (reverse = add these rows back)
	if removed, ok := versionData["removed"].([]interface{}); ok {
		for _, item := range removed {
			if arr, ok := item.([]interface{}); ok && len(arr) >= 2 {
				tableName, ok := arr[0].(string)
				if !ok {
					continue
				}
				rowData, ok := arr[1].(map[string]interface{})
				if !ok {
					continue
				}
				// Add this row back to the child table
				if table, exists := result[tableName].([]interface{}); exists {
					result[tableName] = append(table, rowData)
				} else {
					result[tableName] = []interface{}{rowData}
				}
			}
		}
	}

	return result
}

// removeRowFromTable removes a row from a child table by matching the "name" field
func removeRowFromTable(table []interface{}, rowToRemove map[string]interface{}) []interface{} {
	rowName, _ := rowToRemove["name"].(string)
	if rowName == "" {
		return table
	}

	var newTable []interface{}
	for _, row := range table {
		if rowMap, ok := row.(map[string]interface{}); ok {
			if name, _ := rowMap["name"].(string); name != rowName {
				newTable = append(newTable, row)
			}
		} else {
			newTable = append(newTable, row)
		}
	}
	return newTable
}

// deepCopyMap creates a deep copy of a map
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	// Use JSON marshal/unmarshal for deep copy
	data, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return m
	}
	return result
}

// HistoricalState represents an entity's state at a point in history
type HistoricalState struct {
	FilePath    string
	Data        map[string]interface{}
	Timestamp   string
	Author      string
	Message     string
	VersionName string // Name of the Version document for traceability
}

// ReconstructFileHistory reconstructs the historical states of a file from versions
// Returns states in chronological order (oldest first)
func ReconstructFileHistory(currentData map[string]interface{}, versions []HistoryEntry) []HistoricalState {
	if len(versions) == 0 {
		return nil
	}

	// Sort versions by timestamp (newest first for backward traversal)
	sorted := make([]HistoryEntry, len(versions))
	copy(sorted, versions)
	sortHistoryByTimestampDesc(sorted)

	// Start with current state and work backwards
	states := make([]HistoricalState, len(versions))
	currentState := deepCopyMap(currentData)

	for i, v := range sorted {
		// Store this state (will be reversed later)
		states[i] = HistoricalState{
			FilePath:    v.FilePath,
			Data:        currentState,
			Timestamp:   v.Timestamp,
			Author:      v.Author,
			Message:     generateCommitMessage(v),
			VersionName: v.VersionName,
		}
		// Apply version in reverse to get previous state
		currentState = applyVersionReverse(currentState, v.VersionData)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(states)-1; i < j; i, j = i+1, j-1 {
		states[i], states[j] = states[j], states[i]
	}

	return states
}

// sortHistoryByTimestampDesc sorts history entries by timestamp descending (newest first)
func sortHistoryByTimestampDesc(history []HistoryEntry) {
	for i := 0; i < len(history)-1; i++ {
		for j := i + 1; j < len(history); j++ {
			if history[i].Timestamp < history[j].Timestamp {
				history[i], history[j] = history[j], history[i]
			}
		}
	}
}

// versionChange represents a single field change with old and new values
type versionChange struct {
	field    string
	oldValue interface{}
	newValue interface{}
}

// parseVersionData extracts meaningful change info from version data JSON
func parseVersionData(data string, entityType EntityType) (action string, changes []versionChange, addedRows, removedRows int) {
	action = "update"

	if data == "" {
		return "create", nil, 0, 0
	}

	var versionData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &versionData); err != nil {
		return "update", nil, 0, 0
	}

	// Extract changed fields with values
	if changed, ok := versionData["changed"].([]interface{}); ok {
		for _, c := range changed {
			if arr, ok := c.([]interface{}); ok && len(arr) >= 1 {
				if fieldName, ok := arr[0].(string); ok {
					vc := versionChange{field: fieldName}
					if len(arr) >= 2 {
						vc.oldValue = arr[1]
					}
					if len(arr) >= 3 {
						vc.newValue = arr[2]
					}
					changes = append(changes, vc)
				}
			}
		}
	}

	// Check for row additions (child tables)
	if added, ok := versionData["added"].([]interface{}); ok {
		addedRows = len(added)
	}

	// Check for row removals
	if removed, ok := versionData["removed"].([]interface{}); ok {
		removedRows = len(removed)
	}

	// Determine action - creation if no field changes but rows added
	if len(changes) == 0 && addedRows > 0 && removedRows == 0 {
		action = "create"
	}

	return action, changes, addedRows, removedRows
}

// describeChanges generates a human-readable description of changes
func describeChanges(changes []versionChange, addedRows, removedRows int, entityType EntityType) string {
	var descriptions []string

	// Process field changes with smart descriptions
	for _, c := range changes {
		desc := describeFieldChange(c, entityType)
		if desc != "" {
			descriptions = append(descriptions, desc)
		}
	}

	// Describe row changes based on entity type
	if addedRows > 0 {
		rowType := describeRowType(entityType, addedRows)
		descriptions = append(descriptions, fmt.Sprintf("add %d %s", addedRows, rowType))
	}
	if removedRows > 0 {
		rowType := describeRowType(entityType, removedRows)
		descriptions = append(descriptions, fmt.Sprintf("remove %d %s", removedRows, rowType))
	}

	// Deduplicate and limit
	seen := make(map[string]bool)
	var unique []string
	for _, d := range descriptions {
		if !seen[d] {
			seen[d] = true
			unique = append(unique, d)
		}
	}

	if len(unique) > 3 {
		return fmt.Sprintf("%s and %d more changes", strings.Join(unique[:2], ", "), len(unique)-2)
	}
	return strings.Join(unique, ", ")
}

// describeFieldChange generates a description for a single field change
func describeFieldChange(c versionChange, entityType EntityType) string {
	field := c.field

	// Handle enabled/disabled toggles
	if field == "enabled" || field == "disabled" {
		if isTruthy(c.newValue) {
			return "enable"
		}
		return "disable"
	}

	// Handle script-related fields
	switch field {
	case "script":
		return "update script"
	case "report_script":
		return "update query"
	case "javascript":
		if entityType == EntityReport {
			return "update client script"
		}
		return "update javascript"
	case "query":
		return "update query"
	case "html":
		return "update template"
	case "css":
		return "update styles"
	case "json":
		return "update config"
	}

	// Handle common metadata fields (skip or simplify)
	switch field {
	case "modified", "modified_by", "idx", "docstatus":
		return "" // Skip these noise fields
	case "owner", "creation":
		return "" // Skip
	}

	// Clean up field name for display
	cleanField := strings.ReplaceAll(field, "_", " ")
	return fmt.Sprintf("update %s", cleanField)
}

// describeRowType returns a human-readable name for child table rows
func describeRowType(entityType EntityType, count int) string {
	plural := count != 1

	switch entityType {
	case EntityDocType:
		if plural {
			return "fields"
		}
		return "field"
	case EntityReport:
		if plural {
			return "columns"
		}
		return "column"
	case EntityWorkflow:
		if plural {
			return "states"
		}
		return "state"
	default:
		if plural {
			return "rows"
		}
		return "row"
	}
}

// isTruthy checks if a value represents a truthy state
func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case int:
		return val != 0
	case string:
		return val == "1" || val == "true" || val == "True"
	}
	return false
}

// generateCommitMessage creates a conventional commit message for a history entry
func generateCommitMessage(entry HistoryEntry) string {
	action, changes, addedRows, removedRows := parseVersionData(entry.VersionData, entry.EntityType)

	// Override action if entry has one
	if entry.Action != "" {
		action = entry.Action
	}

	// Get smart scope and description based on entity type and metadata
	scope, description, prefix := buildSmartCommitParts(entry, action, changes, addedRows, removedRows)

	// Build conventional commit message
	switch action {
	case "create":
		return fmt.Sprintf("feat(%s): add %s", scope, entry.DocName)
	case "delete":
		return fmt.Sprintf("chore(%s): remove %s", scope, entry.DocName)
	default:
		if description != "" {
			return fmt.Sprintf("%s(%s): %s", prefix, scope, description)
		}
		return fmt.Sprintf("%s(%s): update %s", prefix, scope, entry.DocName)
	}
}

// buildSmartCommitParts extracts scope, description, and prefix from entity metadata
func buildSmartCommitParts(entry HistoryEntry, action string, changes []versionChange, addedRows, removedRows int) (scope, description, prefix string) {
	prefix = "chore" // default
	data := entry.EntityData

	switch entry.EntityType {
	case EntityServerScript:
		return buildServerScriptCommit(entry, data, changes, addedRows, removedRows)

	case EntityClientScript:
		return buildClientScriptCommit(entry, data, changes)

	case EntityReport:
		return buildReportCommit(entry, changes, addedRows, removedRows)

	case EntityDocType:
		return buildDocTypeCommit(entry, changes, addedRows, removedRows)

	case EntityWorkflow:
		// Workflow scope is the target doctype
		if data != nil {
			if docType := getString(data, "document_type"); docType != "" {
				scope = docType
			} else {
				scope = entry.DocName
			}
		} else {
			scope = entry.DocName
		}
		description = describeChanges(changes, addedRows, removedRows, entry.EntityType)
		if description == "" {
			description = "update workflow"
		}
		return scope, description, "chore"

	case EntityCustomField:
		// Custom field scope is the target doctype
		if data != nil {
			if dt := getString(data, "dt"); dt != "" {
				scope = dt
				description = fmt.Sprintf("update custom field %s", entry.DocName)
				return scope, description, "chore"
			}
		}
		scope = "custom-field"
		description = describeChanges(changes, addedRows, removedRows, entry.EntityType)
		if description == "" {
			description = fmt.Sprintf("update %s", entry.DocName)
		}
		return scope, description, "chore"

	case EntityPropertySetter:
		// Property setter scope is the target doctype
		if data != nil {
			if docType := getString(data, "doc_type"); docType != "" {
				scope = docType
				fieldName := getString(data, "field_name")
				property := getString(data, "property")
				if fieldName != "" && property != "" {
					description = fmt.Sprintf("set %s.%s property", fieldName, property)
				} else {
					description = fmt.Sprintf("update property setter")
				}
				return scope, description, "chore"
			}
		}
		scope = "property-setter"
		return scope, fmt.Sprintf("update %s", entry.DocName), "chore"

	case EntityNotification:
		// Notification scope is the target doctype
		if data != nil {
			if docType := getString(data, "document_type"); docType != "" {
				scope = docType
				description = fmt.Sprintf("update %s notification", entry.DocName)
				return scope, description, "chore"
			}
		}
		scope = "notification"
		return scope, fmt.Sprintf("update %s", entry.DocName), "chore"

	default:
		scope = strings.ReplaceAll(strings.ToLower(string(entry.EntityType)), "_", "-")
		description = describeChanges(changes, addedRows, removedRows, entry.EntityType)
		if description == "" {
			description = fmt.Sprintf("update %s", entry.DocName)
		}
		return scope, description, "chore"
	}
}

// buildServerScriptCommit creates commit parts for server scripts
func buildServerScriptCommit(entry HistoryEntry, data map[string]interface{}, changes []versionChange, addedRows, removedRows int) (scope, description, prefix string) {
	prefix = "fix" // script changes are typically fixes/improvements

	if data == nil {
		return "server-script", fmt.Sprintf("update %s", entry.DocName), prefix
	}

	scriptType := getString(data, "script_type")
	refDoctype := getString(data, "reference_doctype")
	doctypeEvent := getString(data, "doctype_event")
	apiMethod := getString(data, "api_method")
	eventFreq := getString(data, "event_frequency")

	// Determine what changed
	scriptChanged := false
	for _, c := range changes {
		if c.field == "script" {
			scriptChanged = true
			break
		}
	}

	switch scriptType {
	case "DocType Event":
		if refDoctype != "" {
			scope = refDoctype
		} else {
			scope = entry.DocName
		}
		eventName := formatEventName(doctypeEvent)
		if scriptChanged {
			description = fmt.Sprintf("update %s hook", eventName)
		} else {
			desc := describeChanges(changes, addedRows, removedRows, entry.EntityType)
			if desc != "" {
				description = fmt.Sprintf("update %s hook (%s)", eventName, desc)
			} else {
				description = fmt.Sprintf("update %s hook", eventName)
			}
		}

	case "Scheduler Event":
		scope = "scheduler"
		freq := formatFrequency(eventFreq)
		if scriptChanged {
			description = fmt.Sprintf("update %s job: %s", freq, entry.DocName)
		} else {
			description = fmt.Sprintf("update %s job config: %s", freq, entry.DocName)
			prefix = "chore"
		}

	case "Permission Query":
		if refDoctype != "" {
			scope = refDoctype
		} else {
			scope = "permission"
		}
		description = "update permission query"

	case "API":
		scope = "api"
		if apiMethod != "" {
			description = fmt.Sprintf("update %s endpoint", apiMethod)
		} else {
			description = fmt.Sprintf("update %s", entry.DocName)
		}

	default:
		scope = "server-script"
		description = fmt.Sprintf("update %s", entry.DocName)
	}

	return scope, description, prefix
}

// buildClientScriptCommit creates commit parts for client scripts
func buildClientScriptCommit(entry HistoryEntry, data map[string]interface{}, changes []versionChange) (scope, description, prefix string) {
	prefix = "fix" // script changes are typically fixes/improvements

	if data == nil {
		return "client-script", fmt.Sprintf("update %s", entry.DocName), prefix
	}

	// Get target doctype
	dt := getString(data, "dt")
	if dt != "" {
		scope = dt
	} else {
		scope = entry.DocName
	}

	// Check what changed
	scriptChanged := false
	enabledChanged := false
	var enabledValue interface{}

	for _, c := range changes {
		switch c.field {
		case "script":
			scriptChanged = true
		case "enabled":
			enabledChanged = true
			enabledValue = c.newValue
		}
	}

	if enabledChanged && !scriptChanged {
		prefix = "chore"
		if isTruthy(enabledValue) {
			description = "enable client script"
		} else {
			description = "disable client script"
		}
	} else if scriptChanged {
		description = "update client script"
	} else {
		description = "update client script config"
		prefix = "chore"
	}

	return scope, description, prefix
}

// buildReportCommit creates commit parts for reports
func buildReportCommit(entry HistoryEntry, changes []versionChange, addedRows, removedRows int) (scope, description, prefix string) {
	scope = "report"
	prefix = "fix"

	// Check what changed
	queryChanged := false
	jsChanged := false

	for _, c := range changes {
		switch c.field {
		case "report_script", "query":
			queryChanged = true
		case "javascript":
			jsChanged = true
		}
	}

	var parts []string
	if queryChanged {
		parts = append(parts, "update query")
	}
	if jsChanged {
		parts = append(parts, "update UI")
	}
	if addedRows > 0 {
		parts = append(parts, fmt.Sprintf("add %d %s", addedRows, pluralize("column", addedRows)))
	}
	if removedRows > 0 {
		parts = append(parts, fmt.Sprintf("remove %d %s", removedRows, pluralize("column", removedRows)))
	}

	if len(parts) > 0 {
		description = fmt.Sprintf("%s in %s", strings.Join(parts, ", "), entry.DocName)
	} else {
		description = fmt.Sprintf("update %s", entry.DocName)
		prefix = "chore"
	}

	return scope, description, prefix
}

// buildDocTypeCommit creates commit parts for doctypes
func buildDocTypeCommit(entry HistoryEntry, changes []versionChange, addedRows, removedRows int) (scope, description, prefix string) {
	scope = entry.DocName
	prefix = "chore"

	var parts []string

	for _, c := range changes {
		switch c.field {
		case "is_submittable", "is_tree", "track_changes", "track_views":
			parts = append(parts, fmt.Sprintf("set %s", strings.ReplaceAll(c.field, "_", " ")))
		case "title_field", "naming_rule", "autoname", "quick_entry":
			parts = append(parts, fmt.Sprintf("update %s", strings.ReplaceAll(c.field, "_", " ")))
		}
	}

	if addedRows > 0 {
		parts = append(parts, fmt.Sprintf("add %d %s", addedRows, pluralize("field", addedRows)))
	}
	if removedRows > 0 {
		parts = append(parts, fmt.Sprintf("remove %d %s", removedRows, pluralize("field", removedRows)))
	}

	if len(parts) > 0 {
		description = strings.Join(parts, ", ")
	} else if len(changes) > 0 {
		// Generic change description
		description = fmt.Sprintf("update config")
	} else {
		description = "update schema"
	}

	return scope, description, prefix
}

// formatEventName converts doctype_event to readable format
func formatEventName(event string) string {
	switch event {
	case "before_insert":
		return "before_insert"
	case "after_insert":
		return "after_insert"
	case "before_save":
		return "before_save"
	case "after_save":
		return "after_save"
	case "before_submit":
		return "before_submit"
	case "after_submit":
		return "on_submit"
	case "before_cancel":
		return "before_cancel"
	case "after_cancel":
		return "on_cancel"
	case "on_update":
		return "on_update"
	case "on_trash":
		return "on_trash"
	case "before_validate":
		return "validate"
	default:
		if event == "" {
			return "document"
		}
		return event
	}
}

// formatFrequency converts event_frequency to readable format
func formatFrequency(freq string) string {
	switch freq {
	case "All":
		return "minutely"
	case "Hourly":
		return "hourly"
	case "Daily":
		return "daily"
	case "Weekly":
		return "weekly"
	case "Monthly":
		return "monthly"
	case "Yearly":
		return "yearly"
	case "Cron":
		return "cron"
	default:
		if freq == "" {
			return "scheduled"
		}
		return strings.ToLower(freq)
	}
}

// pluralize returns singular or plural form based on count
func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
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

// CommitInfo contains information needed to create a git commit
type CommitInfo struct {
	Timestamp    string                            // RFC3339 formatted timestamp
	Author       string                            // Author name and email formatted for git
	Message      string                            // Commit message
	Files        []string                          // Files to include in this commit
	FileContents map[string]map[string]interface{} // File path -> content to write
}

// BuildCommitPlan creates a list of commits from history entries and entities
// Reconstructs historical file states to provide actual diffs in git history
// users map provides full name resolution for author emails
func BuildCommitPlan(history []HistoryEntry, entities []Entity, entitiesWithoutHistory map[string]Entity, users map[string]UserInfo) []CommitInfo {
	// Create entity map for lookup by file path
	entityByPath := make(map[string]Entity)
	for _, e := range entities {
		entityByPath[e.FilePath] = e
	}

	// Group history entries by file path
	historyByPath := make(map[string][]HistoryEntry)
	for _, entry := range history {
		historyByPath[entry.FilePath] = append(historyByPath[entry.FilePath], entry)
	}

	// Reconstruct historical states for each file
	// Map: timestamp -> filePath -> HistoricalState
	statesByTimestamp := make(map[string]map[string]HistoricalState)
	allTimestamps := make(map[string]bool)

	for filePath, fileHistory := range historyByPath {
		entity, exists := entityByPath[filePath]
		if !exists {
			continue
		}

		// Reconstruct history for this file
		states := ReconstructFileHistory(entity.Data, fileHistory)

		for _, state := range states {
			ts := truncateTimestamp(state.Timestamp)
			allTimestamps[ts] = true

			if statesByTimestamp[ts] == nil {
				statesByTimestamp[ts] = make(map[string]HistoricalState)
			}
			statesByTimestamp[ts][filePath] = state
		}
	}

	// Sort timestamps chronologically
	var sortedTimestamps []string
	for ts := range allTimestamps {
		sortedTimestamps = append(sortedTimestamps, ts)
	}
	sortStrings(sortedTimestamps)

	// Build commit infos with proper file contents
	var commits []CommitInfo

	for _, ts := range sortedTimestamps {
		states := statesByTimestamp[ts]

		var msgParts []string
		var versionRefs []string
		var files []string
		fileContents := make(map[string]map[string]interface{})
		var author string

		for filePath, state := range states {
			msgParts = append(msgParts, state.Message)
			files = append(files, filePath)
			fileContents[filePath] = state.Data
			if author == "" {
				author = state.Author
			}
			if state.VersionName != "" {
				versionRefs = append(versionRefs, fmt.Sprintf("Version: %s", state.VersionName))
			}
		}

		// Build message with version references in body
		message := strings.Join(msgParts, "\n")
		if len(msgParts) > 1 {
			message = fmt.Sprintf("Multiple changes (%d)\n\n%s", len(msgParts), message)
		}
		if len(versionRefs) > 0 {
			message = message + "\n\n" + strings.Join(versionRefs, "\n")
		}

		commits = append(commits, CommitInfo{
			Timestamp:    ts,
			Author:       formatAuthor(author, users),
			Message:      message,
			Files:        files,
			FileContents: fileContents,
		})
	}

	// Add final commit for entities without version history
	if len(entitiesWithoutHistory) > 0 {
		var files []string
		fileContents := make(map[string]map[string]interface{})
		for _, e := range entitiesWithoutHistory {
			files = append(files, e.FilePath)
			fileContents[e.FilePath] = e.Data
		}

		// Use the earliest creation time among these entities
		timestamp := ""
		for _, e := range entitiesWithoutHistory {
			if creation, ok := e.Data["creation"].(string); ok && creation != "" {
				if timestamp == "" || creation < timestamp {
					timestamp = creation
				}
			}
		}
		if timestamp == "" {
			timestamp = "2000-01-01 00:00:00"
		}

		commits = append(commits, CommitInfo{
			Timestamp:    timestamp,
			Author:       "Weg <noreply@weg.io>",
			Message:      fmt.Sprintf("chore(sync): add %d entities without version history", len(files)),
			Files:        files,
			FileContents: fileContents,
		})
	}

	return commits
}

// sortStrings sorts a slice of strings in ascending order
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// sortHistoryByTimestamp sorts history entries by timestamp
func sortHistoryByTimestamp(history []HistoryEntry) {
	for i := 0; i < len(history)-1; i++ {
		for j := i + 1; j < len(history); j++ {
			if history[i].Timestamp > history[j].Timestamp {
				history[i], history[j] = history[j], history[i]
			}
		}
	}
}

// truncateTimestamp truncates a timestamp to second precision for grouping
func truncateTimestamp(ts string) string {
	// Frappe timestamps are in format "2025-01-21 14:31:36.892644"
	// Truncate to "2025-01-21 14:31:36"
	if len(ts) > 19 {
		return ts[:19]
	}
	return ts
}

// formatAuthor formats an author email with full name for git commits
func formatAuthor(email string, users map[string]UserInfo) string {
	if email == "" {
		return "Weg <noreply@weg.io>"
	}

	// Look up user info
	if user, exists := users[email]; exists && user.FullName != "" {
		return fmt.Sprintf("%s <%s>", user.FullName, email)
	}

	// Fallback: derive name from email
	name := deriveNameFromEmail(email)
	return fmt.Sprintf("%s <%s>", name, email)
}
