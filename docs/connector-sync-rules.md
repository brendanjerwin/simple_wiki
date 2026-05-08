# Connector Sync Rules — Authoritative Reference

This is the single-source-of-truth for how the SyncEngine reconciles a
wiki checklist with a remote backend (Google Tasks, Google Keep, future
iCloud Reminders). All rules are adapter-agnostic unless explicitly
called out.

## Core architecture

- **Engine** (`internal/connectors/engine/`): owns reconcile, bind,
  unbind, force-resync, pause/resume, precondition recovery, dead-letter
  retry, scheduler tick, sync debouncer, binding store. Adapter-agnostic.
- **BackendAdapter** (`internal/connectors/adapter.go`): per-vendor
  primitives. Each adapter implements `PullRemote`, `InsertRemote`,
  `PatchRemote`, `DeleteRemote`, `RemoteToWiki`, `WikiToRemote`,
  `AdvanceCursor`, `RefreshItemBaseline`, `SyncCollectionState`,
  `RebuildAdapterState`, `ReadRemoteByRef`, `ClassifyError`,
  `SeedBindingState`, `ValidateRemoteBinding`,
  `ListRemoteCollections`, `EncodeAdapterState`,
  `DecodeAdapterState`, `Kind`, `SupportsSubtasks`,
  `FetchRemoteListTitle`.
- **Op-log** (`wiki.checklists.<list>.events[]`): every checklist
  mutation (user click, agent edit, connector apply) appends an event
  with `(seq, src, op, uid, ts, …)`. The op-log is the wiki's authority
  for divergence — never field-fingerprint comparison.
- **Causal divergence** (ADR-0015): "wiki diverged" means there's an
  event with `seq > LastSyncedSeq` whose `src` is `user:*`,
  `connector:OTHER:*`, or unknown. Self-writes
  (`connector:<this-kind>:*`) are not divergent.

## The reconcile loop

For each binding, every 30s (or sooner via debouncer), the engine runs
this exact sequence:

1. **Wait for lease-table ready.** Lease table is the cross-binding
   exclusivity registry built at boot.
2. **Find binding** in store. Missing → no-op (boot race). Paused →
   no-op (steady-state). Within `rateLimitChoke` (5s) of last success
   → no-op.
3. **Classify divergence.** Walk op-log events with `seq > cursor`.
   Per-uid: `WikiDiverged = true` iff at least one such event has a
   non-self source. `LatestEventSource` is the `src` of the highest-seq
   event for that uid.
4. **PullRemote.** Adapter fetches all items from the remote since the
   adapter-internal cursor (Tasks: `last_updated_min`; Keep:
   `keep_cursor`).
   - Each `RemoteItem` carries `RemoteDiverged = true` if its
     server-side concurrency token (etag/baseversion) differs from our
     stored value (per-item, in `adapter_state`).
   - Truncated response → engine triggers ForceFullResync on the next
     tick.
5. **applyInbound** (per item; in order):
   - **For non-deleted items: refresh per-item baseline.** Adapter's
     `RefreshItemBaseline` writes the just-pulled etag/baseversion into
     `adapter_state` BEFORE any 4-cell decision. This way the next
     PatchRemote uses the freshest baseline; no 412 loops on stale
     etags. (Production fix 2026-05-06.)
   - **Apply the 4-cell merge** per uid:
     - Deleted=true → mirror delete to wiki via `DeleteItemForSync`,
       remove from `idMap`. (Done.)
     - WikiDiverged=false ∧ RemoteDiverged=false → no-op.
     - WikiDiverged=false ∧ RemoteDiverged=true → apply remote to wiki
       via `UpdateItemForSync` (or `AddItemForSync` if uid not in
       `idMap`).
     - **WikiDiverged=true ∧ RemoteDiverged=false → push-wiki path.**
       Skip apply; pushOutbound below carries the wiki edit.
     - **WikiDiverged=true ∧ RemoteDiverged=true → user-wins refinement
       (ADR-0015 + 2026-05-06 fix).** If `LatestEventSource` starts with
       `user:`, skip apply — preserve user intent. Otherwise (the wiki
       diverged from a cross-connector apply, not a user click) apply
       remote per ADR-0015's `conflict-remote-wins` rule. The user-source
       short-circuit prevents stale-etag 412s from reverting just-
       clicked check-offs (production regression 2026-05-06).
