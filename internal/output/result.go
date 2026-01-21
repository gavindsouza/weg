package output

import (
	"fmt"
	"reflect"
	"strings"
)

// List prints a slice of items in the appropriate format based on CurrentFormat.
//
// For table format, it extracts headers from struct field names or json tags.
// For JSON format, it outputs the slice as a JSON array.
// For plain format, it prints one item per line.
// For quiet format, it prints only the first field (typically ID/name).
//
// Example:
//
//	type Site struct {
//	    Name   string `json:"name"`
//	    Status string `json:"status"`
//	}
//	sites := []Site{{Name: "mysite", Status: "Running"}}
//	output.List(sites)
func List(items any) error {
	format := EffectiveFormat()

	switch format {
	case FormatJSON:
		return JSON(items)
	case FormatTable:
		return listTable(items)
	case FormatPlain:
		return listPlain(items)
	case FormatQuiet:
		return listQuiet(items)
	default:
		return listTable(items)
	}
}

// Item prints a single item in the appropriate format.
//
// For table format, it prints as key-value pairs.
// For JSON format, it outputs as a JSON object.
// For plain format, it prints a simple representation.
// For quiet format, it prints only the first field.
func Item(item any) error {
	format := EffectiveFormat()

	switch format {
	case FormatJSON:
		return JSON(item)
	case FormatTable:
		return itemTable(item)
	case FormatPlain:
		return itemPlain(item)
	case FormatQuiet:
		return itemQuiet(item)
	default:
		return itemTable(item)
	}
}

// listTable renders a slice as a table using tabwriter.
func listTable(items any) error {
	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("List requires a slice, got %T", items)
	}

	if v.Len() == 0 {
		return nil // Empty list, nothing to print
	}

	// Get headers from the first element
	first := v.Index(0)
	if first.Kind() == reflect.Ptr {
		first = first.Elem()
	}

	headers, getters := extractFields(first.Type())
	if len(headers) == 0 {
		return fmt.Errorf("no fields found in %T", items)
	}

	t := NewTable(headers...)

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		values := make([]any, len(getters))
		for j, getter := range getters {
			values[j] = getter(elem)
		}
		t.Row(values...)
	}

	return t.Flush()
}

// listPlain renders a slice as plain text, one item per line.
func listPlain(items any) error {
	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("List requires a slice, got %T", items)
	}

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		fmt.Fprintln(Writer, formatPlainItem(elem))
	}

	return nil
}

// listQuiet renders only the first field (ID/name) of each item.
func listQuiet(items any) error {
	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("List requires a slice, got %T", items)
	}

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct && elem.NumField() > 0 {
			fmt.Fprintln(Writer, elem.Field(0).Interface())
		} else {
			fmt.Fprintln(Writer, elem.Interface())
		}
	}

	return nil
}

// itemTable renders a single item as key-value pairs.
func itemTable(item any) error {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		// For non-structs, just print the value
		fmt.Fprintln(Writer, v.Interface())
		return nil
	}

	t := NewTable("Field", "Value")
	typ := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		name := getFieldName(field)
		value := v.Field(i).Interface()
		t.Row(name, value)
	}

	return t.Flush()
}

// itemPlain renders a single item as plain text.
func itemPlain(item any) error {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	fmt.Fprintln(Writer, formatPlainItem(v))
	return nil
}

// itemQuiet renders only the first field of an item.
func itemQuiet(item any) error {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct && v.NumField() > 0 {
		fmt.Fprintln(Writer, v.Field(0).Interface())
	} else {
		fmt.Fprintln(Writer, v.Interface())
	}

	return nil
}

// extractFields extracts field names and value getters from a struct type.
// It uses json tags if present, otherwise the field name.
func extractFields(t reflect.Type) ([]string, []func(reflect.Value) any) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, nil
	}

	var headers []string
	var getters []func(reflect.Value) any

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Skip fields with json:"-"
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name := getFieldName(field)
		headers = append(headers, name)

		// Capture index for closure
		idx := i
		getters = append(getters, func(v reflect.Value) any {
			return v.Field(idx).Interface()
		})
	}

	return headers, getters
}

// getFieldName returns the display name for a struct field.
// Uses json tag if present, otherwise the field name.
func getFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" && jsonTag != "-" {
		// Handle "name,omitempty" format
		if comma := strings.Index(jsonTag, ","); comma != -1 {
			jsonTag = jsonTag[:comma]
		}
		if jsonTag != "" {
			return jsonTag
		}
	}
	return field.Name
}

// formatPlainItem formats a single item for plain text output.
func formatPlainItem(v reflect.Value) string {
	if v.Kind() != reflect.Struct {
		return fmt.Sprintf("%v", v.Interface())
	}

	var parts []string
	typ := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		value := v.Field(i).Interface()
		// First field is primary (no label), rest get labels
		if i == 0 {
			parts = append(parts, fmt.Sprintf("%v", value))
		} else {
			name := getFieldName(field)
			parts = append(parts, fmt.Sprintf("%s=%v", name, value))
		}
	}

	return strings.Join(parts, " ")
}
