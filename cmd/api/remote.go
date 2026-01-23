package api

import (
	"github.com/gavindsouza/weg/internal/remote"
)

// RemoteResult wraps the result for consistent interface with local API
type RemoteResult struct {
	Success bool
	Data    any
	Error   string
}

// remoteGetDoc fetches a single document via HTTP
func remoteGetDoc(client *remote.Client, doctype, name string) (*RemoteResult, error) {
	data, err := client.GetDoc(doctype, name)
	if err != nil {
		return &RemoteResult{Success: false, Error: err.Error()}, nil
	}
	return &RemoteResult{Success: true, Data: data}, nil
}

// remoteGetList fetches a list of documents via HTTP
func remoteGetList(client *remote.Client, doctype string, filters map[string]any, fields []string, limit int, orderBy string) (*RemoteResult, error) {
	// Convert filters to map[string]interface{}
	var filtersIface map[string]interface{}
	if filters != nil {
		filtersIface = make(map[string]interface{})
		for k, v := range filters {
			filtersIface[k] = v
		}
	}

	data, err := client.GetList(doctype, filtersIface, fields, limit)
	if err != nil {
		return &RemoteResult{Success: false, Error: err.Error()}, nil
	}
	return &RemoteResult{Success: true, Data: data}, nil
}

// remoteCreate creates a new document via HTTP
func remoteCreate(client *remote.Client, doctype string, doc map[string]any) (*RemoteResult, error) {
	// Convert to map[string]interface{}
	docIface := make(map[string]interface{})
	for k, v := range doc {
		docIface[k] = v
	}

	data, err := client.InsertDoc(doctype, docIface)
	if err != nil {
		return &RemoteResult{Success: false, Error: err.Error()}, nil
	}
	return &RemoteResult{Success: true, Data: data}, nil
}

// remoteUpdate updates an existing document via HTTP
func remoteUpdate(client *remote.Client, doctype, name string, doc map[string]any) (*RemoteResult, error) {
	// Convert to map[string]interface{}
	docIface := make(map[string]interface{})
	for k, v := range doc {
		docIface[k] = v
	}

	data, err := client.UpdateDoc(doctype, name, docIface)
	if err != nil {
		return &RemoteResult{Success: false, Error: err.Error()}, nil
	}
	return &RemoteResult{Success: true, Data: data}, nil
}

// remoteDelete deletes a document via HTTP
func remoteDelete(client *remote.Client, doctype, name string) (*RemoteResult, error) {
	err := client.DeleteDoc(doctype, name)
	if err != nil {
		return &RemoteResult{Success: false, Error: err.Error()}, nil
	}
	return &RemoteResult{Success: true, Data: "deleted"}, nil
}

// remoteCall invokes a whitelisted method via HTTP
func remoteCall(client *remote.Client, method string, args map[string]any) (*RemoteResult, error) {
	// Convert to map[string]interface{}
	argsIface := make(map[string]interface{})
	for k, v := range args {
		argsIface[k] = v
	}

	data, err := client.CallMethod(method, argsIface)
	if err != nil {
		return &RemoteResult{Success: false, Error: err.Error()}, nil
	}
	return &RemoteResult{Success: true, Data: data}, nil
}