6. **pushOutbound** (per uid present in wiki AND not skipped):
   - **Inserts** (uid NOT in `idMap`): always push.
     - On `ErrorClassPreconditionFailed`: trigger Insert-recovery via
       `RebuildAdapterState`, persist rebuilt state, exit loop. The
       next tick has the correct uid → ref mapping (from the rebuild)
       and routes through Patch with a fresh baseline.
     - On `Retryable`: bump `push_failures.<uid>.count`; backoff per
       exponential schedule; dead-letter at `count >= 10`.
     - On `AuthFailed`: pause binding; abort tick.
     - Other errors: abort tick.
   - **Patches** (uid IN `idMap` AND `WikiDiverged=true`):
     - Gate: skip if not WikiDiverged (avoid churn).
     - Skip if dead-lettered or in backoff.
     - On 412 / `PreconditionFailed`: enter
       `runPreconditionRecovery`:
       - **Branch A (deleted):** `ReadRemoteByRef` returns
         `Deleted=true` (or NotFound) → mirror delete to wiki + remove
         from `idMap`.
       - **Branch B/C (wiki-wins).** Otherwise:
         - Refresh per-item baseline from the just-read remote (so
           the repatch sends the fresh concurrency token).
         - Re-PATCH with the wiki's intent. **Always.** No more
           remote-wins clobbering. (Production regression 2026-05-06:
           legacy 3-branch behavior reverted user check-offs whenever
           remote-now ≠ wiki-now, even when the divergence was just a
           stale etag bump.)
         - On success: `AppendSyncEvent("outbound_patched")` (so
           cursor can advance past user events) + log
           `precondition_recovery_wiki_wins_repatch`.
   - **Deletes** (uid in `idMap` AND not in current wiki items): always
     push DeleteRemote.
     - Keep specifics: send the LIST_ITEM with `Timestamps.Deleted =
       now` (NOT Trashed — Keep's Changes API only honors `deleted`
       on incremental updates). Production regression 2026-05-07.
   - On every successful primitive: `recordPushSuccess` (clears
     `push_failures` for that uid) + `AppendSyncEvent` with op =
     `outbound_inserted` / `outbound_patched` / `outbound_deleted`.
7. **SyncCollectionState.** Once per tick after the per-item loop.
   Adapter-specific:
   - Keep: walk current wiki items' tags; mint Keep labels for any
     unmapped name (`MergeKeepLabels`); push label CRUD + LIST node
     update with merged labelIDs in a single Changes request. No-op
     when all tags already map. Restored from legacy keepsync
     2026-05-07.
   - Tasks: no-op.
8. **AdvanceCursor.** Adapter writes its per-tick cursor into
   `adapter_state` (Tasks: `max(updated) - 1s`; Keep: server token).
9. **advanceLastSyncedSeq.** Engine sets `binding.LastSyncedSeq = max
   self-event seq` from the op-log. Per ADR-0015: cursor advances ONLY
   past our own writes (events with src starting `connector:<this-
   kind>:`). User and cross-connector events stay visible to next
   tick's classify until a covering self-write lands.
10. **Save binding.**

## Per-binding state shape

```toml
[wiki.connectors.<kind>.bindings[N]]
page = "shopping_lists"
list_name = "Grocery"
remote_handle = "<vendor-specific list ID>"  # Keep: ServerID; Tasks: tasklist ID
remote_list_title = "Groceries"
state = "active" | "paused"
paused_reason = ""  # "auth_failed" | "" (active)
paused_at = "2026-…"
bound_at = "2026-…"
last_synced_seq = 64  # cursor — see advanceLastSyncedSeq
last_successful_sync_at = "2026-…"

[wiki.connectors.<kind>.bindings[N].adapter_state]
# Common: dead-letter bookkeeping
push_failures.<uid>.count = 3
push_failures.<uid>.next_attempt_at = "2026-…"

# Tasks-specific
item_id_map.<wiki-uid> = "<task-id>"
item_etags.<task-id> = "\"<etag>\""
last_updated_min = "2026-…"  # cursor

# Keep-specific
item_id_map.<wiki-uid> = "<server-id>"  # engine's flat shape (used for insert-vs-patch routing)
item_mapping.<server-id> = { server_id, base_version, client_id }  # adapter's structured shape (used for Patch baseline)
keep_cursor = "<server token>"
keep_note_client_id = "<list node client id>"  # required for label CRUD pushes
label_ids.<wiki-tag> = "<keep label main id>"
```

## Op-log event sources

| `src` prefix | Meaning | Effect on `WikiDiverged` for `<this-kind>` |
|---|---|---|
| `user:<email>` | Operator clicked in wiki UI | Diverged |
| `connector:google_tasks:apply` | Tasks engine wrote to wiki (inbound apply) | Diverged when this-kind ≠ google_tasks |
| `connector:google_keep:apply` | Keep engine wrote to wiki (inbound apply) | Diverged when this-kind ≠ google_keep |
| `connector:<this-kind>:outbound_push` | This engine pushed to remote (self-write event) | Not diverged (cursor advances past it) |
| `connector:<this-kind>:push_recovery` | This engine's recovery wrote to wiki | Not diverged |
| `connector:<this-kind>:apply` | This engine's inbound apply | Not diverged |
| `migration:initial_baseline` | Migration job seeded baseline | Conservative: divergent (treats as user) |
| `system:*` | Future system writes | Conservative: divergent |

## Op-log event ops (informational; engine doesn't switch on these)

`baseline`, `add`, `toggle`, `set_text`, `set_tags`, `delete`,
`bulk_update`, `outbound_inserted`, `outbound_patched`,
`outbound_deleted`.

## Concurrency-token semantics per backend

