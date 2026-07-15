# Issue 1116 — GPS track overlays & tag-based map layer controls

Extend the first-class wiki map (PR #1115) with a GPS **track** overlay type whose large
payload is stored as a content-addressed wiki file (reusing `FileStorageService`) while
frontmatter holds only a reference; a read-only `GetTrackGeometry` RPC parses GPX/GeoJSON
server-side into normalized, simplified segments that the `<wiki-map>` widget loads lazily
and draws as polylines. Add **tags** to markers, polygons, circles, and tracks driving a
native Leaflet layer control (with a virtual `untagged` group; all tags visible by
default, OR semantics for multi-tag overlays), and let tracks be attached both from the
widget (a tap-revealed tools panel mirroring `wiki-image`) and by agents via new MCP tools.

## Assets and how they bind

- **[facts.md](facts.md)** — the acceptance contract. Every line is a fact that must be
  satisfied and **verified**. Per-fact verification mode is recorded in
  **[facts.meta.json](facts.meta.json)**: the 25 facts with `automatedVerification: true`
  must each be backed by a concrete automated check (Go test, `devbox run fe:test`, or
  equivalent); the remaining facts are verified manually per the plan's Verification section.
- **[plan.md](plan.md)** — the approved, gated execution plan: the version-pinned parser
  library decision (`twpayne/go-gpx` v1.5.0 + `paulmach/orb` v0.13.0), the up-front
  de-risking spikes (Step 0), the Skeleton→Red→Green→Refactor steps, and the mandatory
  end-of-work reviews (Step 9). The plan is to be executed **in full** — every step, not a
  subset.

## Done condition

Done requires **both** of the following, with no partial credit:

1. **Every fact in `facts.md` is satisfied and verified.** Each
   `automatedVerification: true` fact (per `facts.meta.json`) has a passing automated test
   that exercises it; each manual fact is confirmed via the plan's Verification section.
   Map each fact to the check that proves it — a fact with no backing verification is not done.

2. **The entire `plan.md` is executed**, including:
   - Step 0 de-risking spikes (with the chosen Douglas–Peucker threshold recorded);
   - all capability steps (filestore `Open`, `internal/trackgeom`, proto + regenerated
     committed code, mapmutator/codec, gRPC handlers, help docs, frontend widget);
   - Step 9 reviews — the **plan-vs-code** and **plan-vs-transcript** review subagents — run
     and their findings resolved.

Plus the gates: `devbox run go:test`, `devbox run fe:test`, and
`devbox run lint:everything` all green; generated proto/MCP/TS regenerated and committed;
and a manual end-to-end smoke uploading a real Rever/Garmin `.gpx` and confirming render,
download, and tag toggling (incl. the virtual `untagged` group).
