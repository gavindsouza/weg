/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/prompt"
	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local changes to the remote site",
	Long: `Push local file changes to the remote Frappe site.

By default, only pushes committed changes since last push.
Use -u to include uncommitted changes, or -a to push everything.

Examples:
  weg remote push           # Push committed changes since last push
  weg remote push -n        # Dry-run (preview what would be pushed)
  weg remote push -u        # Include uncommitted changes
  weg remote push -a        # Push all entities (use with caution)
  weg remote push -f        # Force push even if remote is newer`,
	RunE: runPush,
}

var (
	pushDryRun      bool
	pushForce       bool
	pushAll         bool
	pushUncommitted bool
)

func init() {
	pushCmd.Flags().BoolVarP(&pushDryRun, "dry-run", "n", false, "Preview changes without pushing")
	pushCmd.Flags().BoolVarP(&pushForce, "force", "f", false, "Force push even if remote is newer")
	pushCmd.Flags().BoolVarP(&pushAll, "all", "a", false, "Push all entities (not just changed ones)")
	pushCmd.Flags().BoolVarP(&pushUncommitted, "uncommitted", "u", false, "Include uncommitted changes")
}

func runPush(cobraCmd *cobra.Command, args []string) error {
	// Check if we're in a remote site directory
	if !remote.IsRemoteSite(".") {
		return fmt.Errorf("not a remote site clone (no .weg/site.toml found)")
	}

	// Load config and credentials
	config, err := remote.LoadSiteConfig(".")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	creds, err := remote.LoadCredentials(".")
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	// Find modified files using git diff
	var entities []localEntity

	if pushAll {
		// Push everything (dangerous, but sometimes needed)
		var err error
		entities, err = findLocalEntities(".")
		if err != nil {
			return fmt.Errorf("failed to find entities: %w", err)
		}
	} else {
		// Only push changed files (default, safe behavior)
		var err error
		entities, err = findChangedEntities(".", pushUncommitted)
		if err != nil {
			return fmt.Errorf("failed to find changed entities: %w", err)
		}
	}

	if len(entities) == 0 {
		fmt.Println("No changes to push")
		fmt.Println("(use --all to push all entities)")
		return nil
	}

	// Find deleted entities for dry-run display
	deletedEntities, _ := findDeletedEntities(".", pushUncommitted)

	if pushDryRun {
		if pushAll {
			fmt.Printf("Dry run - would push ALL %d entities:\n", len(entities))
		} else {
			fmt.Printf("Dry run - would push %d changed entities:\n", len(entities))
		}
		for _, e := range entities {
			fmt.Printf("  + %s: %s\n", e.entityType, e.name)
		}
		if len(deletedEntities) > 0 {
			fmt.Printf("\nWould delete %d entities:\n", len(deletedEntities))
			for _, e := range deletedEntities {
				fmt.Printf("  - %s: %s\n", e.entityType, e.name)
			}
		}
		return nil
	}

	// Connect
	fmt.Printf("Connecting to %s...\n", config.Site.URL)
	client := remote.NewClientFromConfig(config, creds)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Println("Connected")

	// Push each entity
	totalChanges := len(entities) + len(deletedEntities)
	if totalChanges == 0 {
		fmt.Println("No changes to push")
		return nil
	}

	fmt.Printf("Pushing %d changes...\n", totalChanges)
	pushed := 0
	deleted := 0
	failed := 0

	for _, e := range entities {
		if err := pushEntity(client, e); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to push %s: %v\n", e.name, err)
			failed++
		} else {
			pushed++
		}
	}

	// Delete removed entities (with confirmation)
	if len(deletedEntities) > 0 && !pushForce {
		fmt.Printf("\nThe following %d entities will be deleted from the remote:\n", len(deletedEntities))
		for _, e := range deletedEntities {
			fmt.Printf("  - %s: %s\n", e.entityType, e.name)
		}
		if !prompt.ConfirmDanger("Delete these entities from remote?") {
			fmt.Println("Skipping deletions")
			deletedEntities = nil
		}
	}

	for _, e := range deletedEntities {
		if err := deleteEntity(client, e); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to delete %s: %v\n", e.name, err)
			failed++
		} else {
			deleted++
		}
	}

	fmt.Printf("Pushed: %d, Deleted: %d, Failed: %d\n", pushed, deleted, failed)

	if failed > 0 {
		return fmt.Errorf("%d entities failed to push", failed)
	}

	// Save current commit as last push point
	if !pushDryRun {
		saveLastPushCommit(".")
	}

	return nil
}

