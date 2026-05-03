# ADR-0012: Connector Abstraction in `internal/connectors/`

## Status

Accepted

## Date

2026-05-02

## Context

Google Keep landed first (#982) as a one-off integration: `internal/keep/protocol/` (wire client), `internal/keep/bridge/` (sync logic + schema mapping), `internal/grpc/api/v1/keep_connector.go`, `<keep-bind-button>`. The shape worked for one integration.

Google Tasks is being added now (this plan). iCloud Reminders is planned. Without a shared abstraction, each integration would copy the Keep shape with its own naming conventions, its own gRPC service, and its own frontend components — three near-duplicate stacks diverging on cosmetics. Worse, the cross-cutting concerns (the `ChecklistSubscription` aggregate from ADR-0011, the `LeaseTable`, the `SyncScheduler`, event constants) would each be implemented three times with subtle variations.

A shared abstraction is needed. The question is *when* and *what shape*.

## Decision

### Shared package: `internal/connectors/`

Hold the cross-cutting types only:

- **`Connector` interface** — `Kind() ConnectorKind`, `Sync(ctx, key) error`, `PausedReason(profile_id) string`, `ForceFullResync(ctx, profile_id) error`. The contract every connector implements. *This is the actual abstraction*; everything else in the shared package serves it.
- **`LeaseTable`** — in-memory cache of `(page, list_name) → (connector_kind, profile_id)`, with `WaitReady(ctx)` for the boot-rebuild gate. Per ADR-0011, derived state.
- **`SyncScheduler`** — single cron-driven scheduler that fans out `Sync(ctx, key)` calls to registered `Connector` impls. One scheduler instance, registered with `pkg/jobs/CronScheduler`, dispatches to N connectors.
- **Event constants** — typed string constants for structured-log event names (`connector.subscribe`, `connector.sync_start`, etc.), so log-grep is uniform across connectors.

### Per-connector packages: `internal/connectors/<vendor>/`

Each connector splits into three sub-packages, named for the layer they occupy (Fowler, *PoEAA*; Evans, *DDD*):

- **`gateway/`** — Gateway pattern (PoEAA). Wire-protocol client only. OAuth, REST, types matching the remote API's vocabulary. No domain logic. (For Keep: gpsoauth + REST + Keep node types. For Tasks: OAuth refresh + Tasks REST CRUD.)
- **`translator/`** — Anti-Corruption Layer (DDD). Named transformations between remote vocabulary and the wiki's `ChecklistItem` shape: `KeepNodeToChecklistItem`, `ChecklistItemToKeepNode`, `TitleAndTagsFromText`, etc. Each transformation is a named function with its own round-trip property test.
- **`sync/`** — orchestrator. Implements the `Connector` interface, holds `SubscriptionStore`, owns cursor handling, idempotence (uid-marker insertion), pause/resume horizon, tombstone-retention hook. Per-connector rate-limit choke (e.g. Tasks' per-user QPS) lives here, not in the shared `SyncScheduler`.

### Wiki-side ports stay consumer-defined

`ChecklistReader`, `ChecklistMutator`, `SyncSuppressor`, `JobEnqueuer` — these are the wiki-side dependencies a connector needs. Each connector package declares its own minimal interface for what it consumes. They are **not** lifted to `internal/connectors/`. This follows the Go idiom (interfaces are defined where they are consumed, not where they are implemented) and keeps the shared package small.

### Refactor-first sequencing

Keep is **lifted into the new shape before Tasks lands**, not after. Per Kent Beck: "Make the change easy, then make the easy change." Both Keep and Tasks were known going in; shaping the abstraction with Tasks already on the roadmap is more honest than building Tasks alongside an unrefactored Keep and extracting later.

## Consequences

### Positive

- Adding iCloud Reminders is mechanical: mirror the `gateway/translator/sync/` triplet, register with `SyncScheduler`, register with `ConnectorService`. No new shared-package surface needed.
- The split between gateway, translator, and sync gives each layer a single responsibility and a natural test boundary.
- The shared package stays small and stable. The fragile, fast-moving code lives in the per-connector packages.

### Negative

- Refactoring Keep before adding Tasks adds one phase of behavior-preserving churn before any new feature lands. (Mitigation: tests pass green throughout; the refactor is mechanical.)
- The "translator" naming is non-obvious for contributors not familiar with DDD. (Mitigation: directory README and ADR cross-link.)

### Neutral

- The `Connector` interface is intentionally narrow. If a fourth integration needs something the interface doesn't expose, the interface grows; that's fine.

## Alternatives considered

- **Build Tasks alongside Keep first, extract abstraction in a third PR once two implementations exist (Fowler's "rule of three").** Rejected. Theoretically cleaner, but the user explicitly preferred the change-easy-first pattern, and both Keep and Tasks were known going in. Waiting for a third concrete implementation to discover an obvious abstraction is ceremony, not discovery.
- **Lift wiki-side ports (`ChecklistReader`, etc.) to `internal/connectors/`.** Rejected. Violates the Go consumer-defined-interface idiom; would couple the shared package to wiki-internal types it doesn't otherwise need.
- **One package per connector, no sub-packages — flat layout.** Rejected. The gateway/translator/sync split is the design; flattening it loses the layer boundaries that make each piece independently testable.

## References

- ADR-0011: The `ChecklistSubscription` aggregate.
- Plan: `now-that-we-landed-groovy-pizza.md` (Critical files, Phasing).
- Fowler, *Patterns of Enterprise Application Architecture* — Gateway.
- Evans, *Domain-Driven Design* — Anti-Corruption Layer.
- Beck, *Tidy First?* — "Make the change easy, then make the easy change."
