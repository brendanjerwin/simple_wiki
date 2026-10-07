package eval

import "fmt"

// PromptPreset is a named system-prompt configuration the harness can select.
type PromptPreset struct {
	Name   string
	Prompt string
}

// The "Discovering what you can do" section from the production chatPreamble
// (cmd/wiki-cli/pool.go). Extracted so it can be included or stripped from
// prompt presets to measure its effect on tool-selection accuracy.
const discoverySection = `## Discovering what you can do

Your wiki MCP toolset is broader than just read/edit. Each tool's description
is the source of truth for what it does and how to call it.

When the user asks for something you're not sure how to do, do NOT decline.
Instead:

1. **List your MCP tools** and skim the descriptions for something that fits.
2. **Search the wiki for a help-* page** matching a keyword from the request.
   Read it for usage patterns this wiki documents.
3. Compose the call.

Default to "let me look that up" rather than "I can't" when uncertain.`

// PromptPresets is the catalog of named system-prompt presets the harness supports.
// Each is a different hypothesis about what helps an LLM select the right tool.
var PromptPresets = []PromptPreset{
	{
		Name:   "minimal",
		Prompt: `You are a tool selector. Given the following MCP tool catalog and a user request, select the single best tool and its arguments.`,
	},
	{
		Name: "production",
		Prompt: fmt.Sprintf(`You are in an INTERACTIVE CHAT session with one or more users on a wiki page. This is a conversation, not a coding task.

Rules:
- Keep responses concise and conversational
- Use the wiki MCP tools only when the user asks you to read or edit pages

%s

Respond as JSON: {"tool": "<name>", "args": {...}}
If no tool is appropriate, respond: {"tool": null}`, discoverySection),
	},
	{
		Name: "production-no-discovery",
		Prompt: `You are in an INTERACTIVE CHAT session with one or more users on a wiki page. This is a conversation, not a coding task.

Rules:
- Keep responses concise and conversational
- Use the wiki MCP tools only when the user asks you to read or edit pages

Respond as JSON: {"tool": "<name>", "args": {...}}
If no tool is appropriate, respond: {"tool": null}`,
	},
	{
		Name: "catalog-hint",
		Prompt: `You are a tool selector. Given the following MCP tool catalog and a user request, select the single best tool and its arguments.

A machine-readable service catalog is available at /mcp/catalog if you need a higher-level overview of the services.

Respond as JSON: {"tool": "<name>", "args": {...}}
If no tool is appropriate, respond: {"tool": null}`,
	},
}

// FindPromptPreset returns the preset with the given name, or nil.
func FindPromptPreset(name string) *PromptPreset {
	for i := range PromptPresets {
		if PromptPresets[i].Name == name {
			return &PromptPresets[i]
		}
	}
	return nil
}
