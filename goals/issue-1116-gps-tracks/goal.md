# Issue 1116 — GPS track overlays & tag-based map layer controls

Extend the first-class wiki map (PR #1115) with a GPS **track** overlay type whose large
payload is stored as a content-addressed wiki file (reusing `FileStorageService`) while
frontmatter holds only a reference; a read-only `GetTrackGeometry` RPC parses GPX/GeoJSON
server-side into normalized, simplified segments that the `<wiki-map>` widget loads lazily
and draws as polylines. Add **tags** to markers, polygons, circles, and tracks driving a
native Leaflet layer control (with a virtual `untagged` group; all tags visible by
default, OR semantics for multi-tag overlays), and let tracks be attached both from the
widget (a tap-revealed tools panel mirroring `wiki-image`) and by agents via new MCP tools.

The shared understanding (acceptance) is the accepted fact sheet in [facts.md](facts.md).

The approved execution plan — including the version-pinned parser library decision
(`twpayne/go-gpx` v1.5.0 + `paulmach/orb` v0.13.0) and the up-front de-risking spikes — is
in [plan.md](plan.md).

**Done** means: every accepted fact in `facts.md` is satisfied; the new GPX/GeoJSON track
overlay, server-side `GetTrackGeometry`, tags + native tag layer control (incl. virtual
`untagged`), widget tap-reveal upload + download, and agent MCP track tools all work
end-to-end; help docs are updated; generated proto/MCP/TS is regenerated and committed; and
verification passes — `devbox run go:test`, `devbox run fe:test`, and
`devbox run lint:everything` all green, plus a manual smoke uploading a real Rever/Garmin
`.gpx` and confirming render, download, and tag toggling.
