# Sync interaction matrix

This is the canonical inventory of every possible delta between wiki state
and Keep state at sync time, what the bridge should do for each, and which
test exercise covers it.

Three sources of truth at every tick:

- **Wiki**: items in `wiki.checklists.<list>.items[]` with `Uid`, `Text`,
  `Checked`, `Tags`, `Description`, `SortOrder`.
- **Keep**: LIST_ITEM nodes returned by `Changes.list` with `ServerID`,
  `Text`, `Checked`, `SortValue`, `Timestamps.{Trashed, Deleted}`,
  `BaseVersion`. (Wall-clock timestamps on Keep nodes are no longer used
  for divergence decisions; `Trashed`/`Deleted` non-zero remains a
  presence/state signal only.)
- **Binding id_map**: `wiki_uid → ItemBinding{ServerID, SyncedText,
  SyncedChecked, SyncedSortValue, …}` per binding, persisted on the user's
  profile. The `Synced*` fields are the per-item content fingerprint at
  the last successful sync — the merge-base for the three-way merge.

## Divergence model

The bridge decides what to do per-item by computing two boolean axes from
content fingerprints (no wall-clock comparisons):

- **`wd` (`wiki_diverged`)**: `Fingerprint(wiki_item) != synced_fp` —
  the wiki's current content differs from the merge-base.
- **`kd` (`keep_diverged`)**: `Fingerprint(keep_node) != synced_fp` —
  Keep's current content differs from the merge-base.

There is no time axis. There is no "newer" or "equal" or "epoch sentinel"
distinction. The four states of `(wd, kd)` exhaust the alive-vs-alive
conflict space.

### Scope and tie-breaker

- **Single-wiki-replica scope.** This design assumes one wiki replica
  syncing with N Keep accounts (one per binding). Convergence depends on
  Keep's `to_version` cursor being the single source of logical time and
  on the deterministic "Keep wins" tie-breaker. A second wiki replica
  with its own independent `synced_fp` baseline would break convergence
  and is explicitly out of scope.
- **"Keep wins" is the hardcoded tie-breaker for `(wd, kd)` conflicts**
  (B1). Configurability — a per-binding policy that picks "wiki wins"
  or some other rule — is deferred. There are no equal-timestamp or
  symmetric "wiki wins" cases in this matrix; if those are added later,
  they require new rule-table rows and tests.
- A single wiki list may be bound to **at most one Keep note per
  profile**; the per-user `keep_note_already_bound_by_you` check at bind
  time enforces this. Two different profiles binding the same wiki list
  to their own Keep notes is supported (each binding maintains its own
  `synced_fp` map and converges independently).

## Notation

| Tag | Meaning |
| --- | --- |
| W*n* | Wiki-side delta |
| K*n* | Keep-side delta |
| B*n* | Both sides changed simultaneously (conflict) |
| S*n* | id_map staleness (entries that no longer correspond to truth) |
| L*n* | Label / tag delta |
| E*n* | Edge / failure-mode delta |

## W — wiki-side changes

| ID | Scenario | Expected behavior |
| --- | --- | --- |
| W0 | No wiki changes, no Keep changes | No push, no apply (`¬wd ∧ ¬kd`). |
| W1 | New wiki item added (no id_map entry) | Push as fresh; pick up server_id from response, write into id_map with seeded synced_fp. |
| W2 | Wiki item checked toggled | `wd ∧ ¬kd` → push update; preserve id, parentServerId, baseVersion. |
| W3 | Wiki item text edited | `wd ∧ ¬kd` → push update; original client_id preserved, serverId distinct. |
| W4 | Wiki item tags edited | `wd ∧ ¬kd` → push with re-encoded text head (`#tag1 #tag2 — text`). |
| W5 | Wiki item description edited | `wd ∧ ¬kd` → push with re-encoded text body (`text\n— description`). |
| W6 | Wiki item sort_order changed | `wd ∧ ¬kd` → push with new SortValue. |
| W7 | Wiki item deleted (removed from items list) | Push soft-delete (Deleted=now, NOT Trashed); drop from id_map after success. |
| W8 | Wiki page tags changed | Re-resolve labels; LIST node push with updated labelIds. |

