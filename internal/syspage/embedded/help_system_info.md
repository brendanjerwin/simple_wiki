+++
identifier = "help_system_info"

[wiki]
system = true
+++

#help #system

# {{.Title}}

The server exposes runtime information about itself: build version, the Tailscale identity of the requesting user, and the live status of background-job queues.

## Version

`GetVersion` returns the Git commit hash, build timestamp, and (when available) the Tailscale identity of the calling user.

## Job queues

`GetJobStatus` returns the current depth, high-water mark, and active flag for every background-job queue. Use it to check whether an import job, search index job, or file scan is still running.

## Streaming updates

`StreamJobStatus` provides real-time streaming queue updates over gRPC, but it is **not** exposed as an MCP tool because MCP's request/response model cannot represent open streams. Use gRPC or `wiki-cli` for streaming.

## For Agents

Use `SystemInfoService` for runtime and queue status. It is exposed both as gRPC and as MCP tools (auto-generated from the proto).

### MCP Tools

| Tool | Purpose |
|---|---|
| `api_v1_SystemInfoService_GetVersion` | Server commit hash, build time, and Tailscale identity of the caller (read-only). |
| `api_v1_SystemInfoService_GetJobStatus` | Current depth, high-water mark, and active flag for every background-job queue (read-only). |

`StreamJobStatus` is a streaming RPC and is intentionally omitted from the MCP tool surface.

## See Also

- [[help-mcp]] — MCP transport and service catalog
- [[help-page-import]] — Background import jobs whose progress is observable here
