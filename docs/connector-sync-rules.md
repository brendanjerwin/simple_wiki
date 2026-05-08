# Connector Sync Rules — Authoritative Reference (v2)

This document specifies how the SyncEngine reconciles a wiki checklist
with a remote backend (Google Tasks, Google Keep, future iCloud
Reminders). It is the contract — implementation must match. If
implementation diverges, the contract wins until the contract is
revised.

> **Revision note (v2, 2026-05-07).** A panel of distributed-systems
> reviewers (Kleppmann, Helland, Shapiro, Bailis, Lamport) flagged
> that v1 conflated "wiki-local total order" with "happens-before"
> and asserted invariants the rules don't actually enforce. v2 fixes
> the terminology, names the actual safety properties, adds explicit
> non-guarantees, and adds operational sections (forensic state
> catalog, primitive contracts, dead-letter lifecycle, mid-tick
> crash recovery, lease lifecycle, memories vs. guesses).

---

## 0. Glossary

- **Replica**: a process or system that holds a copy of an item's
  state. The replicas are: `wiki` (this server), `tasks` (Google's
  servers), `keep` (Google's servers).
- **Source**: the writer of an event in the wiki op-log. Sources are:
  `user:<email>`, `connector:<kind>:<op>`, `migration:<name>`,
  `system:<name>`. The wiki node is the only writer of `seq`; sources
  are *labels* on wiki-local events, not separate clocks.
- **`seq`**: a per-list monotonically-increasing integer assigned by
  the wiki at op-log append time, under a per-list mutex. It is a
  **wiki-local total order**, not a logical clock and not a witness
  of happens-before across replicas.
- **Self-event**: an op-log event whose `src` starts with
  `connector:<this-engine's-kind>:`. Defined per-engine, not
  per-replica (see §11.4 for the multi-replica caveat).
- **Cursor (`last_synced_seq`)**: per-binding scalar. The high-water
  mark of self-events this engine has emitted, in the wiki's total
  order. NOT a vector clock. NOT happens-before across replicas.
- **Memory**: a value the engine durably owns. The op-log is a memory.
  `LastSyncedSeq` is a memory. Source labels are memories.
- **Guess**: the engine's prediction about an external system's state.
  `item_etags`, `item_mapping[…].base_version`, `idMap`,
  `keep_note_client_id`, `keep_cursor`, `last_updated_min`,
  `push_failures.<uid>.next_attempt_at` are guesses. Guesses must
  have explicit refresh + apology paths (§9).

---

## 1. Architecture

- **Engine** (`internal/connectors/engine/`): owns reconcile, bind,
  unbind, force-resync, pause/resume, precondition recovery,
  dead-letter retry, scheduler tick, sync debouncer, binding store.
  Adapter-agnostic. **Single-process.** Multi-replica safety is not
  guaranteed (§11.4).
- **BackendAdapter** (`internal/connectors/adapter.go`): per-vendor
  primitives. Each adapter implements the per-primitive contracts
  declared in §6.
- **Op-log** (`wiki.checklists.<list>.events[]`): every checklist
  mutation appends an event with `(seq, src, op, uid, ts, …)`. The
  op-log is the wiki's authority for **observed-divergence
  detection** (§3). It is NOT a happens-before record.

---

## 2. The reconcile loop

For each binding, every 30s (or sooner via debouncer), the engine
runs this exact sequence. Each step has its own crash-recovery
discipline (§7).

1. **Wait for lease-table ready.** The lease table is the
   per-checklist exclusivity registry built at boot.
2. **Find binding** in store. Missing → no-op (boot race). Paused →
   no-op (steady-state). Within `rateLimitChoke` (5s) of last success
   → no-op. (The 5s is documented but not justified beyond "post-
   success choke against tight loops"; see §11.5.)
3. **Classify observed divergence.** Walk op-log events with `seq >
   cursor`. Per-uid: `WikiDiverged = true` iff at least one such
   event has a non-self source (i.e., is `user:*`, a different
   connector's source, `migration:*`, `system:*`, or unknown).
   `LatestEventSource` is the `src` of the highest-seq event for
   that uid in the wiki's append order. `UncoveredUserEvent`
   (introduced v2) is `true` iff *any* event for that uid in the
   uncovered window has `src` starting `user:` (sticky user
   precedence; see §4).
4. **PullRemote.** Adapter fetches all items from the remote since
   the adapter-internal cursor (Tasks: `last_updated_min`; Keep:
   `keep_cursor`).
   - Each `RemoteItem` carries `RemoteDiverged = true` if its
     server-side concurrency token (etag/baseversion) differs from
     our stored value (per-item, in `adapter_state`). NOTE: this is
     a *change-detection* signal, not a content-difference signal;
     the remote may have bumped the token without changing content
     (Tasks etag bumps for non-content reasons).
   - Truncated response → engine triggers ForceFullResync on the
     next tick (rate-limited per §11.6).
5. **applyInbound** (per item; in order):
   - **For non-deleted items: refresh per-item baseline.** Adapter's
     `RefreshItemBaseline` writes the just-pulled etag/baseversion
     into `adapter_state` BEFORE the 4-cell decision. **Trade-off
     disclosed (v2):** This eager refresh disarms the remote's
     optimistic-concurrency-control protection within the same tick,
     so the subsequent push will succeed against any concurrent
     third-party edit. We accept this for steady-state (avoids 412
     loops); §10 names the lost-update non-guarantee that follows.
     The baseline is persisted to disk via `SaveBinding` at end-of-
     tick (step 10), not before the network call. On crash between
     pull and save, the next tick re-pulls and re-refreshes — safe.
   - **Apply the 4-cell merge** per uid:
     - **Deleted=true ∧ UncoveredUserEvent=true** → sticky user-
       wins for the deleted cell (v3.1, Lamport §10.13). Skip the
       wiki delete; clear `idMap` so pushOutbound INSERTs a fresh
       remote ref carrying the wiki's user-edited state.
     - **Deleted=true ∧ UncoveredUserEvent=false** → mirror delete
       to wiki via `DeleteItemForSync`, remove from `idMap`. (The
       "if uid known to us" lookup uses `refToUID` built from
       `idMap`.)
     - **WikiDiverged=false ∧ RemoteDiverged=false** → no-op.
     - **WikiDiverged=false ∧ RemoteDiverged=true** → apply remote
       to wiki via `UpdateItemForSync` (or `AddItemForSync` if uid
       not in `idMap`).
     - **WikiDiverged=true ∧ RemoteDiverged=false** → push-wiki
       path. Skip apply; pushOutbound (§6) carries the wiki edit.
     - **WikiDiverged=true ∧ RemoteDiverged=true ∧
       UncoveredUserEvent=true** → **sticky user-wins**. Skip apply;
       pushOutbound carries the wiki edit. (v2 strengthening: this
       fires whenever ANY user event for the uid is in the
       uncovered window, not only when the LATEST event is
       user-sourced. Closes the "sandwich" hole where a cross-
       connector apply landed *after* a user click but *before*
       cursor advance.)
     - **WikiDiverged=true ∧ RemoteDiverged=true ∧
       UncoveredUserEvent=false** → cross-connector
       conflict-remote-wins. Apply remote to wiki via
       `UpdateItemForSync`. The wiki's recent change came from
       another connector's apply, so the remote's fresh state from
       *this* connector is the authoritative recent write from this
       side.
6. **pushOutbound** (per uid present in wiki AND not skipped):
   - **Inserts** (uid NOT in `idMap`): always push.
     - On `ErrorClassPreconditionFailed`: trigger Insert-recovery
       via `RebuildAdapterState`, persist rebuilt state, exit loop.
       The next tick has the correct uid → ref mapping (from the
       rebuild) and routes through Patch with a fresh baseline.
     - On `Retryable`: bump `push_failures.<uid>.count`; backoff
       per exponential schedule (§11.7); dead-letter at `count >=
       10` (§9.3 for lifecycle).
     - On `AuthFailed`: pause binding; abort tick.
     - Other errors: abort tick.
   - **Patches** (uid IN `idMap` AND `WikiDiverged=true`):
     - Gate: skip if not WikiDiverged.
     - Skip if dead-lettered or in backoff.
     - On 412 / `PreconditionFailed`: enter
       `runPreconditionRecovery`:
       - **Branch A (deleted):** `ReadRemoteByRef` returns
         `Deleted=true` (or NotFound) → mirror delete to wiki +
         remove from `idMap`.
       - **Branch B (wiki-wins).** Otherwise:
         - Refresh per-item baseline from the just-read remote.
         - Re-PATCH with the wiki's intent. **Always.** Documented
           non-guarantee: this clobbers any concurrent third-party
           edit committed between the pull and the read (§10).
         - On success: `AppendSyncEvent("outbound_patched")` (so
           cursor can advance past covered user events).
   - **Deletes** (uid in `idMap` AND not in current wiki items):
     always push DeleteRemote. Iterate over a snapshot of `idMap`
     taken at the start of step 6 (modifications by applyInbound's
     deleted-branch are visible as missing entries → no double-push).
     - Keep specifics: send the LIST_ITEM with `Timestamps.Deleted =
       now` (NOT Trashed — Keep's Changes API only honors `deleted`
       on incremental updates).
   - On every successful primitive: `recordPushSuccess` (clears
     `push_failures` for that uid) + `AppendSyncEvent` with op =
     `outbound_inserted` / `outbound_patched` / `outbound_deleted`.
7. **SyncCollectionState.** Once per tick after the per-item loop.
   Adapter-specific (§6):
   - Keep: walk current wiki items' tags; mint Keep labels for any
     unmapped name; push label CRUD + LIST node update with merged
     labelIDs in a single Changes request.
   - Tasks: no-op.
8. **AdvanceCursor.** Adapter writes its per-tick cursor into
   `adapter_state` (Tasks: `max(updated) - 1s`; Keep: server token).
9. **advanceLastSyncedSeq.** Engine sets `binding.LastSyncedSeq =
   max self-event seq` from the op-log (where self = src starts
   `connector:<this-kind>:`). Per ADR-0015: cursor advances ONLY
   past our own writes. User and cross-connector events stay visible
   to next tick's classify until a covering self-write lands.
10. **Save binding.** Single atomic write of the binding row to the
    profile page.

---

## 3. What "divergence" means (precise)

This section deliberately avoids the word "causal." `seq` is a
wiki-local total order, not a logical clock. The engine cannot
reason about happens-before across the wiki/remote boundary.

**`WikiDiverged`** for uid `u` at tick T is the boolean:
> *Does the wiki op-log contain an event `e` with `e.uid == u`,
> `e.seq > LastSyncedSeq`, and `e.src` not starting
> `connector:<this-kind>:`?*

This is **observed divergence**: a non-self event we haven't yet
covered with one of our own writes. It is NOT "happens-before."

**`RemoteDiverged`** for ref `r` at tick T is the boolean:
> *Does the just-pulled `RemoteItem.Etag` (Tasks) or
> `BaseVersion` (Keep) differ from the value we have stored in
> `adapter_state` for `r`?*

This is **change-detection at the remote**: the remote's
concurrency token has bumped since we last looked. It is NOT
"the remote's content differs from ours" — Tasks bumps etag for
non-content updates (position, server-side denorm). The 4-cell
merge is a Cartesian product of these two booleans, plus the
sticky-user-wins refinement.

**The 4-cell merge is not a CRDT join.** It is a policy expressed
as a decision procedure over `(WikiDiverged, RemoteDiverged,
UncoveredUserEvent)`. State convergence under exact replay
holds (because `RefreshItemBaseline` runs before the merge and
self-events aren't divergent), but commutativity across
arbitrary schedules does not (the policy is order-sensitive via
`UncoveredUserEvent` membership in the uncovered window). This
is the deliberate price of breaking symmetry to align with
operator intent.

---

## 4. The sticky user-wins refinement (v2)

> When `WikiDiverged=true ∧ RemoteDiverged=true ∧
> UncoveredUserEvent=true`, skip the inbound apply and let
> pushOutbound carry the wiki state.

Rationale: a user click expresses ground-truth operator intent
*at the wiki replica*. Any third-party concurrent edit is
non-comparable (we have no happens-before relation). Privileging
user intent at the wiki is a deliberate policy.

**Strict invariant (v2 strengthened):**
> *If at any point an op-log event `e` exists with `e.src` starting
> `user:`, `e.uid = u`, and `e.seq > LastSyncedSeq`, then the
> engine MUST NOT apply remote state to wiki for uid `u` until a
> self-event has covered `e`.*

Note this is **sticky**: the rule fires whenever ANY uncovered
user event exists for the uid, not only when the LATEST event is
user-sourced. This closes the "sandwich" hole where the op-log
sequence is `[connector:OTHER:apply, user:bren,
connector:OTHER:apply]` — under v1's rule (LatestEventSource
only), the engine would apply remote and clobber the user click;
under v2 the user click prevents the apply.

**Non-guarantees explicitly disclosed:**

- **Concurrent third-party field edits may be lost.** When the
  user toggles `checked` and a third party edits `text` on the
  same item within the same tick window, the wiki-wins repatch
  carries the wiki's `text` value and overwrites the third
  party's edit. The third party's edit is silently destroyed.
  See §10.
- **Multi-connector binding is refused at bind time.** Per §16.9
  the aggregate-root key is `(Page, ListName)` with no `Kind`
  component; the bind ceremony refuses any second binding to a
  checklist already bound by any kind, any profile. The runtime
  multi-connector race that an earlier draft described is moot:
  the configuration that would produce it is rejected before
  either connector starts ticking. Multi-process operation
  (separate concern, §10.8) can violate this if two engine
  processes compete on the same data dir.
- **In-flight user clicks may be silently lost.** If a user
  click lands in the op-log AFTER classify (step 3) but BEFORE
  apply (step 5) within the same tick, classify did not see it;
  the apply may revert it AND the cursor advance at step 9 can
  cover the user event without it ever being pushed (because
  cursor advance reads `max self-event seq`, and the apply's
  self-event has a higher seq than the user event). The strict
  invariant above is a *per-tick* property at classify time, not
  an *instantaneous* property. See §10.14 for the full
  description and the proposed mitigations.

---

## 5. Per-binding state shape

```toml
[wiki.connectors.<kind>.bindings[N]]
page = "shopping_lists"                 # M (memory: durable identity)
list_name = "Grocery"                   # M
remote_handle = "<vendor list ID>"      # M (set at bind, never changes)
remote_list_title = "Groceries"         # G (vendor renames propagate via title sync)
state = "active" | "paused"             # M
paused_reason = "auth_failed" | ""      # M
paused_at = "2026-…"                    # M
bound_at = "2026-…"                     # M (immutable after bind)
last_synced_seq = 64                    # M (cursor, monotone)
last_successful_sync_at = "2026-…"      # M

[wiki.connectors.<kind>.bindings[N].adapter_state]
# Common: dead-letter bookkeeping (M, durable failure history)
push_failures.<uid>.count = 3
push_failures.<uid>.next_attempt_at = "2026-…"

# Tasks-specific
item_id_map.<wiki-uid> = "<task-id>"    # M (engine-owned mapping)
item_etags.<task-id> = "\"<etag>\""     # G (vendor-side concurrency token)
last_updated_min = "2026-…"             # G (cursor, vendor's perspective)

# Keep-specific
item_id_map.<wiki-uid> = "<server-id>"  # M (engine's flat shape, used to route insert vs patch)
item_mapping.<server-id>                # G (adapter's structured shape)
  .server_id, .base_version, .client_id
keep_cursor = "<server token>"          # G
keep_note_client_id = "<list cli id>"   # G (required for label CRUD; self-heal hook in §11.2)
label_ids.<wiki-tag> = "<keep label id>" # G
```

**M = memory (durable, engine-owned, no apology needed); G =
guess (cached prediction about external system; must have explicit
refresh and apology paths).** See §9 for guess apology paths.

---

## 6. Primitive contracts

For each adapter primitive, the contract specifies effect class,
retry safety, and what the adapter must do to make the claim
true. Adapters are audited against these contracts in their tests.

### `PullRemote(ctx, binding) → (RemotePullResult, error)`

- **Effect:** Read-only on the engine's side; observes vendor
  state.
- **Retry safety:** Idempotent (re-pulling doesn't change vendor).
- **Truncation:** May return `Truncated=true`; engine triggers
  `ForceFullResync` next tick (rate-limited per §11.6). Partial
  inbound apply from a truncated pull IS performed (the items
  returned are valid; the truncation just means more exist beyond).

### `InsertRemote(ctx, binding, item) → (RemoteRef, error)`

- **Effect:** Creates an item on the vendor.
- **Retry safety:** Requires deterministic dedup key on Keep
  (uses `client_id` derived from wiki uid via `buildKeepItemID`).
  Tasks does NOT have client-side dedup keys; a retry after
  network swallow may produce a duplicate. Mitigation: engine
  treats network errors on Insert as `Retryable` only when the
  adapter can guarantee no partial commit (gateway timeout
  semantics). On ambiguous failure, the next tick's PullRemote
  will reveal the duplicate (both items appear in the response);
  the second occurrence is currently treated as a foreign item.
  **Non-guarantee:** Tasks Insert is NOT guaranteed exactly-once
  under network ambiguity. Tracked as known limitation §10.

### `PatchRemote(ctx, binding, ref, item) → (RemoteRef, error)`

- **Effect:** Updates an existing item on the vendor.
- **Retry safety:** Idempotent at the field level — the same
  fields, sent twice, produce the same vendor state. The vendor's
  concurrency token bumps each time, but this is benign (next
  tick's PullRemote refreshes baseline).
- **Idempotence note:** The engine's `AppendSyncEvent` for a
  retried patch creates a duplicate self-event. This is forensic
  noise but not a correctness issue (cursor advance is monotone).

### `DeleteRemote(ctx, binding, ref) → error`

- **Effect:** Removes an item on the vendor.
- **Retry safety:** Idempotent. Repeated deletes are no-ops on
  both Tasks (404 silently swallowed) and Keep (Trashed/Deleted
  timestamp updates idempotently).

### `RefreshItemBaseline(binding, remote) → Binding`

- **Effect:** Updates the engine's stored concurrency token for
  one item from a freshly-read RemoteItem.
- **Retry safety:** Idempotent at the field level.
- **Disclosed trade-off:** Disarms remote OCC for the duration
  of the tick (§5 detail in step 5).

### `SyncCollectionState(ctx, binding, items) → (Binding, error)`

- **Effect:** Reconciles per-binding remote state from current
  wiki items (Keep: hashtag → label CRUD; Tasks: no-op).
- **Retry safety:** Idempotent — labels are content-addressed by
  name; existing labels are reused, missing ones minted.
- **Failure handling:** Errors logged but do NOT abort the tick.

### `RebuildAdapterState(ctx, binding) → (AdapterState, error)`

- **Effect:** Full re-pull from vendor; rebuilds all guesses
  (etag map, BaseVersion map, idMap, etc.) from scratch.
- **Retry safety:** Idempotent — full rebuild produces same
  result modulo vendor-side concurrent writes.
- **When invoked:** ForceFullResync (operator-triggered or
  pause ≥ 7d), Insert-recovery on PreconditionFailed, pull
  truncation.

### `ReadRemoteByRef(ctx, binding, ref) → (RemoteItem, error)`

- **Effect:** Read-only single-item fetch.
- **Retry safety:** Idempotent.
- **TOCTOU window:** The vendor state read here may differ from
  the state at the subsequent re-PATCH. The engine accepts this
  as the "wiki-wins always" branch's concurrency window (§10).

---

## 7. Mid-tick crash recovery

Steps 4–10 of the reconcile loop are NOT a transaction. The
engine touches two stores (op-log + binding) and a remote (the
vendor API) within a single tick. Crash points and recovery:

| Crash point | On-disk state after recovery | Next tick's behavior |
|---|---|---|
| Mid-PullRemote | Unchanged (read-only). | Re-pull on next tick. |
| Between PullRemote and applyInbound | adapter_state unchanged. | Re-pull, re-apply. Idempotent: refresh-baseline writes same value; UpdateItemForSync is idempotent on identical input. |
| Mid-applyInbound (between item N and N+1) | Items 0..N applied to wiki via UpdateItemForSync (durable on wiki write). adapter_state baselines refreshed for items 0..N (in-memory only — NOT yet saved). | Re-pull, re-refresh, re-apply. Items 0..N's apply is a no-op (wiki state already matches); items N+1..end apply normally. |
| Between applyInbound and pushOutbound | Wiki has applied changes; adapter_state baselines in-memory only. | Re-pull will re-fetch with stored (stale) baseline; if the wiki side's diff against current wiki state still triggers WikiDiverged, push proceeds. Push uses the freshly-refreshed baseline (not the stale on-disk one). |
| Mid-pushOutbound (after PatchRemote success, before AppendSyncEvent) | Vendor has the patch; wiki op-log has no `outbound_patched` event. | Next tick's classify still sees the user-source event (cursor unchanged). It re-pushes. Vendor accepts (idempotent). Spurious self-event is appended this time. Cursor advances. |
| Mid-pushOutbound (after AppendSyncEvent, before SaveBinding) | Op-log has self-event; binding cursor on disk still old. | Next tick's classify reads op-log, sees self-event, advances cursor (`max self-event seq from op-log`). The advanceLastSyncedSeq rule is recovery-correct: it reads from op-log, not from prior tick's in-memory state. |
| Mid-SyncCollectionState (Keep label CRUD; multi-call sequence: label create → LIST node update) | Vendor may have a half-committed label set: the label exists server-side but `label_ids[name]` was never persisted, OR `label_ids[name]` persisted but the LIST node's tag-set on the server was never updated. | Next tick's `SyncCollectionState` is invoked again. The vendor's label-create primitive is name-keyed and idempotent (re-creating a same-name label returns the existing ID). The LIST-node tag-set update is also idempotent on the desired set. **Footgun:** if the local `label_ids` map gained an entry for a label that is no longer wanted (rare, but possible if state mutated between crash and recovery), the orphan entry persists until ForceFullResync. Tracked as known limitation. |
| Mid-SaveBinding | Partial write; binding row may be invalid. | The frontmatter persistence layer is single-write atomic (TOML round-trip). Either the new binding lands or it doesn't. On corruption: binding load fails, engine logs and skips the tick; operator restores from git. |

**Required invariant:** every step's effect must be replayable
from prior on-disk state without producing a different external
effect than running the tick from scratch would. Every primitive
in §6 is audited against this.

---

## 8. Op-log event sources

| `src` prefix | Meaning | Effect on `WikiDiverged` for `<this-kind>` | Effect on `UncoveredUserEvent` |
|---|---|---|---|
| `user:<email>` | Operator clicked in wiki UI | Diverged | True |
| `connector:<this-kind>:apply` | This engine's inbound apply | Not diverged | False |
| `connector:<this-kind>:outbound_push` | This engine's outbound primitive | Not diverged | False |
| `connector:<this-kind>:push_recovery` | This engine's recovery branch | Not diverged | False |
| `connector:<other-kind>:*` | Another connector's apply/push | Diverged (cross-connector) | False |
| `migration:*` | Migration job seeded baseline | Diverged (conservative) | False |
| `system:*` | Future system writes | Diverged (conservative) | False |

Op-log event ops (informational; engine doesn't switch on these):
`baseline`, `add`, `toggle`, `set_text`, `set_tags`, `delete`,
`bulk_update`, `outbound_inserted`, `outbound_patched`,
`outbound_deleted`.

---

## 9. Apology paths for guesses (G fields)

Each guess in §5 must have a refresh path (when does it get
updated?) and an apology path (what happens when it's wrong?).

### 9.1 `item_etags` / `item_mapping[…].base_version`

- **Refresh:** `RefreshItemBaseline` called for every non-deleted
  remote item in `applyInbound` (step 5).
- **Apology:** PatchRemote returning `PreconditionFailed` →
  `runPreconditionRecovery` → re-read via `ReadRemoteByRef` →
  refresh baseline → re-PATCH.

### 9.2 `idMap` / `keep_note_client_id` / `keep_cursor` /

`last_updated_min`

- **Refresh:** populated at bind via `SeedBindingState`; updated
  per-tick by `AdvanceCursor` (cursors), inbound apply (idMap on
  successful insert), and `RefreshItemBaseline` (per-item
  mapping).
- **Apology paths:**
  - `idMap` empty AND `remote_handle` non-empty → Insert-recovery
    via `RebuildAdapterState` on first PreconditionFailed insert.
  - `keep_note_client_id` empty → PullRemote self-heal (§11.2):
    capture from LIST node if in incremental pull, else trigger
    ForceFullResync.
  - `keep_cursor` rejected by vendor → `Truncated=true` → next
    tick triggers ForceFullResync.

### 9.3 `push_failures.<uid>` (dead-letter lifecycle)

**Entry:** `Retryable` error on Insert/Patch/Delete →
`recordPushFailure` → bump `count`, set `next_attempt_at` from
exponential backoff.

**Skip:** Subsequent ticks' pushOutbound calls
`shouldSkipPush(binding, uid)` which returns `(true, "backoff")`
if `now < next_attempt_at`, `(true, "dead_letter")` if `count >=
10`.

**Exit conditions:**

1. `recordPushSuccess` clears `push_failures.<uid>` entirely on
   any successful primitive for that uid. NOTE: a dead-lettered
   uid never reaches the primitive (skipped in pushOutbound), so
   this exit fires only after another exit clears the entry.
2. **Any user event for the uid in the wiki op-log** clears the
   entry. The user re-edited; the engine respects the new intent.
   (Implemented as: on classify, if uid has a user-source event
   with `seq > cursor`, clear `push_failures.<uid>` before the
   pushOutbound iteration.)
3. `ForceFullResync` clears all `push_failures` (the rebuild
   wipes adapter_state).
4. Operator-triggered "clear dead letters" RPC (planned; tracked
   as known limitation in §10).

**Operator-visible signal:** when any uid is dead-lettered, the
binding's status surfaces `degraded_reason="dead_letter:N items"`
alongside `state` and `paused_reason`. This is an explicit signal
that `last_successful_sync_at` advancing is misleading — work has
been silently skipped.

---

## 10. What this engine does NOT guarantee

The reviewers were unanimous that v1 was missing this section.
Each non-guarantee below is named so an operator reading the
rules at 2am knows what classes of bugs are out-of-scope vs.
in-scope.

### Availability stance

The wiki replica is a single writer and a single reader. The
engine prioritizes wiki availability over remote consistency: a
vendor partition does not block wiki writes (the op-log accepts
unconditionally; sync resumes when the partition heals). The
engine does not implement read-your-writes or monotonic-read
guarantees across the wiki/remote boundary; it implements
eventual convergence to wiki-wins-on-conflict (§4) modulo the
non-guarantees enumerated below. No formal HAT-tier
classification is claimed.

### 10.1 No happens-before across replicas

`seq` is wiki-local. The engine cannot determine whether a wiki
event "happened-before" or was "concurrent with" a remote event.
All cross-replica reasoning is via `RemoteDiverged` (a
change-detection signal, not an order signal).

### 10.2 No CRDT-style commutative merge

The 4-cell merge is order-sensitive at the wiki via the
`UncoveredUserEvent` predicate. Two ticks running with different
arrival schedules can produce different results.

### 10.3 No concurrent-field merge

When a user toggles `checked` in the wiki and a third party
edits `text` of the same item within the same tick window, the
wiki-wins repatch sends the wiki's full WikiToRemote payload —
including `text` — and overwrites the third-party edit. The
engine does NOT detect that only `checked` was user-touched and
limit the repatch to that field. This is the "lost update under
concurrent field edits" non-guarantee.

### 10.4 No multi-connector binding (configuration refused at bind time)

Multi-connector binding to the same checklist is **not a runtime
non-guarantee — it is refused at bind time**. The aggregate-root
key per ADR-0011 is `(Page, ListName)` with no `Kind` component,
and the engine's bind ceremony (§12) checks
`LeaseTable.LookupOwner` inside the per-checklist mutex (see
`internal/connectors/engine/bind.go:69`). Any second bind to a
checklist already bound — by any kind, any profile — returns
`ErrAlreadyBoundForChecklist`. Tested at
`internal/connectors/engine/bind_test.go:244` with explicit
cross-kind setup. See strict invariant §16.9.

This means push-pull oscillation between two connectors over the
same checklist cannot occur in practice; the configuration that
would produce it is rejected before either connector starts
ticking.

### 10.5 No exactly-once Insert under network ambiguity

Tasks Insert lacks a deterministic dedup key. A retry after
gateway timeout may produce a duplicate item. The duplicate
appears as a foreign item on next pull (not in our `idMap`); the
engine treats it as a remote-side add and mirrors it to wiki.
The operator sees two copies of the same item.

### 10.6 No transactional atomicity across reconcile steps

Mid-tick crash leaves partial state. §7 enumerates the recovery
path for each crash point. The recovery is *eventually*
consistent, not transactional.

### 10.7 No protection against op-log corruption

The op-log is the divergence authority. If it is truncated,
restored from a stale backup, or if `seq` is reset, divergence
detection produces undefined results. There is no checksum, no
merkle tree, no operator-visible "op-log healthy" signal.

### 10.8 No cross-process safety

The engine assumes single-process execution per binding set.
The lease table is in-process (rebuilt at boot). Two engine
processes running simultaneously will both believe they hold
all leases, both tag their writes `connector:<kind>:*`, and
race on binding state writes. Operator MUST ensure singleton
via systemd / deployment discipline. (See §11.4.)

### 10.9 No bidirectional merge of user-vs-user concurrent edits

"Concurrent" = user-A clicks in wiki AND user-B clicks in remote
app, within the same tick window. The wiki-wins repatch
overwrites user-B's click without notification. The engine does
not surface a conflict to either user.

### 10.10 No bind-time replay of in-flight user events

At bind time, `LastSyncedSeq` is set to `max(seq)` on the bound
checklist. Any pending user events from BEFORE the bind that
weren't yet covered are silently considered "synced" — the first
post-bind tick will not re-push them. Operator should ensure no
clicking during bind setup.

### 10.11 No protection against vendor-token-collapse on

non-content events
When a vendor (e.g., Tasks) bumps its concurrency token without
changing observable content, our `RemoteDiverged=true` signal is
spurious. The 4-cell merge handles this via the `wd ∧ rd ∧ user`
sticky-user-wins rule, but the operator should not interpret
"RemoteDiverged=true" as "remote changed."

### 10.12 No formal proof of convergence

The reviewers' formal analysis (Shapiro, Lamport) shows the
engine is convergent for the single-connector + single-replica
case under the schedule arrival assumptions in §3. Multi-
connector configurations are refused at bind time (§16.9), so
multi-connector convergence is moot rather than unguaranteed. A
formal TLA+ proof is not in scope.

### 10.13 (resolved — sticky user-wins applies to the Deleted cell)

This entry described the engine mirroring a remote delete to the
wiki even when an uncovered user event existed for the same uid.
v3.1 reorders `applyInboundOneItem`'s `Deleted=true` branch in
`internal/connectors/engine/reconcile.go`: when
`UncoveredUserEvent=true` for the uid, the engine clears `idMap`
and skips `DeleteItemForSync`. The next `pushOutbound` sees no
`idMap` entry → INSERTs a fresh remote ref carrying the wiki's
user-edited state. Tested at
`internal/connectors/engine/reconcile_test.go` (the "Deleted=true
AND uid has UncoveredUserEvent" When-block) and reflected in the
parity test under "USER SCENARIO: remote item is deleted while
operator was editing in wiki." Sticky user-wins is now enforced
for both deleted and non-deleted cells; strict invariant §16.3
holds without the v3 carve-out.

### 10.14 In-flight user clicks during apply may be silently lost

The strict user-wins invariant (§16.3) is a per-tick property at
classify time, not an instantaneous property. If a user click
lands in the op-log AFTER classify (step 3) but BEFORE
applyInbound (step 5) within the same tick, classify did not see
it; the apply may revert it. The cursor then advances to
`max(self-event seq)` at step 9, which can exceed the user
event's `seq`, marking the user event as covered without it
having been pushed. The next tick's classify treats the user
event as already-synced and never re-pushes it. The user's click
is silently lost. Mitigation requires either a stricter cursor
advance rule (advance only past self-events that succeed every
user event of equal-or-lesser seq for the same uid) or running
classify under the same lock that gates user writes. Tracked as
known limitation.

### 10.15 (resolved — see §16.9)

This entry described "engine accepts second connector binding
silently." The disclosure was factually incorrect: the bind
ceremony already refuses, see §10.4 above and §16.9. Retained as
a numbered entry for stable cross-references; the actual
contract is the strict invariant §16.9.

### 10.16 No mechanical defense against multi-process operation

The engine does not acquire a process-exclusive lock at startup.
A second wiki process pointed at the same data directory will
boot, contend on the binding TOML, and corrupt op-log seq
allocation. Operator MUST ensure singleton via deployment
discipline (§10.8, §11.4). Tracked: a `flock(2)` on a pidfile
inside the data directory would close this with ~20 LOC of Go
and would also defeat the most common cause (two dev shells run
against the same dir).

---

## 11. Operational sections

### 11.1 Forensic state catalog

When the operator inspects a binding, these state combinations
indicate specific conditions:

| Observed state | What it means | Operator action |
|---|---|---|
| `last_synced_seq < max(op-log seq)` AND latest op-log src is `connector:<this-kind>:*` | Mid-tick crash; engine restarting. | Wait one tick (≤30s). |
| `last_synced_seq < max(op-log seq)` AND latest op-log src is `user:*` or other | Expected divergent state; tick should be in-flight or imminent. | Wait one tick. |
| `last_synced_seq` not advancing across multiple ticks AND `last_successful_sync_at` not advancing | Stuck (auth failed, vendor unreachable, or bug). | Check `paused_reason`; check vendor status; check journalctl for `reconcile_*` log lines. |
| `state=active` AND `paused_reason=""` AND `degraded_reason="dead_letter:N items"` | Some uids skipped permanently; others syncing fine. | Investigate dead-lettered uids; force-resync the binding to clear. |
| `push_failures.<uid>.count >= 10` | Dead-lettered. | User-edit clears; force-resync clears; (planned) operator API clears. |
| `state=paused` AND `paused_reason=auth_failed` | Token expired or revoked. | Reconnect via UI. |
| `state=paused` AND `paused_at` > 7 days ago AND user resumes | Resume triggers ForceFullResync. | Wait for one tick after resume. |
| `idMap` empty AND `remote_handle` non-empty | Insert-recovery will trigger on next push. | Wait one tick; or force-resync. |
| `remote_handle` empty | Migration gap; binding fundamentally broken. | Operator must unbind + rebind. |
| `keep_note_client_id` empty AND `remote_handle` non-empty | Self-heal will capture from LIST node on next pull (or trigger ForceFullResync). | Wait one tick. |
| `keep_cursor` invalid (engine logs `truncated_pull`) | ForceFullResync triggered next tick. | Wait one tick. |
| Op-log has `outbound_patched` immediately after `user:` event with same uid AND tick advanced to that seq | Healthy: push round-tripped. | None. |
| Op-log has `connector:<other-kind>:apply` and stays without subsequent `connector:<this-kind>:*` event for the same uid | This connector's tick hasn't run yet, OR `wd ∧ rd ∧ ¬user` fired and remote-wins applied. | If unexpected, check classify behavior. |
| `push_failures.<uid>.count = N` where `1 ≤ N < 10` | In exponential backoff per §11.7; not yet dead-lettered. Next attempt at `next_attempt_at`. | Wait for backoff to elapse, or force-resync to clear. |
| `last_successful_sync_at` advancing AND op-log shows `user:*` events with no subsequent `outbound_*` events for ≥2 minutes for the same uid | Push side is failing without dead-lettering yet (vendor 5xx storm or transient gateway error). PullRemote still works so `last_successful_sync_at` is misleading. | Check journalctl for `recordPushFailure` log lines; check vendor status page; wait for backoff to elapse. |
| Op-log has `outbound_patched` AND `last_synced_seq` did not advance to that seq on the next tick | Mid-pushOutbound crash window per §7 row 5 (push succeeded, AppendSyncEvent landed, SaveBinding did not). Recovery is the cursor-advance-from-op-log rule. | Wait one tick. If multiple ticks elapse without advancement, escalate. |
| `bound_at` very recent (≤30s ago) AND `last_synced_seq` already equals `max(op-log seq)` AND user reports pre-bind clicks didn't sync | Bind-time replay limitation per §10.10. Pre-bind user clicks are silently considered synced; first tick won't re-push them. | Re-touch each affected item to re-emit a user event; engine will push on next tick. |
| Op-log shows alternating `connector:<A>:apply` / `connector:<B>:apply` for the same uid across multiple ticks with no `outbound_*` between | **Should be impossible** per §16.9 (at most one Binding per checklist; bind ceremony refuses cross-kind). If observed, the bind-time refusal was bypassed (e.g., direct frontmatter edit, restored backup with a stale binding from a different kind). | Inspect profile pages for two simultaneous `bindings[]` entries on the same `(page, list_name)`. Remove one. The lease table will rebuild from disk on next boot. |
| `last_synced_seq` observed to *decrease* between two reads of the same binding | Two engine processes are racing on the binding (§10.8, §10.16). Data integrity at risk. | Stop one process immediately. Check `ps`, deployment systemd units. Restore binding from git if cursor went into invalid state. |
| `truncated_pull` log line appears on consecutive ticks | ForceFullResync loop per §11.6 (rate limit not enforced). | Pause the binding to break the loop until rate-limit fix lands. Investigate why each pull is being truncated (vendor cursor expired? state size?). |
| `item_etags.<task-id>` value missing for a uid present in `idMap` | Stale/incomplete adapter_state — likely from a partial rebuild, migration drop, or an item that was never RefreshItemBaseline'd. | Force-resync the binding to repopulate adapter_state. |

### 11.2 Self-heal hooks

- **`keep_note_client_id` self-heal** (`PullRemote`): when stored
  client_id is empty AND `remote_handle` is non-empty AND the
  LIST node is in the pull → capture from the LIST node. If LIST
  node not in pull → set `Truncated=true` (engine triggers
  `ForceFullResync` → full pulls always include LIST). Bails out
  when `remote_handle` is empty (re-bind is the only recovery).

- **Tombstone with cleared parent linkage** (`PullRemote`): Keep
  may clear ParentID/ParentServerID on user-deleted-in-app
  tombstones. Filter accepts items whose ServerID is in our
  `item_id_map` regardless of parent.

- **Insert-recovery via `RebuildAdapterState`**: on
  PreconditionFailed insert, full rebuild repopulates idMap;
  next tick routes through Patch.

### 11.3 Lease table lifecycle

- **Scope:** in-process. Rebuilt at boot from frontmatter
  (`profile_*` pages' bindings).
- **Granularity:** per-checklist (`profile, page, list_name`).
- **Failure semantics:** lease holder crash → leader-election
  N/A (single-process); whole-process restart rebuilds lease
  table from binding state.
- **Required external discipline:** singleton enforcement via
  systemd / deployment. Two engine processes running
  simultaneously will race; correctness is not guaranteed.
- **Lease scope vs. ticks:** structural ops (bind, unbind,
  ForceFullResync) hold per-checklist lock for the whole
  operation. Per-tick reconciles also acquire the lock; if held,
  the tick yields (the next debouncer/cron firing will retry).

### 11.4 Multi-replica caveat

Self-events are tagged `connector:<kind>:<op>`, which is
**kind-scoped**, not **replica-scoped**. If two engine processes
of the same kind run simultaneously, each tags its writes
identically; each treats the other's writes as self-writes.
Result: both believe they own the binding; binding state
mutations race; cursor advance is undefined.

**Mitigation:** the operator MUST ensure singleton (§11.3).
Defense-in-depth proposal (not yet implemented): tag with
`connector:<kind>:<replica-id>:<op>` and detect cross-replica
self-events as `migration:foreign-replica` (treat as divergent).
Tracked as known limitation.

### 11.5 The 5-second `rateLimitChoke`

Post-success choke: a tick within 5s of the previous successful
tick is a no-op. **Unjustified historical constant.** Original
intent: defend against tight loops where the debouncer might
re-fire on a self-event. With `cursor advances only past
self-writes` + suppressor, this defense is redundant. Retained
for now; replacement by token-bucket against vendor quota is
tracked as known limitation.

### 11.6 ForceFullResync rate limit

ForceFullResync triggered by truncation, pause-resume horizon,
or operator request. **Open:** rate limiting between
auto-triggers is not currently enforced; in pathological
truncated-pull scenarios this could loop. Tracked as known
limitation.

### 11.7 Backoff schedule

`pushFailureBackoff(n)`: `60s * 2^(n-1)`, capped at `1h`.
Ten failures span: 60s + 120s + 240s + ... + 30720s (capped),
total ~17 minutes of attempts before dead-letter.

**Known limitation:** retry backoff is per-uid, not per-binding
or per-(profile, kind). During a vendor-wide outage, all uids on
all bindings of that kind enter independent backoff schedules
simultaneously, producing thundering-herd retry against an
already-unhealthy vendor. Per-(profile, kind) circuit-breaker
tracking consecutive 5xx + auto-pause is tracked as future work.

### 11.8 Cost model (informational)

Per binding per tick at steady state, in vendor RPCs:

- **PullRemote:** 1 (incremental; may paginate at very large
  lists).
- **applyInbound:** 0 (engine-internal; wiki writes only).
- **pushOutbound:** 1 per uid with `WikiDiverged=true` (no
  batching primitive yet — see known limitation below).
- **SyncCollectionState:** Keep — 0 if no new tags this tick,
  else 1 Changes request to update LIST node tag-set and 1
  per new label create. Tasks — 0.
- **AdvanceCursor / SaveBinding:** 0 (wiki-internal).

Worst-case sustained: `(1 + |changed_items|) RPC / tick` per
binding. With 30s tick interval, Tasks at 600 RPC/min/user
quota allows ≈ 300 changed items per 30s before quota pressure
on a single binding. Multi-binding deployments must sum across
bindings; multi-connector deployments multiply by the number of
connectors per checklist.

**Known limitation:** the BackendAdapter contract does not
declare a batch primitive. Per-tick cost scales O(items
requiring push) in RPCs. At current single-binding usage this is
within vendor quotas; multi-binding scale-out will require
revisiting the contract to add `BatchPatchRemote` /
`BatchInsertRemote`.

### 11.9 Per-RPC deadlines (engine-enforced)

**Enforced by the engine** (round-3 panel, Bailis Option A
"invariant by construction"). Every engine→adapter I/O primitive
call site wraps the inbound `ctx` with
`context.WithTimeout(ctx, PerRPCDeadline)` via the engine helper
`Engine.withRPCDeadline` defined in
`internal/connectors/engine/rpc_deadline.go`. The default
deadline is 15s; the variable is overridable for tests but the
production code path always reads the same shared value.

**Wrapped call sites (13, audited; opengrep-enforced):**

- `reconcile.go` (5): PullRemote, InsertRemote (in pushOutbound
  loop), PatchRemote (in pushOutbound loop), DeleteRemote (in
  pushOutbound loop), SyncCollectionState
- `precondition_recovery.go` (2): ReadRemoteByRef, PatchRemote
  (re-PATCH branch)
- `bind.go` (2): ValidateRemoteBinding, SeedBindingState
- `force_resync.go` (1): RebuildAdapterState
- `insert_recovery.go` (1): RebuildAdapterState
- `title_sync.go` (2): FetchRemoteListTitle, ListRemoteCollections

The `.semgrep/rules.yml` rule
`go.engine-adapter-call-needs-deadline` flags any direct
`e.adapter.<I/O primitive>(ctx, ...)` call inside the engine
package as ERROR, blocking new bypasses at edit time.

**Why every site is wrapped:** the deadline is the engine's
responsibility, not the adapter's. Adapters are not trusted to
honor deadlines on their own — and need not be, because the
engine binds the deadline to the ctx before the call. A future
adapter (e.g., iCloud Reminders) inherits the deadline guarantee
without having to implement it.

**Failure mode prevented:** a vendor-side hang on a single RPC
no longer holds the per-checklist lease (§11.3) indefinitely.
After 15s the primitive returns with `context.DeadlineExceeded`;
the engine's error-classification path (typically
`ErrorClassRetryable`) bumps `push_failures.<uid>.count` and the
backoff schedule (§11.7) takes over.

**Tested:** unit-tested at
`internal/connectors/engine/rpc_deadline_test.go` (deadline is
applied; cancel propagates; tight-deadline override fires).

---

## 12. Bind ceremony

1. Resolve credentials for the profile.
2. Acquire per-checklist lease.
3. Validate remote (`ValidateRemoteBinding`): for Tasks, reject
   lists with subtasks; for Keep, reject non-LIST notes and
   trashed/deleted notes.
4. `SeedBindingState`: full pull of remote items, populate
   adapter state. For Keep: capture LIST node's `client_id` from
   the pull. The first post-bind tick handles remote items not
   yet in wiki via the `WikiDiverged=false ∧ RemoteDiverged=true`
   cell of applyInbound.
5. Insert `Binding` into store with `state=active`,
   `LastSyncedSeq = max(seq)` on the bound checklist at bind
   time. **Documented trade-off (v2):** any pending user events
   from before the bind that weren't yet covered are considered
   "synced" — the first post-bind tick will not re-push them
   (§10.10).
6. Release lease on success; on any error roll back.

---

## 13. Unbind ceremony

1. Acquire per-checklist lease.
2. Remove binding from store.
3. Release lease. Op-log on the wiki is preserved (history is
   immutable).

---

## 14. Pause / Resume / auth-failed transitions

- `AuthFailed` from any adapter primitive →
  `applyPausedTransition` with `PausedReason = "auth_failed"`.
  Steady-state, not an error.
- `Resume` (operator-triggered or reconnect): if pause duration
  ≥ 7d, run `runForceFullResync` (cursors possibly invalid);
  otherwise unpause and resume normal ticks.

---

## 15. Migrations from the legacy connector

- `subscriptions[]` → `bindings[]` (Phase 7, idempotent eager).
- Legacy adapter state translated:
  - Tasks: `synced_items` dropped (observed-divergence
    classification replaces fingerprint comparison).
  - Keep: `item_id_map[uid] = ItemMapping{…}` → flat
    `item_id_map[uid] = serverID` (engine) AND structured
    `item_mapping[serverID] = {server_id, base_version,
    client_id}` (adapter). Legacy fingerprint baselines
    (`synced_text/checked/sort_value`) dropped.
  - Keep: legacy field aliases translated:
    `keep_note_id → remote_handle`,
    `keep_note_title → remote_list_title`.

---

## 16. Strict invariants

These are properties the engine MUST enforce. Each is paired
with the rule(s) that enforce it.

1. **Cursor monotonicity.** `LastSyncedSeq` only ever increases
   for a given binding. Enforced by `advanceLastSyncedSeq`'s
   `max self-event seq` rule and `SaveBinding` atomicity.

2. **Self-events are non-divergent.** Events with `src` starting
   `connector:<this-kind>:` do not contribute to `WikiDiverged`.
   Enforced by `isDivergentSource` + the source table in §8.

3. **Sticky user-wins.** When ANY uncovered user event exists for
   uid `u`, the engine does not apply remote state to wiki for
   `u` until a self-event covers the latest user event for `u`.
   Enforced by `UncoveredUserEvent` predicate in §3 + the 4-cell
   rule in §2 step 5.

4. **Self-events are tagged at the source.** Every wiki write
   from inside the engine MUST go through a code path that calls
   `WithSource(ctx, ConnectorSource(kind, op))` before invoking
   the mutator. Enforced by the engine code organization
   (apply/outbound_push/push_recovery branches each set source);
   not currently enforced by a lint or test, so any new wiki
   write path MUST add the source explicitly. Failure mode: a
   missed source tag would make the engine's own write classify
   as `user:*` and trigger a self-divergence loop.

5. **Suppressor wraps every wiki write that mirrors a remote
   read.** Required for: applyInbound's UpdateItemForSync,
   AddItemForSync, DeleteItemForSync; precondRemoteDeleted's
   DeleteItemForSync. Enforced by the engine code; not currently
   enforced by lint.

6. **Per-binding mutations are serialized within a process.**
   The `LeaseTable`'s per-checklist lock ensures only one tick
   (or one bind/unbind) runs at a time per `(profile, page,
   list_name)`. Cross-process is not guaranteed (§10.8, §11.4).

7. **Primitive contracts hold (§6).** Every adapter must satisfy
   the effect-class + retry-safety claims for each primitive.

8. **Crash-replay safety.** Every step's effect must be
   replayable from prior on-disk state without producing a
   different external effect than running the tick from scratch
   (§7).

9. **At most one Binding per `(Page, ListName)`.** The aggregate-
   root key per ADR-0011 is `(Page, ListName)` with no `Kind`
   component. A second bind to a checklist already bound — by
   any kind, any profile — is refused with
   `ErrAlreadyBoundForChecklist`. Enforced by
   `LeaseTable.LookupOwner` against `ChecklistKey` inside the
   per-checklist mutex of the bind ceremony (§12 step 3),
   tested at `engine/bind_test.go:244` with cross-kind setup.
   This invariant is what makes multi-connector convergence
   (§10.4) a moot question rather than a runtime race.

10. **Per-RPC deadlines applied at engine→adapter boundary.**
    Every I/O-bearing adapter primitive call goes through
    `Engine.withRPCDeadline(ctx)` before invocation. Deadline
    is `PerRPCDeadline` (default 15s, overridable for tests).
    Enforced by code organization: 13 call sites audited (§11.9),
    no engine code path invokes an adapter I/O primitive on a
    deadline-less ctx. Tested at `engine/rpc_deadline_test.go`.

---

## 17. References

- ADR-0011: ChecklistBinding aggregate.
- ADR-0012: Connector abstraction (engine + adapter).
- ADR-0015: Per-checklist operation log + observed-divergence
  classification (formerly "causal divergence").
- MATRIX.md: per-row audit of which behaviors are engine-owned
  vs. adapter-specific.
- This document: authoritative single source of truth for the
  sync contract. v3 incorporates round-2 panel findings from
  Kleppmann, Helland, Shapiro, Bailis, and Lamport (2026-05-07):
  added §10.13–§10.16 (Deleted+UCE hole, in-flight click loss,
  multi-connector bind warning, multi-process defense),
  availability stance preface to §10, mid-SyncCollectionState
  crash row in §7, seven additional forensic catalog rows in
  §11.1, cost model §11.8, per-RPC deadline contract §11.9.
  v3.1 (same date) corrects a factual error: §10.4 originally
  described multi-connector binding as a runtime non-guarantee,
  but the bind ceremony already refuses the configuration via
  the kindless aggregate-root key per ADR-0011. §10.4 rewritten
  to describe the bind-time refusal; §10.15 retained as a
  resolved cross-reference; new strict invariant §16.9 added.
  v4 (round-3 final, same date) lands two engine fixes the
  panel called blocking: (a) Lamport §10.13 — `applyInboundOneItem`
  reorders so `Deleted ∧ UncoveredUserEvent` clears `idMap` and
  preserves the wiki state for re-INSERT via pushOutbound,
  rather than mirroring the remote delete; (b) Bailis Option A
  — every engine→adapter I/O primitive call site wraps `ctx`
  with `Engine.withRPCDeadline` so per-RPC deadlines are
  invariant by construction, not adapter-responsibility. New
  strict invariant §16.10. §10.13 and §10.15 retained as
  resolved cross-references.
