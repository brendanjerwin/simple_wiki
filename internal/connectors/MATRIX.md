# Connector Behavior Comparison Matrix

This matrix is the **audit deliverable for Phase 0** of the SyncEngine extraction (plan: `to-build-issue-998-warm-glacier.md`). It maps every behavior the existing connectors implement today to its disposition in the post-extraction world. The `BackendAdapter` interface in `internal/connectors/adapter.go` is grown to match the **Adapter primitive(s)** column.

**Resolution policy:** *strictest-behavior-wins*. When one connector handles an edge case better than the other, the engine adopts that behavior and the laggard is uplifted in the same PR. `stays-per-adapter` is reserved for true capability differences (e.g., Tasks supports parent-child task hierarchies; Keep doesn't have the concept), not for accidental divergence.

**Verb note:** the matrix uses **Bind/Unbind** throughout (the post-rename names). Phase 6 renames `Subscribe` → `Bind` everywhere.

## Behavior matrix

| # | Behavior | Keep impl (file:line) | Tasks impl (file:line) | Disposition | Adapter primitive(s) | Parity test |
|---|----------|-----------------------|------------------------|-------------|----------------------|-------------|
| 1 | Per-tick reconcile (inbound apply + outbound push + cursor advance) | `google_keep/sync/connector.go` SyncToKeep (≈630–1400) | `google_tasks/sync/connector.go:197-340` (Sync, runSyncPasses) | `lifted-as-is` (engine writes its own orchestration; both connectors collapse to primitives) | `PullRemote`, `InsertRemote`, `PatchRemote`, `DeleteRemote`, `RemoteToWiki`, `WikiToRemote`, `AdvanceCursor` | yes |
| 2 | Bind ceremony (acquire `(page,list)` mutex → fan-out re-read all profiles → write profile → take lease, per ADR-0011) | `google_keep/sync/connector.go` Bind (≈414–498) | `google_tasks/sync/lifecycle.go:213-366` (Subscribe) | `lifted-as-is` | `SeedBindingState(ctx, binding, remoteHandle) (AdapterState, error)` (e.g., Tasks's `seedIDMapForSubscribe`); `ValidateRemoteBinding(ctx, remoteHandle) error` (e.g., Tasks's `ErrTasksListHasSubtasks` refuse-to-bind) | yes |
| 3 | Unbind ceremony (mutex + profile write + lease release) | `google_keep/sync/connector.go` Unbind (≈2173–2198) | `google_tasks/sync/lifecycle.go:552-571` (Unsubscribe) | `lifted-as-is` | none — engine handles the lease + profile write directly | yes |
| 4 | ForceFullResync (cursor-truncation recovery + admin RPC) | `google_keep/sync/iface.go:51-71` + ForceFullResync impl in connector.go | `google_tasks/sync/lifecycle.go:370-411` (ForceFullResync); rebuildIDMapByTextMatch in connector.go:423-491 | `lifted-as-is` | `RebuildAdapterState(ctx, binding) (AdapterState, error)` (Tasks: text-match seed; Keep: re-pull full server tree) | yes |
| 5 | Pause/Resume with auth-failure + user-Disconnect transitions | `google_keep/sync/connector.go:270-272` (Disconnect; preserves bindings as paused, reconnect resumes) | `google_tasks/sync/lifecycle.go:603-642` (Resume w/ 7d horizon → ForceFullResync); transitionToPaused in connector.go:1252-1262 | `lifted-with-uplift: TASKS-WINS` (Keep gets explicit PausedReason field + Resume that calls RebuildAdapterState past horizon) | engine owns the state machine; adapter provides `IsAuthFailure(err) bool` for the auth-failed transition trigger | yes |
| 6 | Precondition recovery (3-branch: remote-deleted / remote-unchanged-repatch / remote-authoritative-apply) | Keep's stage3 HTTP 500 "Unknown Error" on missing/stale `baseVersion` (`google_keep/sync/subscriptions.go:50,145-147`) — currently NO 3-branch recovery | `google_tasks/sync/connector.go:1066-1159` (recoverFromPrecondition) | `lifted-with-uplift: TASKS-WINS` (Keep gains the 3-branch recovery; stage3-500-on-baseVersion maps to ErrorClass = PreconditionFailed) | `ClassifyError(err) ErrorClass`; `ReadRemoteByRef(ctx, binding, ref) (RemoteItem, error)` for the post-412 pull | yes |
| 7 | Dead-letter retry (per-item PushFailureCount + NextAttemptAt + deadLetterThreshold=10) | `google_keep/sync/subscriptions.go:113-148` (PushFailureCount, NextAttemptAt, BaseVersion); dead_letter_test.go | `google_tasks/sync/` — no analogue | `lifted-with-uplift: KEEP-WINS` (Tasks gains dead-letter; engine tracks PushFailureCount/NextAttemptAt in the engine-owned per-item state alongside LastSyncedSeq) | none — dead-letter is engine-internal bookkeeping driven by adapter `Insert/Patch/Delete` errors that ClassifyError marks as `Retryable` vs `Fatal` | yes |
| 8 | 30s scheduler tick registration | `google_keep/sync/cron.go:115-174` (KeepCronTickJob); `RegisterActiveSubscriptions` (cron.go:55) | `google_tasks/sync/lister.go` (TasksCronTickJob) + analogous registration | `lifted-as-is` | none — engine's scheduler_tick fans out to all bindings across all adapters via the existing `Connector` dispatch | no |
| 9 | Sync debouncer (1.5s window, OnChecklistMutated wiring, post-success choke) | `google_keep/sync/sync_debouncer.go` (1.5s default) | `google_tasks/sync/sync_debouncer.go` (1.5s window + 5s post-success choke per `connector.go:37`) | `lifted-with-uplift: TASKS-WINS` (Keep gains the post-success choke; debouncer is engine-owned) | none — engine subscribes to checklistmutator's notify and dispatches to the right binding's adapter; `OnWikiMutated` is implicit via the engine's mutator subscription | yes |
| 10 | Binding store (per-profile mutex + TOML serialization + Add/Remove/Find/LoadState/UpdateBinding) | `google_keep/sync/subscriptions.go` (SubscriptionStore) | `google_tasks/sync/store.go` (SubscriptionStore) | `lifted-as-is` (engine owns BindingStore generically; AdapterState is the per-adapter opaque subtree) | `EncodeAdapterState(s AdapterState) (toml []byte, error)` and `DecodeAdapterState(toml []byte) (AdapterState, error)` (the only adapter touchpoint; engine handles the rest) | no |
| 11 | Title sync (FetchRemoteListTitle) | `google_keep/sync/connector.go:1503-...` (cached lookup over latest pull's LIST nodes) | `google_tasks/sync/lifecycle.go:376-401` (tasklists.list match by ID) | `lifted-as-is` (already in BackendAdapter) | `FetchRemoteListTitle(ctx, profileID, remoteHandle) (title, ok, err)` — already in adapter.go | yes |
| 12 | Subtask handling (refuse-to-bind + tolerant flatten on inbound) | — (Keep has no subtask concept; flat lists only) | `google_tasks/sync/lifecycle.go:520-554` (refuse-to-bind via ErrTasksListHasSubtasks); `google_tasks/sync/connector.go:552-555` (tolerant flatten on inbound) | `stays-per-adapter` (true capability difference; Keep doesn't have parent-child) | `SupportsSubtasks() bool` capability bit; `FlattenSubtasks(items []RemoteItem) []RemoteItem` (no-op for Keep) | no |
| 13 | UID-marker insertion (Tasks: `wiki:uid` in Notes; Keep: server-side stable IDs) | Keep uses `ItemMapping{ServerID, ClientID}` keyed by wiki uid; no marker (`google_keep/sync/subscriptions.go:113-148`) | `google_tasks/sync/connector.go:464-481` (StripWikiUIDMarker); `google_tasks/translator/marker.go` | `stays-per-adapter` (translator-internal; engine doesn't see markers) | none — `RemoteToWiki` returns the resolved `WikiItem.UID`; `WikiToRemote` produces the marker-bearing notes | no |
| 14 | Causal divergence classification (op-log scan) | not yet wired (Keep currently uses fingerprint baseline; this PR completes the migration) | `google_tasks/sync/connector.go:285-298` calls `engine.Classify` | `lifted-as-is` (already extracted to `engine/classify.go`) | none — engine reads the op-log via ChecklistReader, not via adapter | yes |
| 15 | Inbound apply with suppressor wiring (Suppress/Unsuppress around mutator calls) | `google_keep/sync/connector.go` applyInboundFromKeep (≈787–810) | `google_tasks/sync/connector.go:577-580` (Suppress/Unsuppress around applyInboundFromTasks) | `lifted-as-is` | none — engine wraps the apply pass in Suppress/Unsuppress; adapter only provides the items via `PullRemote` | yes |
| 16 | Cursor advance (boundary semantics differ; both opaque to engine) | Keep: server-issued token (KeepCursor = `pull.ToVersion`); no safety buffer | Tasks: `max(updated) - 1s` safety buffer (`google_tasks/sync/connector.go:59`); apply-then-advance | `lifted-as-is` (engine call site is uniform; body is per-adapter) | `AdvanceCursor(binding Binding, result RemotePullResult) Binding` (returns updated binding; engine persists) | yes |
| 17 | Self-event emission to op-log (AppendSyncEvent after outbound success) | not yet wired (Keep gains this in Phase 5 as part of strictest-behavior-wins) | `google_tasks/sync/connector.go:926,948,959,995` (emitSyncEvent → checklistW.AppendSyncEvent) | `lifted-with-uplift: TASKS-WINS` (Keep adopts; engine emits self-events after every successful Insert/Patch/Delete) | none — engine calls `ChecklistMutator.AppendSyncEvent` directly after each adapter primitive returns success | yes |
| 18 | Auth credential lifecycle (token refresh; user-revocation transition) | gpsoauth master-token refresh in gateway | OAuth refresh token + `invalid_grant` retry-once in `google_tasks/gateway/oauth.go` | `lifted-as-is` (auth refresh is wholly inside the gateway; engine doesn't observe credential mechanics) | adapter primitives bubble auth errors via `ClassifyError` → `ErrorClass.AuthFailed` which the engine consumes | no |
| 19 | List candidate remote collections (for the bind UI) | `google_keep/sync/connector.go` ListNotes called from `internal/grpc/api/v1/connector_service.go:391` | Tasks's `tasklists.list` called from gRPC handler | `lifted-as-is` | `ListRemoteCollections(ctx, profileID) ([]RemoteCollection, error)` where `RemoteCollection = {Handle, Title, Capabilities}` | no |

## Surprising findings

1. **Keep's stage3-500 IS its 412.** Keep returns HTTP 500 ("Unknown Error" body) when an outbound LIST_ITEM update is missing or carries a stale `baseVersion`. This is Keep's optimistic-concurrency-control failure (`google_keep/sync/subscriptions.go:50,145-147`). Tasks returns standard 412 Precondition Failed. They are **the same conceptual event** with different wire-encodings; the engine's `ClassifyError` returns `PreconditionFailed` for both, and the same 3-branch recovery (remote-deleted / remote-unchanged-repatch / remote-authoritative-apply) runs identically. Keep adopting this in Phase 5 is a **net-positive correctness improvement** — Keep currently does not have the 3-branch recovery, so a stage3-500 on a stale baseVersion can clobber user edits in the same way Tasks's pre-recovery code did. Smoke test priority: replicate a stage3-500 on Keep after the engine lands.

2. **Keep's pause is via `Disconnect`, but it preserves bindings.** Connector.go:270-272: *"Disconnect wipes the master token from the calling user's profile but preserves the bindings list (paused). Reconnect resumes them."* This is structurally identical to Tasks's `transitionToPaused` (PausedReason="auth_failed"). The engine should unify this as: "auth lost → bindings transition to paused with PausedReason=auth_failed; reconnect → all bindings transition active; the Tasks 7-day horizon → engine-owned `ResumeAfterHorizon` rule that calls `RebuildAdapterState` if pause duration ≥ 7d." Keep gets the explicit PausedReason field (currently it's implicit via "is master token empty?"); Tasks gets the user-initiated Disconnect path (currently it only pauses on auth-failed errors, not on user request).

3. **Dead-letter retry is Keep-only and well-developed.** Keep tracks `PushFailureCount`, `NextAttemptAt`, and skips items whose `NextAttemptAt` is in the future. After `deadLetterThreshold=10` consecutive failures, the item is dead-lettered and the macro renders a UI badge. Tasks has no analogue — every tick re-attempts. **Strictest-behavior-wins:** the engine adopts dead-letter; Tasks gains it for free. Tasks-side smoke test: trigger a fatal-but-not-auth error (e.g., 400 Bad Request from a translator bug) and verify the item stops being retried after threshold.

4. **Both connectors persist `BaseVersion` / `ItemEtags` on the binding for optimistic concurrency, with different shapes.** Keep: `BaseVersion` (string) per item, plus `ClientID` distinct from `ServerID`, surviving incremental pulls that don't echo the node. Tasks: `ItemEtags` map (task-id-keyed). Both are adapter-internal — they go into the opaque `AdapterState` subtree on the binding, encoded/decoded by the adapter's `EncodeAdapterState`/`DecodeAdapterState`. Engine never inspects them.

5. **Cursor mechanisms are fundamentally different and rightly stay opaque to the engine.** Keep: server token (`pull.ToVersion`), no safety buffer, boundary-precise. Tasks: timestamp with `-1s` safety buffer (`updatedMinSafetyBufferSeconds`), reprocesses the boundary item every tick (idempotent under the engine's same-etag-skip and divergence-skip guards). The engine calls `AdvanceCursor(binding, result) Binding` uniformly; the adapter implementation chooses how. The engine does NOT try to unify the cursor data type.

6. **Keep does not yet emit self-events to the op-log; Phase 5 wires it.** Tasks emits via `emitSyncEvent → checklistW.AppendSyncEvent` after every successful Insert/Patch/Delete. Keep currently does not. Until Keep emits self-events, `engine.Classify` cannot correctly distinguish "Keep's own apply" from "user edit" for Keep bindings — Keep is operating on the value-fingerprint fallback. Phase 5 fixes this; until then, Keep bindings keep their existing fingerprint baseline as a safety net (which is removed at the end of Phase 5).

7. **Subtask handling is asymmetric and stays that way.** Tasks's parent-child task hierarchy is a backend feature; Keep notes don't have one. The capability bit `SupportsSubtasks() bool` is the right primitive — engine queries it during Bind to decide whether to refuse on hierarchy detection. Tasks's tolerant-flatten on inbound (when subtasks appear post-bind) is an inbound-apply detail wholly inside `RemoteToWiki`; engine doesn't see it.

8. **`ListRemoteCollections` was missing from the original ADR-0015 sketch.** Both connectors expose a way to enumerate the user's available remote lists (Keep: `ListNotes`; Tasks: `tasklists.list`) so the bind UI can offer a picker. Original sketch didn't include this. Adding to the BackendAdapter for Phase 1.

9. **Validation primitive `ValidateRemoteBinding` was also missing.** Tasks's bind ceremony refuses to bind to a list that already contains subtasks (`ErrTasksListHasSubtasks`). This is a per-adapter pre-condition check the engine runs during `Bind` before writing the binding to the profile. Adding `ValidateRemoteBinding(ctx, remoteHandle) error` to the contract.

10. **Keep persists a `LabelIDs` map per binding (label name → MainID)** to avoid re-stamping label CRUD on every tick. Adapter-internal; lives in AdapterState. No engine touchpoint.

## Phase 1 BackendAdapter contract (proposed)

This is what `internal/connectors/adapter.go` becomes. Every method has a docstring derived from the audit row(s) above.

```go
// BackendAdapter is the contract every connector's per-tick implementation
// MUST honor. The SyncEngine calls these primitives; the adapter provides
// only wire-protocol verbs, translation, capability bits, and error
// classification.
type BackendAdapter interface {
    // Identity.
    Kind() ConnectorKind

    // Pull / push primitives (per-tick reconcile, row 1).
    PullRemote(ctx context.Context, binding Binding) (RemotePullResult, error)
    InsertRemote(ctx context.Context, binding Binding, item WikiItem) (RemoteRef, error)
    PatchRemote(ctx context.Context, binding Binding, ref RemoteRef, item WikiItem) (RemoteRef, error)
    DeleteRemote(ctx context.Context, binding Binding, ref RemoteRef) error

    // Translate (rows 1, 13).
    RemoteToWiki(remote RemoteItem) (WikiItem, error)
    WikiToRemote(wiki WikiItem) (RemoteItem, error)

    // Cursor advance (row 16; opaque body).
    AdvanceCursor(binding Binding, result RemotePullResult) Binding

    // Bind ceremony seed + validate (row 2).
    SeedBindingState(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (AdapterState, error)
    ValidateRemoteBinding(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) error

    // Force full resync rebuild (row 4).
    RebuildAdapterState(ctx context.Context, binding Binding) (AdapterState, error)

    // List candidate collections for the bind UI (row 19).
    ListRemoteCollections(ctx context.Context, profileID wikipage.PageIdentifier) ([]RemoteCollection, error)

    // Title sync (row 11; existing).
    FetchRemoteListTitle(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (title string, ok bool, err error)

    // Adapter-state codec (row 10; opaque payload, structured edges).
    EncodeAdapterState(s AdapterState) (map[string]any, error)
    DecodeAdapterState(raw map[string]any) (AdapterState, error)

    // Capability bits (row 12).
    SupportsSubtasks() bool

    // Read remote item by ref (row 6; for the post-412 pull).
    ReadRemoteByRef(ctx context.Context, binding Binding, ref RemoteRef) (RemoteItem, error)

    // Error classification (rows 5, 6, 7).
    ClassifyError(err error) ErrorClass
}

// ErrorClass is the engine's vocabulary for what the adapter saw.
// The engine routes recovery by class; adapters translate vendor errors.
type ErrorClass int
const (
    ErrorClassNone ErrorClass = iota
    ErrorClassTransient        // retry next tick
    ErrorClassRetryable        // increment PushFailureCount; eventually dead-letter
    ErrorClassFatal            // dead-letter immediately
    ErrorClassAuthFailed       // pause binding, surface reconnect CTA
    ErrorClassPreconditionFailed // run 3-branch precondition recovery
    ErrorClassRateLimited      // back off the 30s tick for this binding
    ErrorClassNotFound         // remote item is gone; mirror delete to wiki
)

// RemotePullResult is the output of PullRemote, normalized across adapters.
type RemotePullResult struct {
    Items     []RemoteItem // every remote item the adapter saw this tick
    NewCursor any          // opaque per-adapter (Tasks: time.Time; Keep: string)
    Truncated bool         // adapter is asking for ForceFullResync
}

// RemoteRef is an opaque handle to a remote item.
type RemoteRef string

// RemoteItem is the normalized shape pre-translation.
type RemoteItem struct {
    Ref      RemoteRef
    Etag     string             // "" if backend has no per-item etag
    Title    string
    Notes    string
    Status   string             // backend-specific; translator normalizes
    Due      time.Time
    Updated  time.Time
    Deleted  bool
    Position string             // backend-specific ordering
    Vendor   map[string]any     // adapter-internal extra fields
}

// WikiItem is the normalized post-translation shape (mirrors apiv1.ChecklistItem).
type WikiItem struct {
    UID         string
    Text        string
    Checked     bool
    Tags        []string
    Description string
    Due         time.Time
    SortOrder   int64
}

// RemoteCollection is a candidate remote list for the bind UI.
type RemoteCollection struct {
    Handle       string // adapter-opaque (Tasks: tasklist ID; Keep: note ID)
    Title        string
    Capabilities CollectionCapabilities
}

type CollectionCapabilities struct {
    HasSubtasks bool // Tasks: list has parent-child; Keep: always false
}

// AdapterState is the per-adapter opaque blob persisted on each binding.
// The engine treats it as a sealed envelope: passes it back to the adapter
// on every primitive call; never inspects fields.
type AdapterState map[string]any
```

## Out-of-scope for this matrix

- Profile-level state (refresh tokens, master tokens, app-specific passwords) stays adapter-managed and lives outside the binding entry. The engine's `LoadBinding` API takes the profile ID; the adapter reads its own credential bundle via the existing per-connector store API. This is a Phase 4/5 detail.
- The gRPC `ConnectorService` API surface is touched in Phase 6 (Bind/Unbind rename). The audit doesn't list every RPC; the rename sweep is mechanical.
