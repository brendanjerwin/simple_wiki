+++
identifier = "help_frontmatter"

[wiki]
system = true
+++

#help #frontmatter

# {{.Title}}

Every wiki page starts with frontmatter: structured metadata written as TOML between `+++` fences at the top of the markdown file. The wiki uses frontmatter for identifiers, system flags, macro state, and any custom fields you add.

## What frontmatter looks like

```toml
+++
identifier = "help_frontmatter"

[wiki]
system = true
+++
```

Do not edit the `identifier` field unless you want to rename the page. The `[wiki]` block holds system keys like `system`.

## Editing frontmatter

You can edit frontmatter directly in the page editor, but most wiki features store their data in reserved `wiki.*` namespaces that are rejected by generic frontmatter writes. Use the dedicated service for each feature instead:

- Checklists — `ChecklistService` and [[help-macro-checklist]]
- Maps — `MapService` and [[help-macro-map]]
- Surveys — `SurveyService` and [[help-macro-survey]]
- Agent schedules / chat context — `AgentMetadataService` and [[help-scheduled-agents]]

## For Agents

Use `Frontmatter` for reading and writing generic frontmatter. It is exposed both as gRPC and as MCP tools (auto-generated from the proto).

### MCP Tools

| Tool | Purpose |
|---|---|
| `api_v1_Frontmatter_GetFrontmatter` | Read the structured frontmatter for a page. |
| `api_v1_Frontmatter_MergeFrontmatter` | Merge the supplied key/value tree into the existing frontmatter. |
| `api_v1_Frontmatter_ReplaceFrontmatter` | Replace the entire frontmatter object. |
| `api_v1_Frontmatter_RemoveKeyAtPath` | Remove a key at a dotted or indexed path. |

### Reserved namespaces

- The top-level `agent` namespace is rejected by all writes. Use `AgentMetadataService` for schedules, chat-context, and background-activity mutations under that namespace.
- The top-level `wiki` namespace is also rejected by `MergeFrontmatter`, `ReplaceFrontmatter`, and `RemoveKeyAtPath`. The dedicated services above are the only legitimate mutation paths for their respective sub-trees.

## See Also

- [[help-mcp]] — MCP transport and service catalog
- [[help-macro-checklist]] — `ChecklistService` tool reference
- [[help-macro-map]] — `MapService` tool reference
- [[help-macro-survey]] — `SurveyService` tool reference
- [[help-scheduled-agents]] — `AgentMetadataService` schedules
