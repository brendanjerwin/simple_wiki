# ADR-0011: The `ChecklistBinding` Aggregate

## Status

Accepted (2026-05-02); revised 2026-05-04 (terminology: `Subscription` → `Binding`; `Subscribe`/`Unsubscribe` → `Bind`/`Unbind`).

## Date

2026-05-02 (original); 2026-05-04 (terminology unification)

## Context

The wiki integrates with multiple remote checklist systems — Google Keep landed first (#982), Google Tasks is being added now, iCloud Reminders is planned. Each integration syncs a single wiki checklist (identified by `(page, list_name)`) to a single remote list. We call the binding between a wiki checklist and a remote list a **Binding**, and each integration a **Connector**.

A consistency model is needed. Without one, the obvious failure modes are:

- **Multi-binding incoherence.** If a checklist could be bound via both Keep and Tasks simultaneously, a single user-side toggle would write to two remotes, each remote's webhook would fire back into the wiki, and round-trip semantics become "which connector wins?" — a question with no good answer.
- **Concurrent bind races.** Two `Bind` calls hitting the same `(page, list_name)` from different connectors must not both win.
- **Crash recovery.** Profiles persist bindings; an in-memory lookup table holds derived bindings. If the two diverge (write profile, crash before lease take), the next boot must reconcile without lost data and without phantom owners.

## Decision

The unit of consistency is the **`ChecklistBinding` aggregate** (Evans, *Domain-Driven Design*).

### Root identity

`(page, list_name)`. Globally unique. There is at most one active Binding per `(page, list_name)` across **all** connectors and **all** profiles.

### Entities

- **`Binding`** — persisted on a user profile page at `wiki.connectors.<kind>.bindings[]`. Contains `page`, `list_name`, `remote_handle`, the engine-owned cursor (`last_synced_seq`) and lifecycle state (`state`, `paused_reason`, `paused_at`, `bound_at`), plus the connector's per-binding bookkeeping in an opaque `adapter_state` subtree (e.g., Tasks's `item_id_map` and `item_etags`; Keep's per-item `ServerID`/`BaseVersion`/`ClientID`/`PushFailureCount`/`NextAttemptAt`). This is the authoritative record.
- **`Lease`** — in-memory entry in `LeaseTable`, keyed by `(page, list_name)`, value `(connector_kind, profile_id)`. Derived from Bindings; rebuilt at boot via fan-out scan over all profiles.

### Invariants

1. At most one `Binding` per `(page, list_name)` across all connectors and all profiles.
2. The `Lease` table is a pure derivation of the union of all profile `Binding` records. It is never the source of truth.

### Operation contracts

- **Bind** is strongly consistent: acquire per-checklist mutex on `(page, list_name)` → fan-out re-read all profiles for any matching Binding → if none exists, write profile + take lease → release mutex. The fan-out re-read **inside the mutex** is the linearizability guarantee. The `LeaseTable` is consulted only for fast `LookupOwner` reads outside the critical section.
- **Unbind** is strongly consistent: acquire mutex → write profile (remove binding) → release lease → release mutex.
- **Boot rebuild** is eventually consistent: fan-out scan over all profiles repopulates `LeaseTable`. During the rebuild window, `ConnectorService` RPCs and the `<connector-bind-button>` block on `LeaseTable.WaitReady(ctx)`; the existing `/healthz` readiness check extends to include lease-rebuild completion.
- **Crash recovery:** if a profile write succeeds but the lease take crashes before completing, the next boot's fan-out scan reconstructs the lease. No data loss; no manual reconciliation.

### Single-process assumption

This aggregate's consistency guarantees rely on a single wiki process holding the per-checklist mutex. Multi-process coordination (ZooKeeper, etcd, distributed locks) is **explicitly out of scope** for this household-scale, self-hosted deployment.

## Consequences

### Positive

- Cross-connector exclusivity is enforced by construction. "Which connector owns this checklist?" has exactly one answer at all times.
- Crash recovery is automatic: lease is derived state, profiles are authoritative.
- The naming makes the design legible. Reviewers see "ChecklistBinding aggregate" and know to look for the root, the entities, and the invariants.

### Negative

- Boot-time fan-out scan over all profiles. At household scale this is single-digit seconds; at larger scale it becomes a real cost. If the wiki ever serves more than household-scale, this rebuild needs an index.
- Multi-process operation is foreclosed without revisiting this ADR.

### Neutral

- Bind latency includes the fan-out re-read inside the mutex. At household scale (a few profiles, a handful of bindings each) this is negligible.

## Alternatives considered

- **Allow multi-binding.** Rejected. Round-trip incoherence has no good resolution; "last writer wins" silently loses user edits.
- **Per-checklist binding pointer co-located on the checklist itself.** Rejected. Splits the binding's authoritative state across two storage locations (the checklist page and the profile page), since binding peer data like `item_id_map` already lives on the profile alongside auth credentials. One subtree per binding, one location.
- **External coordinator (ZooKeeper, etcd) for multi-process.** Rejected as out of scope. The wiki is a single-process binary; if that ever changes, this ADR is revisited.

## Note on terminology (2026-05-04 revision)

The original (2026-05-02) ADR used **Subscription** as the aggregate name and **Subscribe**/**Unsubscribe** as the operation verbs. The 2026-05-04 SyncEngine extraction unified terminology to **Binding** / **Bind** / **Unbind** because:

- The data is genuinely a **binding** between a wiki checklist and a remote list (a paired association), not a subscription (which implies passive consumption like a feed).
- The codebase had drifted: Google Keep used `Bind`/`BindingKey` internally; Google Tasks used `Subscribe`/`Subscription`. The frontend used `<connector-subscribe-button>`. The frontmatter used `subscriptions[]`. This ADR was the primary source of the divergence; standardizing here drives the codebase rename in the same PR.

The Phase 6/7 work in plan `to-build-issue-998-warm-glacier.md` performs the sweep across types, proto, frontend, event constants, and frontmatter (with an eager migration that renames `wiki.connectors.<kind>.subscriptions[]` → `bindings[]` in existing profiles).

**Op-log self-source markers** (`connector:<kind>:apply`, `connector:<kind>:push_recovery`) are NOT renamed — they are causal-source identifiers per ADR-0015, and renaming them would invalidate cursor advance for in-flight bindings on existing checklists. The `<kind>` slug stays as `google_keep` / `google_tasks` / `icloud_reminders`.

## References

- ADR-0009: Reserved frontmatter namespaces and dedicated services.
- ADR-0010: The `wiki.*` namespace.
- ADR-0012: Connector abstraction in `internal/connectors/`.
- ADR-0015: Per-checklist operation log + engine-owned merge rule.
- Plan: `to-build-issue-998-warm-glacier.md` (Phase 6/7 verb rename + frontmatter migration).
- [`internal/connectors/MATRIX.md`](../../internal/connectors/MATRIX.md) — Phase 0 audit of pre-extraction implementations.
- Evans, *Domain-Driven Design* — Aggregate pattern.
- Helland, *Life beyond Distributed Transactions* — single-process consistency boundaries.
