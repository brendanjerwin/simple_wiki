# Sync interaction matrix

This is the canonical inventory of every possible delta between wiki state
and Keep state at sync time, what the bridge should do for each, and which
test exercise covers it.

Three sources of truth at every tick:

- **Wiki**: items in `wiki.checklists.<list>.items[]` with `Uid`, `Text`,
  `Checked`, `Tags`, `Description`, `SortOrder`, `UpdatedAt`.
- **Keep**: LIST_ITEM nodes returned by `Changes.list` with `ServerID`,
  `Text`, `Checked`, `SortValue`, `Timestamps.{Updated, UserEdited,
  Trashed, Deleted}`, `BaseVersion`.
- **Binding id_map**: `wiki_uid → keep_server_id` per binding,
  persisted on the user's profile.

## Notation

| Tag  | Meaning                                                        |
| ---- | -------------------------------------------------------------- |
| W*n* | Wiki-side delta                                                |
| K*n* | Keep-side delta                                                |
| B*n* | Both sides changed simultaneously (conflict)                   |
| S*n* | id_map staleness (entries that no longer correspond to truth)  |
| L*n* | Label / tag delta                                              |
| E*n* | Edge / failure-mode delta                                      |

## W — wiki-side changes

| ID  | Scenario                                       | Expected behavior                                                                       |
| --- | ---------------------------------------------- | --------------------------------------------------------------------------------------- |
| W0  | No wiki changes, no Keep changes               | No push, no apply.                                                                      |
| W1  | New wiki item added (no id_map entry)          | Push as fresh; pick up server_id from response, write into id_map.                      |
| W2  | Wiki item checked toggled                      | Push update; preserve id, parentServerId, baseVersion.                                  |
| W3  | Wiki item text edited                          | Push update; original client_id preserved, serverId distinct.                           |
| W4  | Wiki item tags edited                          | Push with re-encoded text head (`#tag1 #tag2 — text`).                                  |
| W5  | Wiki item description edited                   | Push with re-encoded text body (`text\n— description`).                                 |
| W6  | Wiki item sort_order changed                   | Push with new SortValue.                                                                |
| W7  | Wiki item deleted (removed from items list)    | Push soft-delete (Deleted=now, NOT Trashed); drop from id_map after success.            |
| W8  | Wiki page tags changed                         | Re-resolve labels; LIST node push with updated labelIds.                                |

## K — Keep-side changes

| ID  | Scenario                                       | Expected behavior                                                                       |
| --- | ---------------------------------------------- | --------------------------------------------------------------------------------------- |
| K0  | Keep returns empty / no changes                | No apply.                                                                               |
| K1a | New Keep item, no text-match in wiki           | AddItemForSync; record uid → serverID in id_map.                                        |
| K1b | New Keep item, text-match in wiki              | Adopt: record existing-wiki-uid → serverID; do NOT duplicate.                           |
| K2  | Keep item checked toggled (newer than wiki)    | UpdateItemForSync with new Checked.                                                     |
| K3  | Keep item text edited (newer than wiki)        | UpdateItemForSync with new Text (parsed back into text/tags/description).               |
| K4  | Keep item Trashed (Trashed timestamp non-zero) | DeleteItemForSync; remove from id_map.                                                  |
| K5  | Keep item Deleted (Deleted timestamp non-zero) | DeleteItemForSync; remove from id_map.                                                  |
| K6  | Keep returns epoch-sentinel `Updated` only     | Use `UserEdited` for freshness gate; epoch alone is not "no edit".                      |
| K7  | Keep returns exact-zero `Updated` AND `UserEdited` | Treat as no Keep edit: do not pull, do not gate-block push.                          |

## B — both-side conflicts

For B-class scenarios, the inbound apply runs first; then content-equality
in the push diff means the Keep edit "wins" for that round, and the wiki
edit gets re-applied on the next round once UpdatedAt advances past Keep's.
Tested via timestamp ordering, not race conditions.

