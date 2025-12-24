# Change: Add Agentic Inventory Item Management

## Why

The wiki already has a solid inventory data model (containers, items, frontmatter relationships) and template functions for displaying inventory contents. However, managing inventory—creating items, moving them between containers, and auditing what's where—requires manual editing of frontmatter TOML. This is tedious for household use cases like "I just put the batteries in the drawer" or "where did I put the screwdriver?"

AI agents should be able to help users manage their physical household inventory through natural language commands, making the wiki a practical household inventory system. This feature provides the API foundation that any agentic interface (MCP tools, CLI agents, future integrations) can use.

## What Changes

- **New gRPC API endpoints** for inventory operations:
  - `CreateInventoryItem` - Create a new inventory item page with proper frontmatter
  - `MoveInventoryItem` - Change an item's container (handles both `inventory.container` on the item AND `inventory.items` on containers)
  - `ListContainerContents` - Get all items in a container (direct and nested)
  - `FindItemLocation` - Find which container(s) an item is in

- **Inventory normalization background job** - A scheduled job (cron) that normalizes inventory relationships:
  - Creates pages for items that exist only in `inventory.items` lists but don't have their own page yet
  - Uses the `inv_item` template structure (see `server/site.go:326-365`):
    - Sets `inventory.container` to the parent container
    - Sets `inventory.items` to empty array
    - Auto-generates `title` from identifier
    - Includes the standard inventory item markdown template
  - Detects anomalies (items in multiple containers, orphaned items, circular references)
  - Reports anomalies by updating a designated wiki page (e.g., `inventory_audit_report`)
  - Uses existing `pkg/jobs` infrastructure with new cron scheduling capability

- **Agent-friendly response formats** - Structured responses suitable for LLM consumption with clear success/failure indicators and human-readable summaries

- **Household-friendly defaults** - Sensible defaults for common scenarios (auto-generate titles from identifiers)

## Impact

- Affected specs: New `inventory-management` capability (no existing specs to modify)
- Affected code:
  - `api/proto/api/v1/` - New proto definitions for inventory RPCs
  - `internal/grpc/api/v1/` - Server implementations
  - `pkg/jobs/` - Add cron scheduling capability for normalization job
  - `server/` - Wire up cron scheduler on startup, extract shared item page creation logic
  - `templating/` - May leverage existing inventory query functions
