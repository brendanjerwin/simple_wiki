# ADR-0012: Connector Abstraction in `internal/connectors/`

## Status

Accepted (2026-05-02); revised 2026-05-04 (per-connector floor reduced to `gateway/ + translator/ + adapter.go`; engine owns the entire lifecycle).

## Date

2026-05-02 (original); 2026-05-04 (engine-ownership refinement)

## Context

Google Keep landed first (#982) as a one-off integration: `internal/keep/protocol/` (wire client), `internal/keep/bridge/` (sync logic + schema mapping), `internal/grpc/api/v1/keep_connector.go`, `<keep-bind-button>`. The shape worked for one integration.

Google Tasks was added next (#999). iCloud Reminders is planned (#998). Without a shared abstraction, each integration would copy the Keep shape with its own naming conventions, its own gRPC service, and its own frontend components — three near-duplicate stacks diverging on cosmetics. Worse, the cross-cutting concerns (the `ChecklistBinding` aggregate from ADR-0011, the `LeaseTable`, the `SyncScheduler`, event constants, *and the per-tick reconcile algorithm itself*) would each be implemented three times with subtle variations — a divergence that the user's directive *"I do not want to re-litigate sync edge conditions"* (2026-05-04) was issued to retire.

A shared abstraction is needed. The 2026-05-02 ADR limited it to interface lifting; the 2026-05-04 revision (driven by the audit in [`internal/connectors/MATRIX.md`](../../internal/connectors/MATRIX.md)) extends it to the entire connector lifecycle.

## Decision

### Shared package: `internal/connectors/`

Hold the cross-cutting types AND the algorithm:

- **`Connector` interface** — `Kind() ConnectorKind`, `Sync(ctx, key) error`, `PausedReason(key) (string, bool)`, `ForceFullResync(ctx, key) error`. The dispatch shape every connector exposes for the scheduler / RPC service / lease table. Implementations are produced by the engine wrapping a `BackendAdapter`.
- **`BackendAdapter` interface** — every primitive a backend must implement: pull / push / translate / cursor advance / bind seed + validate / rebuild / list collections / title sync / state codec / error classification / read-by-ref / capability bits. The audited contract lives in [`internal/connectors/adapter.go`](../../internal/connectors/adapter.go), and per-row provenance for every method is in [`MATRIX.md`](../../internal/connectors/MATRIX.md).
- **`internal/connectors/engine/`** — the SyncEngine. Owns the entire lifecycle: per-tick reconcile (pull → classify → decide → push → cursor advance), Bind, Unbind, ForceFullResync, Pause/Resume (with the 7-day horizon), precondition recovery (3-branch path), dead-letter retry (per-item PushFailureCount + NextAttemptAt + threshold), scheduler tick fan-out, sync debouncer (1.5s window + 5s post-success choke), binding store. Per ADR-0015, the engine reads the per-checklist op-log to drive causal divergence classification.
- **`LeaseTable`** — in-memory cache of `(page, list_name) → (connector_kind, profile_id)`, with `WaitReady(ctx)` for the boot-rebuild gate. Per ADR-0011, derived state.
- **`SyncScheduler`** — single cron-driven scheduler that fires the unified 30s tick. Engine's tick-handler dispatches to every registered `Connector`.
- **Event constants** — typed string constants for structured-log event names (`connector.bind`, `connector.unbind`, `connector.sync_start`, etc.), so log-grep is uniform across connectors. (Op-log self-source markers like `connector:<kind>:apply` keep their slug-form per ADR-0015.)

### Per-connector packages: `internal/connectors/<vendor>/`

Each connector splits into:

- **`gateway/`** — Gateway pattern (PoEAA). Wire-protocol client only. OAuth, REST, types matching the remote API's vocabulary. No domain logic. (For Keep: gpsoauth + REST + Keep node types. For Tasks: OAuth refresh + Tasks REST CRUD. For iCloud: app-specific-password Basic Auth + CalDAV verbs.)
- **`translator/`** — Anti-Corruption Layer (DDD). Named transformations between remote vocabulary and the wiki's `ChecklistItem` shape: `KeepNodeToChecklistItem`, `ChecklistItemToKeepNode`, `TaskToChecklistItem`, `ChecklistItemToTaskFields`, `TitleAndTagsFromText`, etc. Each transformation is a named function with its own round-trip property test.
- **`adapter.go`** — implements `BackendAdapter`. Calls into `gateway/` for wire I/O and `translator/` for shape conversion. Holds nothing about the merge algorithm, the cursor advance logic, the bind ceremony, the precondition recovery, or the dead-letter retry — all of those are in the engine. Per-adapter-internal state (Tasks's `wiki:uid` marker handling; Keep's `LabelIDs` cache) lives wholly inside the adapter's `EncodeAdapterState` / `DecodeAdapterState` codec, so the engine never inspects it.

There is **no** `sync/` subpackage anymore. The original ADR-0012 (2026-05-02) had each connector own a `sync/` package with the per-tick algorithm; the 2026-05-04 audit determined that approach left enough algorithmic surface per-adapter to keep growing parallel bug classes (PR #999 shipped a third critical correctness incident before this extraction). Engine-owned lifecycle ELIMINATES the per-connector algorithmic surface.

### Wiki-side ports stay consumer-defined

`ChecklistReader`, `ChecklistMutator`, `SyncSuppressor`, `JobEnqueuer` — these are the wiki-side dependencies the engine needs. The engine package declares the minimal interface for what it consumes. They are **not** lifted to per-connector packages. This follows the Go idiom (interfaces are defined where they are consumed, not where they are implemented).

### Refactor-first sequencing

Keep was lifted into the new shape (`gateway/translator/sync/` triplet) before Tasks landed, per Kent Beck: "Make the change easy, then make the easy change." The 2026-05-04 SyncEngine extraction lifts Keep + Tasks together onto `BackendAdapter` BEFORE iCloud Reminders is added. Both backends collapse onto the same engine; iCloud is then a single `adapter.go` against the audited contract — not a third 1300-line connector replicating the same merge algorithm.

The strictest-behavior-wins resolution rule (ADR-0015, *audited refinement*) means uplifts surfaced by the audit are applied to laggard adapters in the same PR: Keep gains the 3-branch precondition recovery; Tasks gains dead-letter retry; both gain the engine's debounce post-success choke.

## Consequences

### Positive

- Adding iCloud Reminders is mechanical: one `adapter.go` against the audited `BackendAdapter` contract. No new shared-package surface needed; no new sync-algorithm code; no new bind-ceremony code.
- The split between gateway, translator, and the engine gives each layer a single responsibility and a natural test boundary.
- The shared package owns the algorithm. The fragile parts (precondition recovery, dead-letter retry, cursor advance, bind ceremony's mutex+fan-out invariants) are written once and tested at engine level with an in-memory FakeAdapter, plus a parity test that runs canonical scenarios through every real adapter.
- Adapter-specific edge cases that *should* be shared can no longer drift apart silently — adding a new method to `BackendAdapter` is a compile error for every backend that doesn't implement it.

### Negative

- Refactoring Keep + Tasks before adding iCloud adds a phase of behavior-preserving churn before any new feature lands. (Mitigation: tests stay green throughout via parity tests; the refactor is mechanical.)
- The 2026-05-04 extraction is a large diff (engine package + collapse of two connectors + verb rename + frontmatter migration). Reviewable by phase boundary.
- The "translator" naming is non-obvious for contributors not familiar with DDD. (Mitigation: directory README and ADR cross-link.)

### Neutral

- The `BackendAdapter` interface is intentionally narrow. If a fourth integration needs something the interface doesn't expose, the interface grows — that's the whole point of the compile-time enforcement.

## Alternatives considered

- **Keep `sync/` per connector; engine just owns the merge classifier.** This was the 2026-05-03 ADR-0015 sketch. Rejected on 2026-05-04 because shipping iCloud as a third per-connector `sync/` re-implementation would re-introduce the bug-class the user's directive is meant to retire.
- **Build Tasks alongside Keep first, extract abstraction in a third PR once two implementations exist (Fowler's "rule of three").** Rejected (2026-05-02). Theoretically cleaner, but the user explicitly preferred the change-easy-first pattern, and both Keep and Tasks were known going in.
- **Lift wiki-side ports (`ChecklistReader`, etc.) to `internal/connectors/`.** Rejected. Violates the Go consumer-defined-interface idiom; would couple the shared package to wiki-internal types it doesn't otherwise need.
- **Capability bits to let an adapter opt out of an engine behavior.** Rejected (2026-05-04, see ADR-0015 strictest-behavior-wins rule). Capability bits perpetuate the per-adapter divergence the engine extraction is meant to retire. True capability differences (parent-child task hierarchies on Tasks but not Keep) are documented per-row in MATRIX.md, not enabled by general escape hatches.

## References

- ADR-0011: The `ChecklistBinding` aggregate.
- ADR-0015: Per-checklist operation log + engine-owned merge rule (audited 2026-05-04).
- Plan: `to-build-issue-998-warm-glacier.md` (the 2026-05-04 extraction's phasing).
- [`internal/connectors/MATRIX.md`](../../internal/connectors/MATRIX.md) — audited per-row provenance for the `BackendAdapter` interface.
- Fowler, *Patterns of Enterprise Application Architecture* — Gateway.
- Evans, *Domain-Driven Design* — Anti-Corruption Layer.
- Beck, *Tidy First?* — "Make the change easy, then make the easy change."
