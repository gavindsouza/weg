/*
Copyright © 2025 Gavin <me@gavv.in>

Commit generation from entity version history.
*/
package remote

import (
	"encoding/json"
	"fmt"
	"strings"
)

// HistoricalState represents an entity's state at a point in history
type HistoricalState struct {
	FilePath    string
	Data        map[string]interface{}
	Timestamp   string
	Author      string
	Message     string
	VersionName string // Name of the Version document for traceability
}

// CommitInfo contains information needed to create a git commit
type CommitInfo struct {
	Timestamp    string                            // RFC3339 formatted timestamp
	Author       string                            // Author name and email formatted for git
	Message      string                            // Commit message
	Files        []string                          // Files to include in this commit
	FileContents map[string]map[string]interface{} // File path -> content to write
}

// versionChange represents a single field change with old and new values
type versionChange struct {
	field    string
	oldValue interface{}
	newValue interface{}
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