// saveLastPushCommit saves the current HEAD commit as the last push point
func saveLastPushCommit(baseDir string) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = baseDir
	output, err := cmd.Output()
	if err != nil {
		return
	}

	commit := strings.TrimSpace(string(output))
	lastPushFile := filepath.Join(baseDir, ".weg", "last_push_commit")
	os.WriteFile(lastPushFile, []byte(commit), 0644)
}

type localEntity struct {
	filePath   string
	entityType string
	doctype    string
	name       string
	data       map[string]interface{}
}

// findChangedEntities finds only entities that have been modified since last push
func findChangedEntities(baseDir string, includeUncommitted bool) ([]localEntity, error) {
	// Get changed files from git
	changedFiles, err := getChangedFiles(baseDir, includeUncommitted)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(changedFiles) == 0 {
		return nil, nil
	}

	var entities []localEntity
	seen := make(map[string]bool) // Dedupe by file path

	for _, filePath := range changedFiles {
		// Skip non-JSON files and special files
		if !strings.HasSuffix(filePath, ".json") {
			continue
		}
		if strings.HasPrefix(filePath, ".weg/") || strings.HasPrefix(filePath, "weg_workspace/") {
			continue
		}

		// Skip if already processed
		if seen[filePath] {
			continue
		}
		seen[filePath] = true

		// Read and parse the file
		fullPath := filepath.Join(baseDir, filePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			// File might have been deleted
			continue
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(data, &doc); err != nil {
			continue
		}

		// Detect entity type from path
		parts := strings.Split(filePath, string(filepath.Separator))
		if len(parts) < 2 {
			continue
		}

		// Find the entity type directory (e.g., "server_script", "client_script")
		var typeName string
		for _, part := range parts {
			if isEntityType(part) {
				typeName = part
				break
			}
		}
		if typeName == "" {
			continue
		}

		doctype := typeToDocType(typeName)
		name := getString(doc, "name")
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(filePath), ".json")
		}

		entities = append(entities, localEntity{
			filePath:   fullPath,
			entityType: typeName,
			doctype:    doctype,
			name:       name,
			data:       doc,
		})
	}

	return entities, nil
}

