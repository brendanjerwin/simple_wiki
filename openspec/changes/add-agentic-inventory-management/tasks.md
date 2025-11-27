# Tasks: Agentic Inventory Item Management

## 1. Infrastructure

- [x] 1.1 Add cron scheduling capability to `pkg/jobs/` (robfig/cron is already vendored)
- [x] 1.2 Wire up cron scheduler in `server/Site` initialization
- [x] 1.3 Extract shared item page creation logic from `server/site.go` into reusable function

## 2. Proto Definitions

- [x] 2.1 Define `CreateInventoryItemRequest/Response` messages
- [x] 2.2 Define `MoveInventoryItemRequest/Response` messages
- [x] 2.3 Define `ListContainerContentsRequest/Response` messages
- [x] 2.4 Define `FindItemLocationRequest/Response` messages
- [x] 2.5 Add RPC methods to `InventoryManagementService`
- [x] 2.6 Run `go generate ./...` to regenerate proto code

## 3. gRPC Server Implementation

- [x] 3.1 Implement `CreateInventoryItem` - create item page with `inv_item` template
- [x] 3.2 Implement `MoveInventoryItem` - update both `inventory.container` and `inventory.items`
- [x] 3.3 Implement `ListContainerContents` - leverage existing `ShowInventoryContentsOf` logic
- [x] 3.4 Implement `FindItemLocation` - query frontmatter index for `inventory.container` matches

## 4. Normalization Job

- [x] 4.1 Create `InventoryNormalizationJob` implementing `jobs.Job` interface
- [x] 4.2 Implement scan for items in `inventory.items` without their own page
- [x] 4.3 Implement page creation for orphaned items using `inv_item` template
- [x] 4.4 Implement anomaly detection (multiple containers, circular refs)
- [x] 4.5 Implement anomaly report wiki page generation/update
- [x] 4.6 Schedule job on cron (configurable interval)

## 5. Testing

- [x] 5.1 Unit tests for each gRPC endpoint
- [x] 5.2 Unit tests for normalization job logic
- [x] 5.3 Integration tests for move operations (both container approaches)
- [x] 5.4 Integration tests for normalization job page creation

## 6. UI Implementation

- [x] 6.1 Create `inventory-ui/spec.md` under change specs
- [x] 6.2 Create `inventory-action-service.ts` skeleton and tests
- [x] 6.3 Implement `inventory-action-service.ts` with gRPC client integration
- [x] 6.4 Create `inventory-add-item-dialog.ts` skeleton and tests
- [x] 6.5 Implement `inventory-add-item-dialog.ts` component
- [x] 6.6 Create `inventory-add-item-dialog.stories.ts`
- [x] 6.7 Create `inventory-move-item-dialog.ts` skeleton and tests
- [x] 6.8 Implement `inventory-move-item-dialog.ts` component
- [x] 6.9 Create `inventory-move-item-dialog.stories.ts`
- [x] 6.10 Create `inventory-find-item-dialog.ts` skeleton and tests
- [x] 6.11 Implement `inventory-find-item-dialog.ts` component
- [x] 6.12 Create `inventory-find-item-dialog.stories.ts`
- [x] 6.13 Add `addInventoryMenu()` to `simple_wiki.js`
- [x] 6.14 Update `index.tmpl` with dialog element slots
- [x] 6.15 Register components in `index.ts`
- [ ] 6.16 Manual integration testing
