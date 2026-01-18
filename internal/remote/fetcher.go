/*
Copyright © 2025 Gavin <me@gavv.in>

Fetcher for syncing entity customizations from remote Frappe sites.
*/
package remote

import (
	"fmt"
	"path/filepath"
)

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