// getChangedFiles returns list of changed files using git
func getChangedFiles(baseDir string, includeUncommitted bool) ([]string, error) {
	var allFiles []string

	if includeUncommitted {
		// Get uncommitted changes (staged and unstaged)
		cmd := exec.Command("git", "diff", "--name-only", "HEAD")
		cmd.Dir = baseDir
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			files := strings.Split(strings.TrimSpace(string(output)), "\n")
			allFiles = append(allFiles, files...)
		}

		// Get staged changes
		cmd = exec.Command("git", "diff", "--name-only", "--cached")
		cmd.Dir = baseDir
		output, err = cmd.Output()
		if err == nil && len(output) > 0 {
			files := strings.Split(strings.TrimSpace(string(output)), "\n")
			allFiles = append(allFiles, files...)
		}
	}

	// Get committed changes since last push
	// Uses .weg/last_push_commit to know what was last pushed
	lastPushFile := filepath.Join(baseDir, ".weg", "last_push_commit")
	if data, err := os.ReadFile(lastPushFile); err == nil {
		lastCommit := strings.TrimSpace(string(data))
		if lastCommit != "" {
			cmd := exec.Command("git", "diff", "--name-only", lastCommit+"..HEAD")
			cmd.Dir = baseDir
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				files := strings.Split(strings.TrimSpace(string(output)), "\n")
				allFiles = append(allFiles, files...)
			}
		}
	} else {
		// No last push commit, get changes from initial commit to HEAD
		// This handles fresh clones - compare against root commit
		cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
		cmd.Dir = baseDir
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			rootCommit := strings.TrimSpace(strings.Split(string(output), "\n")[0])
			cmd = exec.Command("git", "diff", "--name-only", rootCommit+"..HEAD")
			cmd.Dir = baseDir
			output, err = cmd.Output()
			if err == nil && len(output) > 0 {
				files := strings.Split(strings.TrimSpace(string(output)), "\n")
				allFiles = append(allFiles, files...)
			}
		}
	}

	// Dedupe
	seen := make(map[string]bool)
	var result []string
	for _, f := range allFiles {
		if f != "" && !seen[f] {
			seen[f] = true
			result = append(result, f)
		}
	}

	return result, nil
}

// findDeletedEntities finds entities that were deleted since last push
func findDeletedEntities(baseDir string, includeUncommitted bool) ([]localEntity, error) {
	var entities []localEntity

	// Get deleted files from git
	var deletedFiles []string

	if includeUncommitted {
		// Check for deleted files in working tree
		cmd := exec.Command("git", "diff", "--name-status", "HEAD")
		cmd.Dir = baseDir
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "D\t") {
					deletedFiles = append(deletedFiles, strings.TrimPrefix(line, "D\t"))
				}
			}
		}
	}

	// Check for deleted files since last push
	lastPushFile := filepath.Join(baseDir, ".weg", "last_push_commit")
	if data, err := os.ReadFile(lastPushFile); err == nil {
		lastCommit := strings.TrimSpace(string(data))
		if lastCommit != "" {
			cmd := exec.Command("git", "diff", "--name-status", lastCommit+"..HEAD")
			cmd.Dir = baseDir
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "D\t") {
						deletedFiles = append(deletedFiles, strings.TrimPrefix(line, "D\t"))
					}
				}
			}
		}
	}

	// Dedupe and process
	seen := make(map[string]bool)
	for _, filePath := range deletedFiles {
		if seen[filePath] || !strings.HasSuffix(filePath, ".json") {
			continue
		}
		if strings.HasPrefix(filePath, ".weg/") || strings.HasPrefix(filePath, "weg_workspace/") {
			continue
		}
		seen[filePath] = true

		// Detect entity type from path
		parts := strings.Split(filePath, string(filepath.Separator))
		var typeName string
		for _, part := range parts {
			if isEntityType(part) {
				typeName = part
				break
			}
		}
		if typeName == "" {
			continue
		}

		// Extract name from filename
		name := strings.TrimSuffix(filepath.Base(filePath), ".json")

		entities = append(entities, localEntity{
			filePath:   filePath,
			entityType: typeName,
			doctype:    typeToDocType(typeName),
			name:       name,
		})
	}

	return entities, nil
}

// deleteEntity deletes an entity on the remote site
func deleteEntity(client *remote.Client, e localEntity) error {
	switch e.entityType {
	case "custom_field", "property_setter":
		// These are grouped files - can't delete individual items this way
		// Would need special handling
		return fmt.Errorf("deletion of grouped entities not yet supported")
	default:
		return client.DeleteDoc(e.doctype, e.name)
	}
}

// isEntityType checks if a directory name is an entity type
func isEntityType(name string) bool {
	types := []string{
		"doctype", "custom_field", "property_setter", "client_script",
		"server_script", "report", "print_format", "workflow",
		"notification", "letter_head", "web_template",
	}
	for _, t := range types {
		if name == t {
			return true
		}
	}
	return false
}