| ID  | Scenario                                                      | Expected behavior                                                                  |
| --- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| B1  | Same item edited on both sides, Keep newer                    | Apply Keep's value to wiki; subsequent push diff is empty (content-equality).      |
| B2  | Same item edited on both sides, wiki newer                    | Skip apply (gate); push wiki's value to Keep.                                      |
| B3  | Same item edited on both sides, equal timestamps              | Wiki side wins (gate is strict After).                                             |

## S — id_map staleness

| ID  | Scenario                                                          | Expected behavior                                                          |
| --- | ----------------------------------------------------------------- | -------------------------------------------------------------------------- |
| S1  | id_map points at serverID Keep no longer has (item gone in Keep)  | Drop the stale entry; do not attempt soft-delete (Keep 500s on it).        |
| S2  | id_map empty, wiki and Keep have matching-text items              | Adopt-by-text on first sync to seed the map without duplicating.           |
| S3  | id_map has uid that is no longer in wiki, but is on Keep          | Soft-delete on Keep (W7-style); drop from id_map.                          |

## L — labels

| ID  | Scenario                                              | Expected behavior                                                  |
| --- | ----------------------------------------------------- | ------------------------------------------------------------------ |
| L1  | Page has tags, none exist as Keep labels yet          | Push label CRUD entries + LIST node referencing new labelIds.       |
| L2  | Page has tags, all exist as Keep labels               | LIST node push with referencing labelIds only; no label CRUD.       |
| L3  | Page tags removed, LIST has labelIds                  | LIST push with empty labelIds.                                      |

## E — edge / failure modes

