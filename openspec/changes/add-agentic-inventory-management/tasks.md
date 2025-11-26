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
