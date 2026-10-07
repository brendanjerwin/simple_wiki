+++
identifier = "help_inventory_management"

[wiki]
system = true
+++

#help #inventory

# {{.Title}}

The wiki can track physical inventory — workshop tools, lab supplies, storage bins, and the containers that hold them. Each item is a wiki page built from the `inv_item` template, and containers are pages that list their contents. Moves update both the item page and the container pages automatically.

## Inventory pages

Items use the `inv_item` template and store their current container under an `inventory.container` field. Containers are ordinary pages with an `inventory.items` list. See the operator documentation for the exact template fields.

## Moving items

When an item moves from one container to another, call `MoveInventoryItem`. The wiki removes the item from the old container's `inventory.items` list, adds it to the new container's list, and updates the item page's `inventory.container` field — all in one operation.

## Finding things

- `FindItemLocation` returns the container chain for an item.
- `ListContainerContents` returns every item in a container, recursing into nested sub-containers.

## For Agents

Use `InventoryManagementService` for inventory operations. It is exposed both as gRPC and as MCP tools (auto-generated from the proto).

### MCP Tools

| Tool | Purpose |
|---|---|
| `api_v1_InventoryManagementService_CreateInventoryItem` | Create a new inventory item page with the `inv_item` template structure. |
| `api_v1_InventoryManagementService_MoveInventoryItem` | Move an item from one container to another, updating both sides. |
| `api_v1_InventoryManagementService_ListContainerContents` | List all items in a container, including items in nested containers (read-only). |
| `api_v1_InventoryManagementService_FindItemLocation` | Find which container(s) an item is currently in (read-only). |

## See Also

- [[help-mcp]] — MCP transport and service catalog
