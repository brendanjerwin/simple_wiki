# Google Keep Regression Verification Runbook

This runbook is the live-API smoke test for the Google Keep bridge after the
Tasks-bridge PR substantially restructured the Keep code:

- Phase 1a lifted `internal/keep/{bridge,protocol}` into
  `internal/connectors/google_keep/{gateway,translator,sync}`.
- Phase 1a-extract pulled translation functions into the `translator` package.
- Phase 1b renamed `KeepConnectorService` → `ConnectorService` (with
  `connector_kind` enum) and remapped checklist-collision errors from
  `FailedPrecondition` → `AlreadyExists`.
- Agent A (in flight) is renaming Keep's internal `Binding` → `Subscription`,
  wiring `Subscribe` through `LeaseTable.WithChecklistLock` + `LookupOwner`,
  and adding a one-time profile fan-out scan at boot.

Unit tests are green. **No live end-to-end test has been run against the real
Keep API since Phase 1a.** This runbook closes that gap.

## Required environment variables

`cmd/keep-debug` reads three env vars (see `cmd/keep-debug/main.go:44–50`):

```sh
export KEEP_EMAIL="brendanjerwin@gmail.com"
export KEEP_MASTER_TOKEN="…"   # gpsoauth master token (NOT an oauth_token)
export KEEP_DEVICE_ID="…"      # 16-hex-char android_id used during the exchange
```

If any are unset, the binary exits with `set KEEP_EMAIL,
KEEP_MASTER_TOKEN, KEEP_DEVICE_ID` on stderr.

### How to capture them

The wiki normally captures the master token via the in-browser oauth_token
exchange (see `internal/syspage/embedded/help_google_keep.md`). For this
out-of-band CLI use, the simplest path is:

1. Pull your existing master token from your wiki profile page
   frontmatter under `wiki.connectors.google_keep.master_token` (it was
   persisted at Connect time).
2. Pull the device ID from the same submap
   (`wiki.connectors.google_keep.android_device_id`).

If those aren't on the profile yet (e.g., this is a fresh deployment), you'll
need to run the in-browser Connect flow first. Don't fabricate a fresh
master_token via password login — `master_login` is gated by Google for almost
every account; only `exchange_token` from a captured oauth cookie still works.

**Do not commit or paste these values anywhere.** A master token is
credential-equivalent to your Google password.

## The runbook

Run from the worktree root. Each step has a copy-pasteable command, expected
output, and what failure would mean.

### Step 1 — `list`: enumerate Keep notes

```sh
go run ./cmd/keep-debug -cmd=list
```

**Expected output** (`cmd/keep-debug/main.go:737–768`):

```
✓ bearer obtained, len: <some int>
✓ Changes returned <N> nodes; toVersion: <opaque-cursor> truncated: false
by type: map[NOTE:… LIST:… LIST_ITEM:… …]
ALL LIST nodes (with state):
  [alive] serverID=<id> title="<title>"
  [trashed 2024-01-15] serverID=<id> title="<old>"
  …
```

**What success means:** auth handshake (`ExchangeMasterTokenForBearer`) works;
the `Changes` RPC returns nodes; the response decodes against the gateway's
node types after the `internal/keep/protocol` → `internal/connectors/
google_keep/gateway` lift.

**What failure would mean:**
- `Stage 2 failed` → master token bad/expired or device ID malformed (gateway
  authenticator is in `internal/connectors/google_keep/gateway/gpsoauth.go`).
  Not a regression — the credential just needs refreshing.
- `Changes failed: …` with HTTP 4xx → token-to-bearer flow regressed in the
  lift; investigate `gateway.NewKeepClient` wiring.
- Decode error / panic on response → `gateway/types.go` schema drift; the lift
  changed the unmarshalling shape. **This is a regression to flag.**

Cross-check: the `ALL LIST nodes` block should match what you see in the
Google Keep app. If a list is in the app but missing here, the pull pagination
or filter logic regressed.

### Step 2 — `create-and-push`: create a fresh Keep list

```sh
go run ./cmd/keep-debug -cmd=create-and-push -title="Wiki regression test $(date +%Y-%m-%d-%H%M)"
```

**Expected output** (`cmd/keep-debug/main.go:770–800`):

```
✓ bearer obtained, len: …
✓ CreateList returned serverID: <new-list-server-id>
✓ found new list in pull:
  {
    "id": "…",
    "serverId": "<new-list-server-id>",
    "type": "LIST",
    "title": "Wiki regression test …",
    …
  }
```

**Note the `serverID` value — you'll need it for steps 4 and 5.**

**Verify in Google Keep app:** open Google Keep on your phone or
keep.google.com — the new (empty) list should appear within a few seconds.

**What failure would mean:**
- `CreateList failed` → the create-list path in `gateway.KeepClient`
  regressed in the lift.
