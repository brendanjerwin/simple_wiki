# ADR-0015: Per-Checklist Operation Log + Engine-Owned Merge Rule

## Status

Accepted (2026-05-03); revised 2026-05-04 with audited `BackendAdapter` interface (see "Audited refinement" below).

## Date

2026-05-03 (original); 2026-05-04 (audited refinement, [`internal/connectors/MATRIX.md`](../../internal/connectors/MATRIX.md))

## Context

ADR-0011 established the `ChecklistSubscription` aggregate. ADR-0012 split each connector into `gateway/translator/sync/` and lifted the cross-cutting types (`Connector`, `LeaseTable`, `SyncScheduler`, event constants) into `internal/connectors/`. Both Keep and Google Tasks were implemented against that shape.

ADR-0012 explicitly stopped at *interface* lifting. The actual sync algorithm — the per-item three-way merge that decides "push wiki / apply remote / no-op / conflict" — was left in each connector's `sync/connector.go`. Two near-identical implementations.

Both implementations rely on a **fingerprint-snapshot baseline**:

- `Synced{Field}` (Keep: `SyncedText`/`SyncedChecked`/`SyncedSortValue`; Tasks: `SyncedTitle`/`SyncedNotes`/`SyncedStatus`/`SyncedDue`) — a snapshot of the wiki-derived fields at the last successful round-trip.
- The merge rule classifies items by computing a current fingerprint and comparing to the snapshot: `wiki_diverged := wiki_fp != synced_fp`.

This worked for steady state but bit us hard during PR #999 manual smoke testing. The failure mode: the cursor advances to `max(Task.updated) - 1s` (a deliberate safety buffer for at-least-once delivery), so the next inbound poll re-fetches the same revision. Without an etag-skip and a divergence guard, the inbound apply rewrites the wiki to remote state on every tick — clobbering any local edit that happened since the last successful round-trip.

We added two guards to Tasks (`inbound_skipped_same_etag` for the etag-equal case, `wiki_diverged_skipped_inbound` for the value-mismatch case). Both work. Both are also a **band-aid**:

