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
	Type       EntityType
	Name       string
	Module     string
	Data       map[string]interface{}
	FilePath   string // Relative path where this should be saved
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
	Client *Client
	Config *SiteConfig
}

// NewFetcher creates a new entity fetcher
func NewFetcher(client *Client, config *SiteConfig) *Fetcher {
	return &Fetcher{
		Client: client,
		Config: config,
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

	// Group by target doctype
	byDocType := make(map[string][]map[string]interface{})
	moduleByDocType := make(map[string]string)

	for _, doc := range docs {
		dt := getString(doc, "dt")
		if dt == "" {
			continue
		}
		byDocType[dt] = append(byDocType[dt], doc)

		// Custom fields are site-level customizations - always use catch-all module
		// regardless of the target DocType's module
		if _, exists := moduleByDocType[dt]; !exists {
			moduleByDocType[dt] = "_"
		}
	}

	var entities []Entity
	for dt, fields := range byDocType {
		module := moduleByDocType[dt]

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

	// Group by target doctype
	byDocType := make(map[string][]map[string]interface{})
	moduleByDocType := make(map[string]string)

	for _, doc := range docs {
		dt := getString(doc, "doc_type")
		if dt == "" {
			continue
		}
		byDocType[dt] = append(byDocType[dt], doc)

		if _, exists := moduleByDocType[dt]; !exists {
			module := getString(doc, "module")
			if module == "" {
				module = "_"
			}
			moduleByDocType[dt] = module
		}
	}

	var entities []Entity
	for dt, setters := range byDocType {
		module := moduleByDocType[dt]

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

		module := getString(fullDoc, "module")
		if module == "" {
			module = "_"
		}

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

		module := getString(fullDoc, "module")
		if module == "" {
			module = "_"
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

		fullDoc, err := f.Client.GetDoc("Report", name)
		if err != nil {
			continue
		}

		module := getString(fullDoc, "module")
		if module == "" {
			module = "_"
		}

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

		fullDoc, err := f.Client.GetDoc("Print Format", name)
		if err != nil {
			continue
		}

		module := getString(fullDoc, "module")
		if module == "" {
			module = "_"
		}

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

		// Workflows don't have module field, use Custom
		module := "Custom"

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

		module := getString(fullDoc, "module")
		if module == "" {
			module = "Custom"
		}

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
