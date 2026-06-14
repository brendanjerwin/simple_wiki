# Plan — Issue 1116: GPS track overlays & tag-based map layer controls

## Solution approach

Extend the existing first-class wiki map (PR #1115) with a fourth overlay type —
**tracks** — whose large payload is stored as a content-addressed wiki file (reusing
`FileStorageService`) while frontmatter holds only a reference (`file_hash`, `filename`,
`format`). A new read-only `GetTrackGeometry` RPC parses the referenced GPX/GeoJSON file
server-side into normalized segments, simplifying very large tracks; the `<wiki-map>`
widget fetches that geometry lazily after first paint and draws polylines. **Tags** are
added to all four overlay types and drive a native Leaflet layer control, where overlays
with no real tags carry a virtual `untagged` tag (all tags visible by default; OR
semantics for multi-tag overlays). Tracks can be attached both from the widget (a
tap-revealed tools panel mirroring `wiki-image`) and by agents via new MCP tools.

Every layer follows the established #1115 patterns: the mutator funnel
(`server/mapmutator`), the frontmatter codec, the gRPC handler shape
(`internal/grpc/api/v1/map.go`), and the Leaflet renderer (`wiki-map.ts`). New code is
introduced only where no reusable seam exists: a `FileStorer.Open` content-read method
and a thin `internal/trackgeom` package that wraps mature libraries.

### Parser library decision (researched + version-pinned)
Rather than hand-roll XML/JSON parsing and a simplification algorithm, `internal/trackgeom`
wraps two well-maintained libraries:
- **GPX → `github.com/twpayne/go-gpx` v1.5.0** (2025-04). `gpx.Read(io.Reader) (*GPX, error)`
  yields `GPX{Trk []*TrkType, Rte []*RteType, Wpt []*WptType}`, with
  `TrkType.TrkSeg[].TrkPt[]` and `RteType.RtePt[]` all `*WptType{Lat, Lon float64}`. Because
  it builds on `encoding/xml`, unknown namespaced extensions (Garmin `gpxx`, Rever) are
  ignored gracefully — exactly what facts f-gpx requires. Maps cleanly: each `TrkSeg` and
  each `Rte` becomes one segment.
- **GeoJSON + simplification → `github.com/paulmach/orb` v0.13.0** (2026-03). `geojson`
  sub-package parses Geometry/Feature/FeatureCollection (LineString, MultiLineString, …);
  `simplify.DouglasPeucker(threshold).Simplify(geom)` provides Douglas–Peucker for **both**
  formats (GPX segments are converted to `orb.LineString` and simplified the same way),
  so f-simplify needs no bespoke algorithm.

Both repos vendor cleanly; the repo vendors deps, so `go get` + `go mod vendor` are part of
the parser step. The library choice is de-risked by spikes (Step 0) before the feature is built.

Note on storage availability: file upload is intrinsic to the wiki (not a configurable
add-on). The track feature does not model an "uploads disabled" state as a feature; the
server still guards a nil storer defensively (no panic), but that is an internal
invariant, not a user-facing behavior.

## Ordered steps

Each capability follows Skeleton → Red → Green → Refactor. Step 0 (spikes) runs first to
de-risk the library choice. Steps 1–2 are independent and parallelizable; step 3 (proto +
first codegen) unblocks the Go layers (4–5) and the TS layer (8).

### 0. De-risking spikes (do first, throwaway code) — de-risks: f-gpx, f-geojson, f-simplify
Validate the library choice against **real** exports before building on it. Drop a real
Rever-exported `.gpx` and a real Garmin-exported `.gpx` into `internal/trackgeom/testdata/`
(if the user can't supply them, synthesize GPX 1.1 with Garmin `gpxx` TrackExtension +
Rever-style `trk`/`rte` so the spike still exercises namespaced extensions).
- Spike A — `go get github.com/twpayne/go-gpx@v1.5.0`; tiny `main` (or throwaway test) that
  `gpx.Read`s each file and prints segment + point counts for `Trk`/`TrkSeg` and `Rte`.
  Confirms both track and route extraction and that extensions don't break parsing.
- Spike B — `go get github.com/paulmach/orb@v0.13.0`; feed a long LineString through
  `simplify.DouglasPeucker(threshold)` at a few thresholds; record the point-count reduction
  to pick the production threshold (resolves the simplification-tuning open question).
- Promote: keep the chosen versions in `go.mod` (+ `go mod vendor`), delete throwaway mains,
  fold the representative fixtures into the real `internal/trackgeom` tests in Step 2.
- Verify: spike programs compile and print sane counts/reductions; record the chosen DP
  threshold in Step 2's notes.

### 1. `FileStorer.Open` content read  — facts: f-open
- Files: `filestore/file_storer.go` (add `Open(hash) (io.ReadCloser, error)` to interface),
  `filestore/disk_file_storer.go` (impl via `validateHashPath` + `os.Open(dataDir/<hash>.upload)`),
  `filestore/disk_file_storer_test.go`.
- Verify: `devbox run go:test` — returns content; `ErrInvalidHash` on traversal; `os.ErrNotExist` when absent.

### 2. `internal/trackgeom` parser package — facts: f-gpx, f-geojson, f-simplify, f-geom-rpc(part)
- Files (new): `internal/trackgeom/trackgeom.go`, `..._test.go`, `internal/trackgeom/testdata/*`.
  Wraps go-gpx + orb (chosen in Step 0).
- `ParseGPX(io.Reader) ([]Segment, error)` (map `Trk.TrkSeg` and `Rte` → segments via go-gpx),
  `ParseGeoJSON(io.Reader) ([]Segment, error)` (orb `geojson` → segments), `Parse(format, reader)`
  dispatcher. Simplify each segment with `orb` Douglas–Peucker at the Step 0 threshold, plus a
  hard safety cap on total points. A `Segment` is an ordered `[]*apiv1.GeoPoint` (mapped at the edge).
- Verify: `devbox run go:test` — GPX `trk/trkseg/trkpt` multi-segment, `rte/rtept`, Garmin/Rever
  namespaced extensions tolerated, invalid XML errors; GeoJSON LineString/MultiLineString/
  Feature(Collection), invalid JSON errors; simplification reduces a dense line; unknown format errors.

### 3. Proto + first codegen — facts: f-file-ref, f-reads, f-tags-all, f-reorder, f-crud, f-geom-rpc
- Files: `api/proto/api/v1/map.proto`; then `devbox run` go:generate → commit `gen/go/...`
  (incl. `apiv1mcp`, Connect-Go) and `static/js/gen/...` (Connect-ES).
- Add: `MAP_ELEMENT_TYPE_TRACK=4`; `enum TrackFormat{UNSPECIFIED,GPX,GEOJSON}`; `repeated string tags`
  on `MapMarker`(=6)/`MapPolygon`(=7)/`MapCircle`(=8); `message MapTrack`; `Map.tracks=10`;
  `MapOutline.track_count=8`; `GetMapRequest.include_tracks=7`; `GetElementResponse.track=4`;
  `message TrackSegment`; RPCs `AddTrack`/`UpdateTrack`/`DeleteTrack` and read-only `GetTrackGeometry`
  (with `api.v1.description`/`read_only` annotations matching existing style).
- Verify: `devbox run` go:generate clean; `git status` shows regenerated Go/TS/MCP; project compiles.

### 4. mapmutator codec + mutator — facts: f-track-fm, f-uid, f-crud, f-reorder, f-tags-all, f-failfast(data)
- Files: `server/mapmutator/codec.go` (keys `tracksKey/fileHashKey/filenameKey/formatKey/tagsKey`;
  encode/decode tracks user-data + metadata; `tags` encode/decode on all overlays;
  `format`↔`TrackFormat` string), `server/mapmutator/mutator.go` (`AddTrack`/`UpdateTrack`/`DeleteTrack`
  via `mutateMap`+`newMetadata`; `validateTrack` — `file_hash` required, `format` known; extend
  `metadataForElement`/`sortMapElements`/`totalElementCount` + clone/find/remove for tracks),
  `server/mapmutator/mutator_test.go`.
- Verify: `devbox run go:test` — track CRUD; reorder track; uid stable & file swappable; tags
  round-trip on every overlay; validation errors.

### 5. gRPC handler — facts: f-crud, f-failfast, f-reads, f-geom-rpc, f-geom-errors
- Files: `internal/grpc/api/v1/map.go`, `internal/grpc/api/v1/map_grpc_test.go`.
- `AddTrack`/`UpdateTrack`/`DeleteTrack` handlers (mirror markers; `requireMapMutation`). **Fail-fast:**
  before mutating, `s.fileStorer.Open(file_hash)` + `trackgeom.Parse` → missing file ⇒ `NotFound`,
  unparseable ⇒ `InvalidArgument`. `GetTrackGeometry` (read auth via `readAuthorizedMap`): find track →
  open → parse → segments; unknown uid/missing file ⇒ `NotFound`, corrupt stored file ⇒ parse error.
  `GetMap` honors `include_tracks`; `GetElement` track case; `elementOutlines`/`ListMaps` count tracks
  (track outline representative point nil; bbox skips tracks — documented). Nil-storer guarded defensively.
- Verify: `devbox run go:test` — AddTrack happy + parse-reject + missing-file; GetTrackGeometry
  success/not-found/corrupt; GetMap include_tracks; GetElement track; tags echoed; ListMapElements lists tracks.

### 6. Help docs — facts: f-help, f-aspect, f-agent
- File: `internal/syspage/embedded/help_macro_map.md`. Document tracks (data model, file reference,
  GPX/GeoJSON, KML deferred), tags + virtual `untagged`, the layer control, both upload paths
  (widget tap-reveal + agent `UploadFile`+`AddTrack`), download, lazy loading, new MCP tools; restate
  configurable aspect ratio. Add `tags` to the live demo markers so the control renders in the demo
  (no live track — its hash wouldn't exist).
- Verify: render `help_macro_map` in a running instance; demo control shows tag toggles.

### 7. (covered by 3) Regenerate is committed as part of step 3 and re-run after any proto touch. — fact: f-gen

### 8. Frontend `<wiki-map>` — facts: f-polyline, f-lazy, f-control, f-toggle, f-or, f-untagged, f-affordance, f-popover, f-upload-errors, f-download
- Files: `static/js/web-components/wiki-map.ts`, `static/js/web-components/wiki-map.test.ts`
  (+ optional stories). Reference pattern: `wiki-image.ts` `tools-open` tap-reveal.
- **Tracks (lazy):** after first paint, per `mapData.tracks` call `client.getTrackGeometry` and add an
  `L.polyline` per segment (track color); errors via `AugmentErrorService`.
- **Tag control (native + virtual untagged):** build `L.control.layers` with one entry per distinct
  real tag plus a virtual `untagged` entry (overlays with zero real tags). All enabled by default.
  Drive visibility via `overlayadd`/`overlayremove` → `enabledTags` set → recompute: a tagged overlay is
  on the map iff any of its tags ∈ `enabledTags`; an untagged overlay iff `untagged` ∈ `enabledTags`.
- **Upload (tap-reveal tools panel):** tapping the map opens a tools panel (mirror `wiki-image` `toolsOpen`)
  containing "Add GPS track" → popover: file input (`.gpx,.geojson,.json`), label (default = filename),
  optional comma-separated tags → `FileStorageService.UploadFile` (new Connect-ES client) →
  `MapService.AddTrack` (format from extension) → reload. Errors surfaced, never silent.
- **Download:** track popup includes a `/uploads/<file_hash>?filename=<filename>` link.
- Verify: `devbox run fe:test` — polyline from stubbed `getTrackGeometry`; control lists tags + `untagged`;
  toggling hides/shows matching overlays; untagged toggle hides untagged; tap reveals tools (hidden at rest);
  upload flow UploadFile→AddTrack→reload with format/tags parsed; upload error surfaces; download link present;
  lazy fetch after initial render.

### 9. End-of-work reviews + full gate — facts: f-slice, f-verify
- Plan-vs-code review subagent (`git diff main...HEAD` vs this plan + facts).
- Plan-vs-transcript review subagent (session transcript vs this plan).
- `devbox run lint:everything` green; manual smoke per Verification below.

## Verification (end-to-end)
1. `devbox run go:test`, `devbox run fe:test`, `devbox run lint:everything` all green.
2. Manual: `devbox services start`; page with `{{ Map "ride" }}`; tap map → Add GPS track → upload a
   real Rever/Garmin `.gpx`; track renders as a polyline; download link returns the file; tag toggles
   (incl. `untagged`) show/hide overlays; untagged visible by default; large GPX doesn't block first paint.
3. MCP smoke: `api_v1_MapService_AddTrack` then `api_v1_MapService_GetTrackGeometry` return segments;
   `GetMap` includes the track.

## Resolved decisions (previously open — answered via research/spike)
- **Parser library:** RESOLVED — go-gpx v1.5.0 + orb v0.13.0 (see decision section); confirmed
  against the libraries' source and de-risked by Step 0 spikes on real Rever/Garmin exports.
- **Simplification approach:** RESOLVED — use orb Douglas–Peucker (no hand-rolled algorithm),
  segment-wise to preserve segment breaks, plus a hard safety cap. The exact DP threshold is fixed
  by Step 0 Spike B against real exports and recorded in Step 2.
- **Track outline representative point / bbox:** RESOLVED — a track's geometry isn't known without
  parsing the file, so track outlines carry no representative point and are not bbox-filtered in
  `ListMapElements`. Documented; acceptable for the agent-outline use case.
- **Multi-tag + native Leaflet control:** RESOLVED — `L.control.layers` removes a group's child
  layers via `map.removeLayer` per child, so a layer instance shared across tag groups can't express
  OR visibility through native group membership. Approach: keep the native control UI but drive real
  overlay visibility ourselves from the control's `overlayadd`/`overlayremove` events against an
  `enabledTags` set. Verified by frontend tests, not by Leaflet's shared-layer semantics.
- **First FE mutation surface:** RESOLVED (not a risk) — the widget gains its first client-side
  mutation (upload); it mirrors the existing `wiki-image` `tools-open` tap pattern, so it is
  consistent with established component behavior.

## Remaining open question
- **Real-export fixtures:** the strongest de-risking uses genuine Rever- and Garmin-exported `.gpx`
  files. If the user can provide them they go in `internal/trackgeom/testdata/`; otherwise Step 0
  synthesizes GPX 1.1 with Garmin `gpxx` + Rever-style `trk`/`rte`. This only affects fixture realism,
  not the design.