1. **The fingerprint compare is value-based, not causal.** It cannot distinguish "user just edited locally" from "user re-edited to a value that happens to match an older state." The classification is approximate.
2. **The algorithm is duplicated per backend.** When iCloud Reminders lands as the third connector ("we are going to be adding another binding service later as well. all with the same interactions"), we will write the same merge logic for the third time. The bug-class we just hit on Tasks (which Keep had handled, but Tasks's parallel re-implementation omitted) is structurally guaranteed to recur unless the algorithm is shared.
3. **There is no audit trail.** "Why did this item get checked at 13:53:39?" has no answer. Was it the user? An inbound apply? Which backend? Which tick?

The user direction (2026-05-03), after this exact bug surfaced for the third time in PR #999:

> *"we need to keep the operations log, at the algorithm level so that it helps with all implementations. otherwise it'll be hacks and per-system fixes all the time."*

## Decision

### A per-checklist **operation log** at the wiki layer

Each checklist gains an `events` slice in its frontmatter, alongside the existing `items` and `tombstones`:

```toml
[wiki.checklists.dad]
[[wiki.checklists.dad.items]]
uid = "01KQ..."
text = "Sun: Make Dinner"
checked = false
# … existing item fields …

[[wiki.checklists.dad.tombstones]]
uid = "01KQ..."
deleted_at = "2026-04-30T12:00:00Z"
gc_after = "2026-05-07T12:00:00Z"

[[wiki.checklists.dad.events]]
seq = 1247
ts = "2026-05-03T13:53:06.123Z"
src = "user:brendanjerwin@gmail.com"
op = "toggle"
uid = "01KQ..."
checked = false

[[wiki.checklists.dad.events]]
seq = 1248
ts = "2026-05-03T13:53:39.456Z"
src = "connector:google_tasks:apply"
op = "set_status"
uid = "01KQ..."
checked = true
```

The log is **a checklist concept, not a binding concept** — see "Why checklist-level" below.

### Event entry schema

- **`seq`** (int64): Monotonic per-checklist counter. Assigned at write time under the same lock that mutates `items`. Never reused, never reset.
- **`ts`** (RFC3339): Wall-clock when the mutation was applied. Diagnostic only — `seq` is the causal authority.
- **`src`** (string): One of `user:<email>`, `connector:<kind>:apply`, `connector:<kind>:push_recovery`, `system:<rule>`, or `migration:<reason>`. The `<kind>` matches the existing `ConnectorKind` enum (`google_keep`, `google_tasks`, `icloud_reminders`, …).
- **`op`** (string): One of `add`, `delete`, `toggle`, `set_text`, `set_due`, `set_description`, `set_tags`, `set_sort_order`, `baseline`. New ops added as wiki capabilities grow.
- **`uid`** (string): The item's wiki ULID. Empty only for whole-list ops if any are added later.
- **field deltas** (typed): Just the fields the op mutates (`checked`, `text`, `due`, etc.). Old values are reconstructible from the previous event for the same uid.

### Engine-owned lifecycle (audited 2026-05-04)

The `internal/connectors/engine` package owns the **entire** connector lifecycle, not just the per-item merge classification. The original (2026-05-03) sketch limited the engine to "merge rule"; the audit (2026-05-04, see [`MATRIX.md`](../../internal/connectors/MATRIX.md)) determined that lifting only the merge rule leaves bind/unbind, force-resync, pause/resume, precondition recovery, dead-letter retry, scheduler tick, debouncer, and binding store as parallel per-connector implementations — exactly the divergence the user's directive *"I do not want to re-litigate sync edge conditions"* is meant to retire.

The engine owns:

- Op-log read against `LastSyncedSeq`.
- Per-item divergence classification using the log (causal, not value-based).
- The 4-cell merge: no-op / push-wiki / apply-remote / conflict-remote-wins.
- **Bind / Unbind ceremony** (mutex + fan-out re-read + profile write + lease take, per ADR-0011).
- **ForceFullResync** — driven by adapter's `RebuildAdapterState` primitive.
- **Pause/Resume** (auth-failed transition + 7-day horizon → reseed). Adapter signals auth failure via `ClassifyError(err) == ErrorClassAuthFailed`.
- **Precondition recovery** (3-branch: remote-deleted / remote-unchanged-repatch / remote-authoritative-apply). Triggered by `ErrorClassPreconditionFailed`. Tasks's existing 3-branch path becomes engine policy; Keep's stage3-500-on-stale-baseVersion adopts it.
- **Dead-letter retry** (per-item PushFailureCount / NextAttemptAt with `deadLetterThreshold=10` + exponential backoff). Keep's existing implementation becomes engine policy; Tasks adopts it.
- Etag / precondition handling generically (delegated to adapter via `ClassifyError`).
- Suppressor wiring around the apply pass.
- Cursor advance with adapter-specific safety buffer (delegated via `AdvanceCursor`).
- Scheduler tick (the unified 30s fan-out via `connectors.SyncScheduler`).
- Sync debouncer (1.5s window + 5s post-success choke; both backends adopt the choke).
- Binding store (per-profile mutex + TOML serialization with adapter-opaque AdapterState subtree).
- Tombstone GC interaction.

Each backend provides a `BackendAdapter` whose contract is the **audited 2026-05-04 interface** in [`internal/connectors/adapter.go`](../../internal/connectors/adapter.go). Per-row provenance for every method on that interface is in [`MATRIX.md`](../../internal/connectors/MATRIX.md). Adopting the audited interface as the source-of-truth means a single edit to `adapter.go` keeps all four documents (this ADR, the matrix, the interface, the test parity scenarios) in sync.

#### Audited interface (2026-05-04, summary)

```go
type BackendAdapter interface {
    Kind() ConnectorKind

    // Per-tick reconcile primitives.
    PullRemote(ctx, binding) (RemotePullResult, error)
    InsertRemote(ctx, binding, item) (RemoteRef, error)
    PatchRemote(ctx, binding, ref, item) (RemoteRef, error)
    DeleteRemote(ctx, binding, ref) error

    // Translate.
    RemoteToWiki(remote) (WikiItem, error)
    WikiToRemote(wiki) (RemoteItem, error)

    // Cursor advance (opaque body per-adapter).
    AdvanceCursor(binding, result) Binding

    // Bind ceremony.
    SeedBindingState(ctx, profileID, remoteHandle) (AdapterState, error)
    ValidateRemoteBinding(ctx, profileID, remoteHandle) error

    // Force full resync.
    RebuildAdapterState(ctx, binding) (AdapterState, error)

    // Title sync (kept from v1).
    FetchRemoteListTitle(ctx, profileID, remoteHandle) (string, bool, error)

    // Bind UI picker support.
    ListRemoteCollections(ctx, profileID) ([]RemoteCollection, error)

    // Adapter-state codec (engine treats AdapterState as opaque).
    EncodeAdapterState(state) (map[string]any, error)
    DecodeAdapterState(raw) (AdapterState, error)

    // Capability bits.
    SupportsSubtasks() bool

    // Read-by-ref (for the post-412 precondition-recovery pull).
    ReadRemoteByRef(ctx, binding, ref) (RemoteItem, error)

    // Error classification — routes engine recovery by class.
    ClassifyError(err) ErrorClass
}
```

The audited interface ADDS to the original 2026-05-03 sketch: `AdvanceCursor`, `SeedBindingState`, `ValidateRemoteBinding`, `RebuildAdapterState`, `FetchRemoteListTitle` (existed pre-engine), `ListRemoteCollections`, `EncodeAdapterState`/`DecodeAdapterState`, `ReadRemoteByRef`. Each addition has a row in [`MATRIX.md`](../../internal/connectors/MATRIX.md) with the audit-time justification.

#### Historical: original 2026-05-03 sketch

```go
// Original sketch — superseded by the audited 2026-05-04 contract above.
type BackendAdapter interface {
    Kind() ConnectorKind
    PullRemote(ctx context.Context, sub Subscription) (RemotePullResult, error)
    InsertRemote(ctx context.Context, sub Subscription, item WikiItem) (RemoteRef, error)
    PatchRemote(ctx context.Context, sub Subscription, ref RemoteRef, item WikiItem) (RemoteRef, error)
    DeleteRemote(ctx context.Context, sub Subscription, ref RemoteRef) error
    RemoteToWiki(remote RemoteItem) (WikiItem, error)
    WikiToRemote(wiki RemoteItem) (RemoteItem, error)
    SupportsSubtasks() bool
    ClassifyError(err error) ErrorClass
}
```

The original sketch limited the engine to the merge rule. The 2026-05-04 audit determined that approach left too much per-connector lifecycle code as parallel implementations. The audited interface lets the engine own the *entire* lifecycle.

### Strictest-behavior-wins resolution rule (2026-05-04)

When the audit surfaces edge-case behaviors that one connector handles and the other doesn't, the engine adopts the *better* implementation as policy. The laggard adapter is uplifted in the same PR. **No capability bits to opt out** — that's the half-shared-engine trap.

Documented per-row exceptions in `MATRIX.md` are the only escape hatch, with explicit justification. The matrix has two such exceptions in v1: subtask handling (Tasks supports parent-child; Keep doesn't have the concept — true capability difference) and UID-marker insertion (translator-internal; engine doesn't see markers — adapter-internal concern).

The strictest-behavior-wins rule, applied to MATRIX.md's row dispositions, produces these PR-15 uplifts:

- **Keep gains the 3-branch precondition recovery** (originally Tasks-only). Keep's stage3-500 on stale baseVersion maps to `ErrorClassPreconditionFailed`; the engine runs the same recovery.
- **Tasks gains dead-letter retry** (originally Keep-only). Per-item `PushFailureCount` / `NextAttemptAt` enter the engine's bookkeeping for every adapter.
- **Keep gains explicit Pause/Resume state** with `PausedReason` (originally Tasks-only at the field level; Keep had it implicitly via "is master token empty?").
- **Keep gains the post-success debounce choke** (originally Tasks-only).
- **Keep gains self-event op-log emission** after every successful push primitive (originally Tasks-only).

### Per-binding state shrinks

The 2026-05-04 audit also resolved the verb drift between Keep (which used `Bind/BindingKey`) and Tasks (which used `Subscribe/Subscription`): the engine extraction unifies on **`Binding`** because the data is genuinely a binding between a wiki checklist and a remote list, not a subscription (which implies passive consumption). The post-extraction shape:

Before (per-connector `Subscription`, parallel implementations):

```go
type Subscription struct {
    Page, ListName, RemoteListID string
    ItemIDMap     map[string]string                   // adapter-specific
    ItemEtags     map[string]string                   // adapter-specific
    SyncedItems   map[string]ItemSyncState            // ALGORITHM STATE
    LastUpdatedMin time.Time                          // adapter-specific
    State         SubscriptionState
    // …
}
```

After (engine-owned `connectors.Binding`, one type, used by every adapter):

```go
type Binding struct {
    // Identity (the aggregate root per ADR-0011).
    ProfileID    wikipage.PageIdentifier
    Page         string
    ListName     string
    RemoteHandle string

    // Display.
    RemoteListTitle string

    // Engine-owned cursor (causal, not value-based).
    LastSyncedSeq int64

    // Engine-owned lifecycle state.
    State        BindingState
    PausedReason string
    PausedAt     time.Time
    BoundAt      time.Time

    // Engine-owned per-binding scheduling.
    LastSuccessfulSyncAt time.Time

    // Adapter-opaque state subtree.
    AdapterState AdapterState
}
```

`AdapterState` is `map[string]any` — opaque to the engine, encoded/decoded by the adapter via `EncodeAdapterState`/`DecodeAdapterState`. It holds whatever the adapter needs (Tasks: `item_id_map`, `item_etags`, `last_updated_min`; Keep: `item_mapping` with per-item `ServerID`, `BaseVersion`, `ClientID`, `PushFailureCount`, `NextAttemptAt`). `SyncedItems` is gone — replaced by the op-log scan.

### Causal divergence rule

Per item, on each tick, the engine looks at events with `seq > binding.LastSyncedSeq`:

- No events for this uid → `¬wiki_diverged`.
- Latest event is `src=user:…` → `wiki_diverged`.
- Latest event is `src=connector:<this>:…` → `¬wiki_diverged` (our own apply, idempotent re-fetch).
- Latest event is `src=connector:<other>:…` → `wiki_diverged` (cross-connector — defer to that connector's authority).
- Latest event is `src=migration:…` → `¬wiki_diverged`.

Combined with `remote_diverged` (computed from the adapter's pull result vs the previous remote snapshot), the merge produces the same 4-cell decision Keep already documents in `connector.go:1527-1554` — but driven by causality instead of by value compare.

### Compaction

Events with `seq < min(LastSyncedSeq across all bindings on this checklist)` AND `ts older than 30 days` are GC'd by the lazy migration walker that already prunes tombstones. A checklist with no bindings keeps all its events forever (cheap; logs are tiny rows; future bindings get history for free).

### Migration

On first read after deploy, a checklist with `events` absent and `items` non-empty gets a synthesized `seq=0, src=migration:initial_baseline, op=baseline` event per existing item. Subsequent edits start logging from `seq=1`.

Existing bindings migrate at first tick after deploy: `LastSyncedSeq = max(seq) at read time`, treating the current wiki state as the synced baseline. `SyncedItems` is dropped from the persisted state in the same write. The Phase 7 eager frontmatter migration (cf. plan `to-build-issue-998-warm-glacier.md`) handles the simultaneous `subscriptions[]` → `bindings[]` key rename and the legacy fingerprint-field collapse into the new `adapter_state` subtree.

## Consequences

### Positive

- The merge rule is **causal**, not value-based. "Did the user just edit locally" has a deterministic answer: there exists an event with `seq > LastSyncedSeq` and `src=user:…`. No more "fingerprint coincidence" edge cases.
- The merge rule is **shared** across all backends. Adding iCloud Reminders means writing one `BackendAdapter` implementation. The merge logic is not re-implemented — it is reused from the engine. **Missing behavior becomes a compile error**, not a behavior gap.
- The log is a **first-class audit trail**. "Why did this item get checked at 13:53:39?" answers itself: read the events log. Same for support, debugging, and forensic reconstruction.
- Cross-connector exclusivity stays enforced by the LeaseTable, but the engine no longer needs that to be the case for its own correctness — `connector:<other>:…` events are first-class in the rule.
- The schema is forward-compatible: new ops (`set_priority` for iCloud, `assign_to` for Tasks) just add new entries; old engines tolerate unknown ops as `wiki_diverged` defensively.

### Negative

- **Substantial surface change**. ChecklistMutator gains an emit-event step on every mutation path. Both connector packages collapse into adapters. Both binding schemas change. Migrations land in the same release.
- Frontmatter grows by one row per mutation. Compaction handles steady-state size, but a busy household chore chart could see ~50 events/day = ~30KB/month uncompacted. Acceptable for a household-scale wiki; revisit if ever multi-tenant.
- `LastObservedWiki*` and the dead-letter-retry rule on Keep get re-expressed against the log (they currently key off field-fingerprint). Mechanical port; no semantic change.

### Neutral

- The `seq` counter is per-checklist, not per-binding or global. Conflict-free across checklists, deterministic within one. No global ordering across checklists is implied or needed.
- The engine ships with an in-memory fake adapter for tests, so engine-level rule tests don't depend on either Keep or Tasks fakes.

## Why checklist-level (not binding-level)

The log records mutations to a wiki checklist. That is wiki-truth, not connector-truth. Three reasons it has to live with the checklist:

1. **Single source of truth.** A wiki edit happens once. If the log were per-binding, an edit on a checklist with N bindings would be written N times — race-prone, bloating, and a brand-new binding would have no history of edits made before it existed.
2. **Cross-connector is on the roadmap.** Even if we keep one-binding-per-checklist enforcement via the LeaseTable, the engine has to be able to ask "did any *other* connector apply to this checklist since my last tick" — which requires a shared chronicle, not N parallel ones.
3. **Tombstones already work this way.** They live at `wiki.checklists.X.tombstones` for the same reason: a deletion is a wiki fact, not a binding fact. The new event log sits right next to them.

The cursor (`LastSyncedSeq`) is binding state. Each binding tracks its own position in the shared chronicle.

## Why now (not as a follow-up)

The fingerprint-snapshot approach **just shipped a third critical correctness bug** during PR #999 manual smoke. We caught it; the user wired the band-aid; both Keep and Tasks now have the divergence guard. But:

- The band-aid is value-based, not causal. There is a known class of edge cases (concurrent edits to identical values, second-rapid-edit-after-apply) it cannot classify correctly.
- Shipping #999 with two duplicated implementations of "almost the same algorithm" is the exact pattern the user has been pushing against (`feedback_extract_engine_when_n2.md`). iCloud is next on the roadmap; the third re-implementation would land before we got to fix the first two.
- The user's directive: *"has to be done as part of this PR in order for it to work reliably."*

So this ADR is in-scope for #999. The implementation is staged into discrete TDD-shaped phases (see plan file Phase 15).

## Supersedes

- The fingerprint-baseline approach in Keep (`SyncedText/Checked/SortValue`).
- The fingerprint-baseline approach in Tasks (`SyncedTitle/Notes/Status/Due`).
- The `wiki_diverged_skipped_inbound` and `inbound_skipped_same_etag` band-aid guards added to Tasks in commit `dcda37e2`. The engine's causal rule subsumes both.

## See also

- ADR-0011 (`ChecklistBinding` aggregate)
- ADR-0012 (`internal/connectors/` abstraction)
- [`internal/connectors/MATRIX.md`](../../internal/connectors/MATRIX.md) — the audited per-row provenance for the `BackendAdapter` interface in `internal/connectors/adapter.go`.
- `feedback_extract_engine_when_n2.md` (the meta-rule this ADR operationalizes)
- `feedback_function_contract_purity.md` (functions do their job or error — the engine's primitive contract)
