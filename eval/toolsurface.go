// Package eval provides a tool-selection evaluation harness for the wiki MCP surface.
//
// It measures how well an LLM tool-selector maps natural-language user requests
// to the correct MCP tool, given the wiki's tools/list catalog. Used to quantify
// the improve-mcp-surface PR's value and establish a baseline for future renames.
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ToolDef is one entry in the MCP tools/list response — the subset of fields
// the eval harness needs.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// ToolSurface is a snapshot of the MCP tool catalog at a point in time.
type ToolSurface struct {
	Label string    `json:"label"`
	Tools []ToolDef `json:"tools"`
}

// ToolCount returns the number of tools in the surface.
func (s ToolSurface) ToolCount() int {
	return len(s.Tools)
}

// Find returns the ToolDef with the given name, or nil.
func (s ToolSurface) Find(name string) *ToolDef {
	for i := range s.Tools {
		if s.Tools[i].Name == name {
			return &s.Tools[i]
		}
	}
	return nil
}

// Names returns all tool names in order.
func (s ToolSurface) Names() []string {
	out := make([]string, len(s.Tools))
	for i := range s.Tools {
		out[i] = s.Tools[i].Name
	}
	return out
}

// Descriptions returns a map of tool name → description.
func (s ToolSurface) Descriptions() map[string]string {
	out := make(map[string]string, len(s.Tools))
	for i := range s.Tools {
		out[s.Tools[i].Name] = s.Tools[i].Description
	}
	return out
}

// FetchSurface contacts the wiki's /mcp endpoint, performs the MCP
// initialize + tools/list handshake, and returns the live ToolSurface.
// baseURL should not have a trailing slash (e.g. "http://localhost:8050").
func FetchSurface(baseURL string) (ToolSurface, error) {
	return fetchSurface(baseURL, http.DefaultClient)
}

// fetchSurface is the testable inner form of FetchSurface.
func fetchSurface(baseURL string, client *http.Client) (ToolSurface, error) {
	ctx := context.Background()

	// 1. Initialize to get a session
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"eval","version":"0.1"}}}`
	initReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp", strings.NewReader(initBody))
	if err != nil {
		return ToolSurface{}, fmt.Errorf("create init request: %w", err)
	}
	initReq.Header.Set("Content-Type", "application/json")
	initResp, err := client.Do(initReq)
	if err != nil {
		return ToolSurface{}, fmt.Errorf("init call: %w", err)
	}
	_ = initResp.Body.Close()
	sessionID := initResp.Header.Get("Mcp-Session-Id")

	// 2. tools/list
	listBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	listReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mcp", strings.NewReader(listBody))
	if err != nil {
		return ToolSurface{}, fmt.Errorf("create list request: %w", err)
	}
	listReq.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		listReq.Header.Set("Mcp-Session-Id", sessionID)
	}
	listResp, err := client.Do(listReq)
	if err != nil {
		return ToolSurface{}, fmt.Errorf("list call: %w", err)
	}
	defer func() { _ = listResp.Body.Close() }()

	if listResp.StatusCode != http.StatusOK {
		return ToolSurface{}, fmt.Errorf("list returned %d", listResp.StatusCode)
	}

	var rpcResp struct {
		Result struct {
			Tools []ToolDef `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&rpcResp); err != nil {
		return ToolSurface{}, fmt.Errorf("decode list response: %w", err)
	}

	return ToolSurface{
		Label: "post-PR",
		Tools: rpcResp.Result.Tools,
	}, nil
}