func findLocalEntities(baseDir string) ([]localEntity, error) {
	var entities []localEntity

	// Walk through module directories
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		modulePath := filepath.Join(baseDir, entry.Name())
		moduleEntries, err := os.ReadDir(modulePath)
		if err != nil {
			continue
		}

		for _, typeEntry := range moduleEntries {
			if !typeEntry.IsDir() {
				continue
			}

			typeName := typeEntry.Name()
			typePath := filepath.Join(modulePath, typeName)

			// Find JSON files
			err := filepath.Walk(typePath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
					return nil
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				var doc map[string]interface{}
				if err := json.Unmarshal(data, &doc); err != nil {
					return nil
				}

				// Determine doctype based on directory
				doctype := typeToDocType(typeName)
				name := getString(doc, "name")
				if name == "" {
					// Try to get from filename
					name = strings.TrimSuffix(filepath.Base(path), ".json")
				}

				entities = append(entities, localEntity{
					filePath:   path,
					entityType: typeName,
					doctype:    doctype,
					name:       name,
					data:       doc,
				})

				return nil
			})
			if err != nil {
				continue
			}
		}
	}

	return entities, nil
}

func typeToDocType(typeName string) string {
	switch typeName {
	case "doctype":
		return "DocType"
	case "custom_field":
		return "Custom Field"
	case "property_setter":
		return "Property Setter"
	case "client_script":
		return "Client Script"
	case "server_script":
		return "Server Script"
	case "report":
		return "Report"
	case "print_format":
		return "Print Format"
	case "workflow":
		return "Workflow"
	case "notification":
		return "Notification"
	case "letter_head":
		return "Letter Head"
	default:
		return typeName
	}
}

func pushEntity(client *remote.Client, e localEntity) error {
	// Handle special cases
	switch e.entityType {
	case "custom_field":
		return pushCustomFields(client, e)
	case "property_setter":
		return pushPropertySetters(client, e)
	default:
		return pushDocument(client, e.doctype, e.name, e.data)
	}
}

func pushDocument(client *remote.Client, doctype, name string, data map[string]interface{}) error {
	// Check if document exists and get current modified timestamp
	existing, err := client.GetDoc(doctype, name)
	if err != nil {
		// Doesn't exist, create it
		_, err = client.InsertDoc(doctype, data)
		return err
	}

	// Copy the server's modified timestamp to avoid version conflict
	if modified, ok := existing["modified"]; ok {
		data["modified"] = modified
	}

	// Exists, update it
	_, err = client.UpdateDoc(doctype, name, data)
	return err
}

func pushCustomFields(client *remote.Client, e localEntity) error {
	// Custom fields are grouped by target doctype
	fields, ok := e.data["custom_fields"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid custom fields format")
	}

	for _, f := range fields {
		field, ok := f.(map[string]interface{})
		if !ok {
			continue
		}

		name := getString(field, "name")
		if name == "" {
			// New field, insert
			_, err := client.InsertDoc("Custom Field", field)
			if err != nil {
				return err
			}
		} else {
			// Get current modified timestamp to avoid version conflict
			existing, err := client.GetDoc("Custom Field", name)
			if err != nil {
				// Doesn't exist on server, insert
				_, err := client.InsertDoc("Custom Field", field)
				if err != nil {
					return err
				}
				continue
			}

			// Copy the server's modified timestamp
			if modified, ok := existing["modified"]; ok {
				field["modified"] = modified
			}

			// Existing field, update
			_, err = client.UpdateDoc("Custom Field", name, field)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func pushPropertySetters(client *remote.Client, e localEntity) error {
	setters, ok := e.data["property_setters"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid property setters format")
	}

	for _, s := range setters {
		setter, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		name := getString(setter, "name")
		if name == "" {
			_, err := client.InsertDoc("Property Setter", setter)
			if err != nil {
				return err
			}
		} else {
			_, err := client.UpdateDoc("Property Setter", name, setter)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