- Created but `NOT visible in next pull` → pull/cursor handling regression
  (the create succeeded but a follow-up `Changes` call didn't see it).

### Step 3 — `create-with-items`: create-then-bulk-push (the bundled wire shape)

```sh
go run ./cmd/keep-debug -cmd=create-with-items \
  -title="Wiki regression test items $(date +%H%M)" \
  -items="Eggs,Milk,Bread"
```

**Expected output** (`cmd/keep-debug/main.go:717–735`):

```
✓ bearer obtained, len: …
✓ list created: serverID=<new-list-server-id>
✓ 3 items pushed; server-assigned IDs:
  [0] "Eggs" -> <item-server-id-0>
  [1] "Milk" -> <item-server-id-1>
  [2] "Bread" -> <item-server-id-2>
```

**Note the list `serverID` — you'll use it for step 4.**

**Verify in Google Keep app:** the new list should appear with all three items
visible, in the order Eggs/Milk/Bread (sort values 3000/2000/1000).

**What failure would mean:** This is the bundled `CreateListWithItems` shape
that exists specifically because Keep returns 500 on a two-step create-list-
then-push-items. If this regresses (after the translator extraction), `Bind`
on a fresh checklist with seed items will fail at the gateway level. Investigate
`gateway.KeepClient.CreateListWithItems`.

### Step 4 — `push-item-to-existing`: append to an existing list

Use the `serverID` from step 3:

```sh
go run ./cmd/keep-debug -cmd=push-item-to-existing \
  -parent-id=<serverID-from-step-3> \
  -item-text="Late add — regression check"
```

**Expected output** (`cmd/keep-debug/main.go:645–715`):

```
✓ bearer obtained, len: …
✓ pull got toVersion=<cursor> (<N> nodes)
✓ item created on existing list: <client-id> -> <new-item-server-id>
```

**Verify in Google Keep app:** the item appears in the list from step 3.

**What failure would mean:**
- `pull failed` → pull-then-push pattern broken (Keep requires a fresh
  `TargetVersion` on push; the gateway must forward it correctly).
- `push-item-to-existing failed: … 500 Unknown Error` → almost always means
  `parent_server_id` was dropped. The `Node.ParentServerID` field must
  survive JSON marshalling. **This is a high-signal regression** and
  reproduces the original bug that motivated `verify-listitem-update-shape`.
- `item NOT echoed back` → translator's request shape doesn't match Keep's
  expectations after the Phase 1a-extract; check `translator/mapping.go`
  against `internal/connectors/google_keep/sync/MATRIX.md`.

### Step 5 — `verify-cursor-monotonic`: pagination invariant

```sh
go run ./cmd/keep-debug -cmd=verify-cursor-monotonic
```

**Expected output** (`cmd/keep-debug/main.go:569–621`):

```
✓ bearer obtained, len: …
pull[0] targetVersion="" -> toVersion="<cursor-0>" (nodes=…, truncated=false)
pull[1] targetVersion="<cursor-0>" -> toVersion="<cursor-1>" (nodes=0, truncated=false)
pull[2] targetVersion="<cursor-1>" -> toVersion="<cursor-2>" (nodes=0, truncated=false)
pull[3] targetVersion="<cursor-2>" -> toVersion="<cursor-3>" (nodes=0, truncated=false)
pull[4] targetVersion="<cursor-3>" -> toVersion="<cursor-4>" (nodes=0, truncated=false)
pull[5] targetVersion="<cursor-4>" -> toVersion="<cursor-5>" (nodes=0, truncated=false)
verified: 6 consecutive to_version values are lexicographically monotonic
```

**What success means:** the gateway's `Changes` RPC keeps cursor monotonicity
across consecutive pulls; the cron-tick + debouncer story stays sound after
the lift.

**What failure would mean:**
- `FAIL: lex order violated at pair […]` → cursor regression. The wiki's
  delta-pull loop will eventually re-deliver old changes or skip new ones.
  This breaks the at-least-once semantics for inbound sync.
- `pull N failed` mid-loop → transient API issue, retry once before
  flagging.

### Step 6 — Cleanup

In the Google Keep app, manually trash the two test lists created in steps 2
and 3. (The CLI's `trash-one` / `delete-many` commands work too, but manual
deletion via the UI is the fastest cleanup and confirms the trash flow from
the user side.)

## What this verifies

| Risk surface | Step that would catch it |
| --- | --- |
| Auth/bearer exchange after `internal/keep/bridge` → `gateway` lift | 1 |
| Response unmarshalling drift (gateway types.go) | 1 |
| `CreateList` path regression | 2 |
| Bundled `CreateListWithItems` regression (translator extraction) | 3 |
| Pull-then-push protocol after translator extraction | 4 |
| `ParentServerID` survives JSON marshalling | 4 |
| Cursor monotonicity across `Changes` calls | 5 |

## What this does NOT verify

This runbook is intentionally scoped to the **gateway + translator** layers —
the parts the Phase 1a/1a-extract refactor touched directly. It does not
exercise:

- The wiki-side `Bind` ceremony or `LeaseTable` integration (Agent A's
  in-flight work). Once Agent A lands, exercise this through the profile-page
  Subscribe button against a real checklist.
- The `Binding` → `Subscription` rename (also Agent A's in-flight work).
- The boot-rebuild fan-out scan (currently a no-op
  `leaseTable.SignalReady()` in `internal/bootstrap/server.go:845`; once Agent A
  wires the real scan, restart the wiki and watch logs for "lease table
  rebuild complete" or equivalent before declaring rebuild verified).
- The `FailedPrecondition` → `AlreadyExists` collision-error remapping (no
  frontend code branches on the error code today; both
  `connector-subscribe-button.ts` and `keep-connect.ts` only display
  `err.message`. Verify by attempting two Subscribe calls for the same
  checklist and confirming the user-visible error message is still readable).

For the post-Agent-A version of this runbook, append:

- **Bind via Subscribe button** to a real wiki checklist; verify the new
  Keep list appears in the Keep app and the profile-page subscription badge
  shows `✓ Synced with Google Keep list <title>`.
- **Cross-connector collision:** subscribe a checklist to Keep, then attempt
  to subscribe the same checklist to Tasks from another profile; expect
  `AlreadyExists` with the current owner named (matches plan step 14).
- **Boot-rebuild correctness:** restart the wiki with at least one Keep
  subscription on a profile; verify the lease table is repopulated before
  any Subscribe call returns.