- **Google Tasks**: per-item etag (RFC 7232). `If-Match: <etag>` on
  PATCH. 412 = stale etag. Etag bumps on EVERY server-side update,
  including non-content metadata (position changes, server denorm).
  Treating "etag differs" as "remote content changed" is wrong —
  hence the user-wins refinement above.
- **Google Keep**: per-item `BaseVersion`. Sent on outbound LIST_ITEM
  updates. Stage3 HTTP 500 "Unknown Error" = stale baseVersion.
  Gateway wraps stage3-500 as `ErrProtocolDrift`; adapter classifies
  as `PreconditionFailed`; engine routes to recovery.

## Bind ceremony

1. Resolve credentials for the profile.
2. Acquire per-checklist lease (cross-binding exclusivity).
3. Validate remote (`ValidateRemoteBinding`): for Tasks, reject lists
   with subtasks; for Keep, reject non-LIST notes and trashed/deleted
   notes.
4. `SeedBindingState`: full pull of remote items, populate adapter
   state. For Keep: capture LIST node's `client_id` from the pull.
5. Insert `Binding` into store with state=active, `LastSyncedSeq` =
   max(seq) on the bound checklist at bind time (treats current wiki
   as the synced baseline).
6. Release lease on success; on any error roll back.

## Unbind ceremony

1. Acquire per-checklist lease.
2. Remove binding from store.
3. Release lease.

## Pause / Resume / auth-failed transitions

- `AuthFailed` from any adapter primitive → `applyPausedTransition`
  with `PausedReason = "auth_failed"`. Steady-state, not an error.
- `Resume` (operator-triggered or reconnect): if pause duration ≥
  7d, run `runForceFullResync` (cursor possibly invalid); otherwise
  unpause and resume normal ticks.

## Migrations from the legacy connector

- `subscriptions[]` → `bindings[]` (Phase 7, idempotent eager).
- Legacy adapter state translated:
  - Tasks: `synced_items` dropped (causal divergence replaces
    fingerprint comparison).
  - Keep: `item_id_map[uid] = ItemMapping{…}` → flat
    `item_id_map[uid] = serverID` (engine) AND structured
    `item_mapping[serverID] = {server_id, base_version, client_id}`
    (adapter). Legacy fingerprint baselines (`synced_text/checked/
    sort_value`) dropped.
  - Keep: legacy field aliases translated:
    `keep_note_id → remote_handle`,
    `keep_note_title → remote_list_title`.

## Self-heal hooks

- **`keep_note_client_id` self-heal** (`PullRemote`): when stored
  client_id is empty AND `remote_handle` is non-empty AND the LIST
  node is in the pull → capture from the LIST node. If LIST node not
  in pull → set `Truncated=true` (engine triggers `ForceFullResync`
  → full pulls always include LIST). Bails out when `remote_handle` is
  empty (re-bind is the only recovery for that case).

## Tombstone handling on PullRemote

- Items with `Trashed` OR `Deleted` timestamp set surface as
  `RemoteItem{Deleted: true}`.
- Filter: include items whose `ParentID` or `ParentServerID` matches
  `binding.RemoteHandle` OR (for tombstones) whose `ServerID` is in
  our `item_id_map`. Keep clears parent linkage on user-deleted-in-app
  tombstones; the item_id_map lookup catches them.

## Strict invariants

- **Wiki check-offs are durable.** Once a `user:*` event for a uid
  appears with `seq > LastSyncedSeq`, the engine MUST NOT revert that
  uid's state to a prior value via any inbound apply or recovery
  branch. The wiki-wins refinements above enforce this.
- **Cursor advances only past self-writes.** User and cross-connector
  events stay visible until covered by a self-event (typically the
  engine's own `outbound_*` event). This guarantees never-lost user
  intent across ticks.
- **Self-events are tagged via `WithSource(ctx, ConnectorSource(kind,
  op))`.** Three sources today: `apply` (inbound), `outbound_push`
  (per-item primitive), `push_recovery` (recovery branches A and B).
- **Suppressor wraps every wiki write that mirrors a remote read.**
  Prevents the engine's apply from reflecting back through the wiki's
  notify-and-tick path as a user-source event.
- **Per-binding mutations are serialized.** The `LeaseTable`'s
  per-checklist lock ensures only one tick (or one bind/unbind) runs
  at a time per `(profile, page, list_name)`.

## What is NOT a sync rule

- **Field fingerprinting.** ADR-0015 explicitly removed this. The
  engine never compares Title/Notes/Status across syncs to decide
  divergence — the op-log is the authority.
- **Bidirectional merge of concurrent edits.** "Concurrent edit"
  means: user clicks in wiki AND user clicks in remote app within the
  same tick window. Engine resolves by `LatestEventSource`: if the
  most recent wiki event is `user:*`, wiki wins. If it's
  `connector:OTHER:apply` (cross-connector), remote wins. Real
  conflicts (both sides wrote different values) require the user to
  re-do one side; the engine doesn't merge.
- **Position / sort order propagation.** Best-effort. Tasks: `position`
  round-trips. Keep: `sort_value` round-trips. Neither is
  divergence-relevant.