| ID  | Scenario                                                        | Expected behavior                                                     |
| --- | --------------------------------------------------------------- | --------------------------------------------------------------------- |
| E1  | Multi-item update where every item is byte-identical            | Skip those items (Keep 500s on no-op multi-item updates).             |
| E2  | Fresh bind: KeepNoteID == ""                                    | Bundled CreateListWithItems; populate binding from response.          |
| E3  | `forceFullResync` returned                                      | TODO: full reconcile (currently logged via #70).                      |
| E4  | `truncated` returned                                            | TODO: paginate via cursor (currently logged via #70).                 |
| E5  | `upgradeRecommended` returned                                   | TODO: surface to UI (currently logged via #70).                       |

---

## Full combinatoric enumeration

State variables per item:
- **W** ∈ {absent, present}
- **K** ∈ {absent, alive, trashed, deleted}
- **M** (id_map) ∈ {none, correct, stale}
- **C** (content) ∈ {same, diff} — meaningful only when W=present, K=alive
- **T** (time) ∈ {wiki_newer, keep_upd_newer, keep_ue_only_newer, equal, zero, wiki_nil}
  — meaningful only when C=diff

22 illegal-pruned (W,K,M) cells:

| Cell  | W       | K       | M       | Expected                                      | Test         |
|-------|---------|---------|---------|-----------------------------------------------|--------------|
| C-01  | absent  | absent  | none    | no-op (item doesn't exist anywhere)           | implicit     |
| C-02  | absent  | absent  | stale   | drop stale id_map entry; no push              | S1           |
| C-03  | absent  | alive   | none    | inbound add OR adopt by wiki text-match       | K1a/K1b      |
| C-04  | absent  | alive   | correct | wiki removed → soft-delete on Keep            | W7           |
| C-05  | absent  | alive   | stale   | drop stale (rare; Keep has it under correct sid)  | C-05  |
| C-06  | absent  | trashed | none    | no-op                                         | implicit     |
| C-07  | absent  | trashed | correct | wiki already removed; drop id_map; no push    | C-07         |
| C-08  | absent  | trashed | stale   | drop stale                                    | implicit (S1) |
| C-09  | absent  | deleted | none    | no-op                                         | implicit     |
| C-10  | absent  | deleted | correct | wiki already removed; drop id_map             | C-10         |
| C-11  | absent  | deleted | stale   | drop stale                                    | implicit     |
| C-12  | present | absent  | none    | push as fresh                                 | W1           |
| C-13  | present | absent  | stale   | drop stale + push as fresh                    | C-13         |
| C-14  | present | alive   | none    | adopt-by-text OR add (depends on match)       | S2 / K1b     |
| C-15  | present | alive   | correct | diff & reconcile (drives C/T sub-matrix)      | W0/W2-6/K2-7/B1-2 |
| C-16  | present | alive   | stale   | drop stale; treat as new fresh (id_map gap)   | C-16         |
| C-17  | present | trashed | none    | unusual; ignore Keep trash signal             | C-17         |
| C-18  | present | trashed | correct | apply delete to wiki                          | K4           |
| C-19  | present | trashed | stale   | apply delete + drop stale                     | C-19         |
| C-20  | present | deleted | none    | unusual; ignore                               | C-20         |
| C-21  | present | deleted | correct | apply delete to wiki                          | K5           |
| C-22  | present | deleted | stale   | apply delete + drop stale                     | implicit (K5) |

C-15 sub-matrix (content × time):

| Sub | C    | T                       | Expected                                       | Test          |
|-----|------|-------------------------|------------------------------------------------|---------------|
| 15a | same | -                       | no-op (content equality skip)                  | W0            |
| 15b | diff | wiki_newer              | push wiki                                      | W2/W3/W4/W5/W6/B2 |
| 15c | diff | keep_upd_newer          | apply Keep                                     | K2/K3         |
| 15d | diff | keep_ue_only_newer      | apply Keep (latestKeepTimestamp picks UE)      | K6            |
| 15e | diff | equal                   | wiki wins (gate is strict After)               | 15e           |
| 15f | diff | zero (both)             | no apply, no push (no signal)                  | K7            |
| 15g | diff | wiki_nil                | apply Keep (nil UpdatedAt → always pull)       | 15g           |
| 15h | diff | keep_upd_only_newer     | apply Keep (UE stale, Updated fresh)           | K2-realworld  |

## Test coverage map

| ID  | Unit test | keep-debug verification | Wiki smoke |
| --- | --- | --- | --- |
| W0  | `no-op tick` | (implicit — every other test) | manual idle |
| W1  | `W1 wiki add` | `create-and-push` | type new item |
| W2  | `W2 wiki check toggle` | `update-item --checked` | check checkbox |
| W3  | `W3 wiki text edit` | `update-item --text` | edit text |
| W4  | `W4 wiki tags edit` | (covered by text edit on encoded form) | add hashtag |
| W5  | `W5 wiki description edit` | (covered by text edit on encoded form) | add description |
| W6  | `W6 wiki sort_order changed` | `update-item --sort-value` | drag reorder |
| W7  | `W7 wiki delete` | `trash-one` | x out an item |
| W8  | `W8 page tags changed` | `dump-items` after L push | edit page tags |
| K1a | `K1 add` | (manual: add via Keep web) | observe wiki add |
| K1b | `K1 adopt` | (covered by stale id_map test) | observe no dup |
| K2  | `K2 keep check toggle` | (manual: check via Keep web) | observe wiki check |
| K3  | `K3 keep text edit` | (manual: edit via Keep web) | observe wiki text |
| K4  | `K4 keep trashed` | (manual) | observe wiki delete |
| K5  | `K5 keep deleted` | (manual) | observe wiki delete |
| K6  | `K6 epoch sentinel updated` | covered by RFC3339 reference | n/a |
| K7  | `K7 zero-zero timestamps` | n/a | n/a |
| B1  | `B1 keep newer wins` | n/a | manual (rare) |
| B2  | `B2 wiki newer wins` | n/a | manual (rare) |
| B3  | `B3 equal-timestamp wiki wins` | n/a | manual (rare) |
| S1  | `S1 stale id_map drop` | covered by W7 + manual | n/a |
| S2  | `S2 adopt by text on empty id_map` | covered by `dump-items` | rebind smoke |
| S3  | `S3 stale uid push delete` | (covered by W7) | delete-then-tick |
| L1  | `L1 push label CRUD` | n/a (no label CLI yet) | tag the page, tick |
| L2  | `L2 reuse existing label` | n/a | repeat-tag tick |
| L3  | `L3 clear labelIds` | n/a | untag page |
| E1  | `E1 byte-identical multi-update skip` | `dump-items` | n/a |
| E2  | `E2 fresh bind bootstrap` | `create-and-push` | first-bind smoke |
