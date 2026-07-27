+++
identifier = "help_file_storage"

[wiki]
system = true
+++

#help #files

# {{.Title}}

Files and images can be attached to wiki pages by dragging them into the editor or by using the `FileStorageService` API. Each uploaded file is addressed by its content hash (SHA-256), so the same bytes always map to the same URL regardless of filename.

## Uploading files

Drag and drop a file into any page editor and the wiki inserts a markdown image/link block for you. The file is stored once and referenced by hash, so renaming or re-uploading identical content does not create duplicates.

## File URLs

Uploaded files are served from `/uploads/<hash>?filename=...`. The `hash` is stable; the `filename` query parameter is only used for the download name.

## Deleting files

Deleting a file removes the stored bytes. Existing markdown references will then 404 until the link is removed or the file is re-uploaded.

## For Agents

Use `FileStorageService` for uploading, querying, and deleting files. It is exposed both as gRPC and as MCP tools (auto-generated from the proto).

### MCP Tools

| Tool | Purpose |
|---|---|
| `api_v1_FileStorageService_UploadFile` | Upload raw bytes with a filename; returns the content hash and public URL. |
| `api_v1_FileStorageService_GetFileInfo` | Return metadata (`size_bytes`) for a file by hash. |
| `api_v1_FileStorageService_DeleteFile` | Delete an uploaded file by hash. |

## See Also

- [[help-mcp]] — MCP transport and service catalog
