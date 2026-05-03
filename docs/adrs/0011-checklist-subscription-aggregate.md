# ADR-0011: The `ChecklistSubscription` Aggregate

## Status

Accepted

## Date

2026-05-02

## Context

The wiki integrates with multiple remote checklist systems — Google Keep landed first (#982), Google Tasks is being added now, iCloud Reminders is planned. Each integration syncs a single wiki checklist (identified by `(page, list_name)`) to a single remote list. We call the binding between a wiki checklist and a remote list a **Subscription**, and each integration a **Connector**.

A consistency model is needed. Without one, the obvious failure modes are:

- **Multi-binding incoherence.** If a checklist could be subscribed via both Keep and Tasks simultaneously, a single user-side toggle would write to two remotes, each remote's webhook would fire back into the wiki, and round-trip semantics become "which connector wins?" — a question with no good answer.
- **Concurrent subscribe races.** Two `Subscribe` calls hitting the same `(page, list_name)` from different connectors must not both win.
- **Crash recovery.** Profiles persist subscriptions; an in-memory lookup table holds derived bindings. If the two diverge (write profile, crash before lease take), the next boot must reconcile without lost data and without phantom owners.

## Decision

The unit of consistency is the **`ChecklistSubscription` aggregate** (Evans, *Domain-Driven Design*).

### Root identity

`(page, list_name)`. Globally unique. There is at most one active Subscription per `(page, list_name)` across **all** connectors and **all** profiles.

### Entities

- **`Subscription`** — persisted on a user profile page at `wiki.connectors.<kind>.subscriptions[]`. Contains `page`, `list_name`, `remote_list_handle`, connector-specific peer fields (e.g. `item_id_map`), and `state` (active / paused / dead-lettered). This is the authoritative record.
- **`Lease`** — in-memory entry in `LeaseTable`, keyed by `(page, list_name)`, value `(connector_kind, profile_id)`. Derived from Subscriptions; rebuilt at boot via fan-out scan over all profiles.

### Invariants

1. At most one `Subscription` per `(page, list_name)` across all connectors and all profiles.
2. The `Lease` table is a pure derivation of the union of all profile `Subscription` records. It is never the source of truth.

### Operation contracts

- **Subscribe** is strongly consistent: acquire per-checklist mutex on `(page, list_name)` → fan-out re-read all profiles for any matching Subscription → if none exists, write profile + take lease → release mutex. The fan-out re-read **inside the mutex** is the linearizability guarantee. The `LeaseTable` is consulted only for fast `LookupOwner` reads outside the critical section.
- **Unsubscribe** is strongly consistent: acquire mutex → write profile (remove subscription) → release lease → release mutex.
- **Boot rebuild** is eventually consistent: fan-out scan over all profiles repopulates `LeaseTable`. During the rebuild window, `ConnectorService` RPCs and the `<connector-subscribe-button>` block on `LeaseTable.WaitReady(ctx)`; the existing `/healthz` readiness check extends to include lease-rebuild completion.
- **Crash recovery:** if a profile write succeeds but the lease take crashes before completing, the next boot's fan-out scan reconstructs the lease. No data loss; no manual reconciliation.

### Single-process assumption

This aggregate's consistency guarantees rely on a single wiki process holding the per-checklist mutex. Multi-process coordination (ZooKeeper, etcd, distributed locks) is **explicitly out of scope** for this household-scale, self-hosted deployment.

## Consequences

### Positive

- Cross-connector exclusivity is enforced by construction. "Which connector owns this checklist?" has exactly one answer at all times.
- Crash recovery is automatic: lease is derived state, profiles are authoritative.
- The naming makes the design legible. Reviewers see "ChecklistSubscription aggregate" and know to look for the root, the entities, and the invariants.

### Negative

- Boot-time fan-out scan over all profiles. At household scale this is single-digit seconds; at larger scale it becomes a real cost. If the wiki ever serves more than household-scale, this rebuild needs an index.
- Multi-process operation is foreclosed without revisiting this ADR.

### Neutral

- Subscribe latency includes the fan-out re-read inside the mutex. At household scale (a few profiles, a handful of subscriptions each) this is negligible.

## Alternatives considered

- **Allow multi-binding.** Rejected. Round-trip incoherence has no good resolution; "last writer wins" silently loses user edits.
- **Per-checklist binding pointer co-located on the checklist itself.** Rejected. Splits the binding's authoritative state across two storage locations (the checklist page and the profile page), since binding peer data like `item_id_map` already lives on the profile alongside auth credentials. One subtree per binding, one location.
- **External coordinator (ZooKeeper, etcd) for multi-process.** Rejected as out of scope. The wiki is a single-process binary; if that ever changes, this ADR is revisited.

## References

- ADR-0009: Reserved frontmatter namespaces and dedicated services.
- ADR-0010: The `wiki.*` namespace.
- ADR-0012: Connector abstraction in `internal/connectors/`.
- Plan: `now-that-we-landed-groovy-pizza.md` (Single-Subscription invariant section).
- Evans, *Domain-Driven Design* — Aggregate pattern.
- Helland, *Life beyond Distributed Transactions* — single-process consistency boundaries.