## K — Keep-side changes

| ID | Scenario | Expected behavior |
| --- | --- | --- |
| K0 | Keep returns empty / no changes | No apply. |
| K1a | New Keep item, no text-match in wiki | AddItemForSync; record uid → ItemBinding{ServerID, synced_fp=keep_fp} in id_map. |
| K1b | New Keep item, text-match in wiki | Adopt: record existing-wiki-uid → ItemBinding; do NOT duplicate. |
| K2 | Keep item checked toggled | `¬wd ∧ kd` → UpdateItemForSync with new Checked. |
| K3 | Keep item text edited | `¬wd ∧ kd` → UpdateItemForSync with new Text (parsed back into text/tags/description). |
| K4 | Keep item Trashed (Trashed timestamp non-zero) | DeleteItemForSync; remove from id_map. |
| K5 | Keep item Deleted (Deleted timestamp non-zero) | DeleteItemForSync; remove from id_map. |
| K4-hard | Keep hard-deletes (item missing from a complete pull, was paired in id_map, ¬wd) | Apply Keep hard-delete to wiki; drop id_map. Refuse if pull was truncated or bogus. |

(`Trashed`/`Deleted` non-zero is a presence/state signal independent of
`(wd, kd)`. They short-circuit the alive-vs-alive matrix.)

## B — both-side conflicts

For B-class scenarios (`wd ∧ kd`), the rule is **Keep wins**: apply Keep's
value to wiki, then `synced_fp` advances to the agreed fingerprint and the
push diff is empty for this item next tick. The wiki edit is discarded.
Rationale and configurability deferral are documented in the "Scope and
tie-breaker" section above.

| ID | Scenario | Expected behavior |
| --- | --- | --- |
| B1 | Same item edited on both sides (`wd ∧ kd`) | Apply Keep's value to wiki; advance `synced_fp` to keep_fp; subsequent push diff is empty. |
| B2 | Wiki edit while Keep at synced baseline (`wd ∧ ¬kd`) | No apply; push wiki's value to Keep. (Same as W2/W3/etc — listed here for symmetry.) |

> **Retired:** former `B3` (equal-timestamp wiki-wins) is removed. Under
> fingerprint semantics there are no timestamps to be equal; the
> divergence axes `(wd, kd)` exhaust the conflict space and `(wd ∧ kd)`
> is unconditionally "Keep wins" until configurability ships.

## S — id_map staleness

| ID | Scenario | Expected behavior |
| --- | --- | --- |
| S1 | id_map points at serverID Keep no longer has (item gone in Keep) | Drop the stale entry; do not attempt soft-delete (Keep 500s on it). |
| S2 | id_map empty, wiki and Keep have matching-text items | Adopt-by-text on first sync to seed the map without duplicating; seed `synced_fp` from current content. |
| S3 | id_map has uid that is no longer in wiki, but is on Keep | Soft-delete on Keep (W7-style); drop from id_map. |

## L — labels

| ID | Scenario | Expected behavior |
| --- | --- | --- |
| L1 | Page has tags, none exist as Keep labels yet | Push label CRUD entries + LIST node referencing new labelIds. |
| L2 | Page has tags, all exist as Keep labels | LIST node push with referencing labelIds only; no label CRUD. |
| L3 | Page tags removed, LIST has labelIds | LIST push with empty labelIds. |

## E — edge / failure modes

