+++
identifier = "help_page_import"

[wiki]
system = true
+++

#help #import

# {{.Title}}

Pages can be created or updated in bulk from a CSV file. Each row becomes one wiki page: scalar columns become frontmatter fields, array columns support add/remove operations, and cells marked `[[DELETE]]` remove fields.

## CSV format

- One row per page.
- Required columns include `identifier` and `template`.
- Array fields use `+value` to ensure a value exists or `-value` to remove it.
- `[[DELETE]]` in any scalar cell removes that field from the page.

## Preview before import

Always run `ParseCSVPreview` first. It returns the records that would be written, validation errors per row, and create/update counts. No pages are modified.

## Running the import

`StartPageImportJob` enqueues a background job and returns a `job_id`. Poll `SystemInfoService.GetJobStatus` or use the job-specific status path to watch progress.

## For Agents

Use `PageImportService` for bulk imports. It is exposed both as gRPC and as MCP tools (auto-generated from the proto).

### MCP Tools

| Tool | Purpose |
|---|---|
| `api_v1_PageImportService_ParseCSVPreview` | Dry-run a CSV import and return the would-be records plus validation errors (read-only). |
| `api_v1_PageImportService_StartPageImportJob` | Enqueue a background job that creates or updates one page per CSV row (long-running). |

## See Also

- [[help-mcp]] — MCP transport and service catalog
- [[help-system-info]] — Background job queue status
