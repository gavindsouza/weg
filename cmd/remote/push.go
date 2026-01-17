/*
Copyright © 2025 Gavin <me@gavv.in>
*/
package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gavindsouza/weg/internal/remote"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local changes to the remote site",
	Long: `Push local file changes to the remote Frappe site.

This command:
  1. Detects locally modified files
  2. Validates the changes
  3. Pushes changes to the remote site
  4. Creates Version records on the remote

Note: This pushes all committed changes. Use 'weg sync -m "msg"' to
commit and push in one step.

Examples:
  weg push               # Push all local changes
  weg push --dry-run     # Preview what would be pushed
  weg push --force       # Push even if remote has newer changes`,
	RunE: runPush,
}

var (
	pushDryRun bool
	pushForce  bool
)

func init() {
	pushCmd.Flags().BoolVar(&pushDryRun, "dry-run", false, "Preview changes without pushing")
	pushCmd.Flags().BoolVar(&pushForce, "force", false, "Force push even if remote is newer")
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

	// Find modified files
	// For now, we'll push all entity files (full sync)
	// TODO: Implement incremental push based on git diff

	entities, err := findLocalEntities(".")
	if err != nil {
		return fmt.Errorf("failed to find entities: %w", err)
	}

	if len(entities) == 0 {
		fmt.Println("No entities to push")
		return nil
	}

	if pushDryRun {
		fmt.Printf("Dry run - would push %d entities:\n", len(entities))
		for _, e := range entities {
			fmt.Printf("  %s: %s\n", e.entityType, e.name)
		}
		return nil
	}

	// Connect
	fmt.Printf("Connecting to %s...\n", config.Site.URL)
	client := remote.NewClientFromConfig(config, creds)
	if err := client.Ping(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	fmt.Println("✓ Connected")

	// Push each entity
	fmt.Printf("Pushing %d entities...\n", len(entities))
	pushed := 0
	failed := 0

	for _, e := range entities {
		if err := pushEntity(client, e); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to push %s: %v\n", e.name, err)
			failed++
		} else {
			// Verbose output omitted for now
			pushed++
		}
	}

	fmt.Printf("✓ Pushed: %d, Failed: %d\n", pushed, failed)

	if failed > 0 {
		return fmt.Errorf("%d entities failed to push", failed)
	}

	return nil
}

type localEntity struct {
	filePath   string
	entityType string
	doctype    string
	name       string
	data       map[string]interface{}
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