| ID | Scenario | Expected behavior |
| --- | --- | --- |
| E1 | Multi-item update where every item is byte-identical | Skip those items (Keep 500s on no-op multi-item updates). |
| E2 | Fresh bind: KeepNoteID == "" | Bundled CreateListWithItems; populate binding from response. |
| E3 | `forceFullResync` returned | TODO: full reconcile (currently logged via #70). |
| E4 | `truncated` returned | Bump `TruncatedTickStreak`; do not advance `KeepCursor`; refuse hard-delete inferences. Two-condition escape hatch: chronic truncation without `synced_fp` progress drops the cursor to force a full resync. |
| E5 | `upgradeRecommended` returned | TODO: surface to UI (currently logged via #70). |
| E6 | Unmigrated binding (`MigratedFingerprints == false`) | Skip pull/push entirely with one-shot info log; eager migration job rebaselines `synced_fp` against current Keep state. |
| E7 | Push partial failure (per-node status) | Advance `synced_fp` only for the success entries; bump `PushFailureCount` and set `LastFailureCode`/`NextAttemptAt` on failures; dead-letter after threshold. |
| E8 | Dead-lettered item | Skipped from the outbound diff until a wiki-side re-edit resets `PushFailureCount`. |

---

## Full combinatoric enumeration

State variables per item:

- **W** ∈ {absent, present}
- **K** ∈ {absent, alive, trashed, deleted}
- **M** (id_map) ∈ {none, correct, stale}
- **wd** (`wiki_diverged`) ∈ {false, true} — meaningful only when W=present and id_map has a baseline (M=correct)
- **kd** (`keep_diverged`) ∈ {false, true} — meaningful only when K=alive and id_map has a baseline (M=correct)

22 illegal-pruned (W,K,M) cells:

| Cell | W | K | M | Expected | Test |
| --- | --- | --- | --- | --- | --- |
| C-01 | absent | absent | none | no-op (item doesn't exist anywhere) | implicit |
| C-02 | absent | absent | stale | drop stale id_map entry; no push | S1 |
| C-03 | absent | alive | none | inbound add OR adopt by wiki text-match | K1a/K1b |
| C-04 | absent | alive | correct | wiki removed → soft-delete on Keep | W7 |
| C-05 | absent | alive | stale | drop stale (rare; Keep has it under correct sid) | C-05 |
| C-06 | absent | trashed | none | no-op | implicit |
| C-07 | absent | trashed | correct | wiki already removed; drop id_map; no push | C-07 |
| C-08 | absent | trashed | stale | drop stale | implicit (S1) |
| C-09 | absent | deleted | none | no-op | implicit |
| C-10 | absent | deleted | correct | wiki already removed; drop id_map | C-10 |
| C-11 | absent | deleted | stale | drop stale | implicit |
| C-12 | present | absent | none | push as fresh | W1 |
| C-13 | present | absent | stale | drop stale + push as fresh | C-13 |
| C-14 | present | alive | none | adopt-by-text OR add (depends on match) | S2 / K1b |
| C-15 | present | alive | correct | divergence reconciliation (drives wd/kd sub-matrix) | W0/W2-6/K2/K3/B1/15e |
| C-16 | present | alive | stale | drop stale; treat as new fresh (id_map gap) | C-16 |
| C-17 | present | trashed | none | unusual; ignore Keep trash signal | C-17 |
| C-18 | present | trashed | correct | apply delete to wiki | K4 |
| C-19 | present | trashed | stale | apply delete + drop stale | C-19 |
| C-20 | present | deleted | none | unusual; ignore | C-20 |
| C-21 | present | deleted | correct | apply delete to wiki | K5 |
| C-22 | present | deleted | stale | apply delete + drop stale | implicit (K5) |

Symmetric to C-04, the W∧¬K cell splits on `wd`:

| Cell | W | K | M | wd | Expected | Test |
| --- | --- | --- | --- | --- | --- | --- |
| C-04a | present | absent | correct | false | apply Keep hard-delete to wiki; drop id_map (paired item is gone in Keep) | `keep_hard_delete_propagates_after_no_op_pull` |
| C-04b | present | absent | correct | true | wiki edited concurrently with Keep delete; push wiki as fresh (re-create) | `wiki_edit_concurrent_with_keep_idle_does_not_block_keep_hard_delete_of_other_item` (negative-side coverage) |

(C-04a/C-04b only fire on a complete, non-truncated pull; a truncated
pull suppresses hard-delete inference — see `K4-hard-truncated`.)

C-15 sub-matrix (`wd × kd`):

| Sub | wd | kd | Expected | Test |
| --- | --- | --- | --- | --- |
| 15a | false | false | no-op | W0 |
| 15b | true | false | push wiki | W2/W3/W4/W5/W6/B2/15e |
| 15c | false | true | apply Keep | K2/K3 |
| 15d | true | true | "Keep wins": apply Keep, discard wiki edit; advance synced_fp | B1 |

## Test coverage map

| ID | Unit test | keep-debug verification | Wiki smoke |
| --- | --- | --- | --- |
| W0 | `W0 — no-op tick` | (implicit — every other test) | manual idle |
| W1 | `W1 — wiki adds a new item` | `create-and-push` | type new item |
| W2 | `W2 — wiki toggles an item's checked state` | `update-item --checked` | check checkbox |
| W3 | `W3 — wiki text edit` | `update-item --text` | edit text |
| W4 | `W4 — wiki adds tags to an item` | (covered by text edit on encoded form) | add hashtag |
| W5 | `W5 — wiki adds a description` | (covered by text edit on encoded form) | add description |
| W6 | `W6 — wiki sort order changed` | `update-item --sort-value` | drag reorder |
| W7 | `W7 — wiki delete (item removed from wiki)` | `trash-one` | x out an item |
| W8 | `L1 — page tags new to Keep, push label CRUD + LIST node` (page-tag side) | `dump-items` after L push | edit page tags |
| K1a | `K1a — new item from Keep (no text-match in wiki) → AddItemForSync` | (manual: add via Keep web) | observe wiki add |
| K1b | `K1b — new item from Keep with text-match → adopt, do not duplicate` | (covered by stale id_map test) | observe no dup |
| K2 | `K2 — Keep toggles checked, wiki is older` / `K2-realworld — recent wiki edit + later Keep check toggle` | (manual: check via Keep web) | observe wiki check |
| K3 | `K3 — Keep text edit (inbound apply)` | (manual: edit via Keep web) | observe wiki text |
| K4 | `K4 — Keep marks an item trashed` | (manual) | observe wiki delete |
| K4-hard | `K4-hard — Keep hard-deletes (item missing from pull)` / `keep_hard_delete_propagates_after_no_op_pull` | n/a | observe wiki delete |
| K4-hard-wiki-only | `K4-hard-wiki-only — wiki-only item not in id_map should never be deleted by Keep absence` | n/a | n/a |
| K4-hard-bogus-pull | `K4-hard-bogus-pull — pull missing all expected items, refuse to delete` | n/a | n/a |
| K4-hard-truncated | `K4-hard-truncated — pull truncated, item missing is ambiguous` | n/a | n/a |
| K5 | `K5 — Keep marks an item deleted` | (manual) | observe wiki delete |
| B1 | `B1 — both edited, Keep newer` (will be renamed to `B1 — both edited, Keep wins`) | n/a | manual (rare) |
| B2 | `B2 — wiki edit while Keep at synced baseline` / `15e — wiki edit only, Keep at synced baseline` | n/a | manual (rare) |
| S1 | `S1 — stale id_map entry, Keep already deleted item` | covered by W7 + manual | n/a |
| S2 | `S2 — empty id_map, adopt by text` | covered by `dump-items` | rebind smoke |
| S3 | `S3 stale uid push delete` | (covered by W7) | delete-then-tick |
| L1 | `L1 — page tags new to Keep, push label CRUD + LIST node` / `L1b — page uses inline #hashtag content, push as labels` | n/a (no label CLI yet) | tag the page, tick |
| L2 | `L2 — page tag has existing Keep label, reuse mainID` | n/a | repeat-tag tick |
| L3 | `L3 clear labelIds` | n/a | untag page |
| E1 | `E1 — multi-item byte-identical content skip` | `dump-items` | n/a |
| E2 | `E2 — fresh bind, no KeepNoteID yet` | `create-and-push` | first-bind smoke |
| E4 | `truncation_streak_increments_on_truncated_pull` / `truncation_streak_resets_on_non_truncated_pull` / `chronic_truncation_with_progress_does_not_force_resync` / `chronic_truncation_without_progress_forces_full_resync` | `verify-cursor-monotonic` | n/a |
| E6 | `SyncToKeep — un-migrated binding gate` (sync-side gate) + `KeepBridgeFingerprintMigrationJob — silent rebaseline` / `… — Keep-wins divergence` / `… — drops entries Keep no longer has` / `… — idempotent on already-migrated` / `… — failure leaves binding un-migrated` / `… — profile mutex serialization` (migration-side) | `dump-write-results` | manual rebaseline |
| E7 | `push_partial_failure_does_not_advance_synced_fp_for_failed_item` / `push_failure_increments_count_and_does_not_advance_synced_fp` / `push_failure_with_no_response_status_uses_no_response_status_code` / `push_success_resets_failure_count_and_clears_failure_code` / `next_attempt_at_skips_recently_failed_item` / `next_attempt_at_advances_after_failure` / `wiki_side_re_edit_resets_push_failure_count` | `dump-write-results` | n/a |
| E8 | `dead_lettered_item_is_skipped_in_outbound_diff` | n/a | n/a |
| 15a | `W0 — no-op tick` | n/a | n/a |
| 15b | `outbound_pushes_when_wiki_diverges_from_synced` / `outbound_advances_synced_fp_after_successful_push` / `outbound_advances_synced_fp_for_fresh_items_after_push` | n/a | n/a |
| 15c | `K3 — Keep text edit (inbound apply)` | n/a | n/a |
| 15d | `B1 — both edited, Keep newer` | n/a | manual (rare) |

### Cursor and baseline coverage (cross-cutting)

| Capability | Test |
| --- | --- |
| KeepCursor passed as TargetVersion | `cursor_passed_as_target_version_on_pull` |
| KeepCursor advances after pull | `cursor_advances_after_successful_pull` |
| KeepCursor advances after push | `cursor_advances_after_successful_push` |
| KeepCursor does not advance on truncated pull | `cursor_does_not_advance_when_pull_is_truncated` |
| KeepCursor advances on empty incremental pull | `cursor_advances_on_empty_incremental_pull` |
| Outbound skips items at synced baseline | `outbound_skips_items_at_synced_baseline` |
| `LastObservedWiki*` written end-of-tick | `last_observed_wiki_fields_written_at_end_of_tick` |
| Add-then-delete within one tick | `keep_add_then_delete_within_one_tick` |
| Add-then-delete across two ticks | `keep_add_then_delete_across_two_ticks` |
| Concurrent wiki edit does not block Keep hard-delete of unrelated item | `wiki_edit_concurrent_with_keep_idle_does_not_block_keep_hard_delete_of_other_item` |

### Coverage gaps

- **15d (`wd ∧ kd` "Keep wins")** — currently exercised by the legacy
  `B1 — both edited, Keep newer` test. The test name still references
  "newer" (timestamp framing); the assertions already match the
  fingerprint-driven outcome. Renaming the test to `B1 — both edited,
  Keep wins` is tracked under the broader B1 coverage refresh.
- **C-04b (`W∧¬K∧correct∧wd` — wiki edit concurrent with Keep delete,
  push wiki as fresh)** — partially covered by
  `wiki_edit_concurrent_with_keep_idle_does_not_block_keep_hard_delete_of_other_item`,
  which pins the negative side (the unrelated wiki edit must not
  suppress Keep's hard-delete on a different item). A direct positive
  test for "wiki edited the same item Keep deleted → re-create on Keep"
  is not yet present; flag for follow-up.
