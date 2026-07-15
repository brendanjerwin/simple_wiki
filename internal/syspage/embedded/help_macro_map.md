+++
identifier = "help_macro_map"

[wiki]
system = true

[maps.demo.view]
lat = 40.7812
lon = -73.9665
zoom = 15

[maps.demo.style]
tile_layer_id = 1

[[maps.demo.markers]]
uid = "marker-belvedere"
label = "Belvedere Castle"
lat = 40.7794
lon = -73.9691
popup_markdown = "Belvedere Castle marker rendered by the first-class `Map` macro."
color = "#2563eb"
tags = ["landmark"]

[[maps.demo.markers]]
uid = "marker-met"
label = "The Met"
lat = 40.7794
lon = -73.9632
color = "#dc2626"
tags = ["landmark", "museum"]

[agent.maps.demo]
updated_at = "2026-06-12T20:00:00Z"
sync_token = 1

[agent.maps.demo.markers.marker-belvedere]
created_at = "2026-06-12T20:00:00Z"
updated_at = "2026-06-12T20:00:00Z"
created_by = "system"
automated = true
sort_order = 1000

[agent.maps.demo.markers.marker-met]
created_at = "2026-06-12T20:00:00Z"
updated_at = "2026-06-12T20:00:00Z"
created_by = "system"
automated = true
sort_order = 2000
+++

#help #templating #maps

# Map macro

The Map macro renders a first-class interactive wiki map from map data on the current page.

```
{{"{{ Map \"map-name\" }}"}}
```

Example:

```
{{"{{ Map \"yard\" }}"}}
```

The rendered page contains:

```html
<wiki-map name="yard" page="current-page"></wiki-map>
```
The component reads data through `MapService.GetMap` and renders markers, polygons, circles, and GPS track overlays with Leaflet.

## Live demo

{{ Map "demo" }}

## Data model

- `maps.<name>` — user-mutable map view, style, markers, polygons, circles, and tracks.
- `agent.maps.<name>` — wiki-managed metadata such as `updated_at`, `sync_token`, stable element `uid`, timestamps, creator attribution, automated-agent attribution, and sparse `sort_order`. Element metadata is keyed by `uid` under `markers`, `polygons`, `circles`, and `tracks`.

Generic Frontmatter writes to `maps.*` and `agent.*` are rejected. Use `MapService` for all mutations so the two trees stay synchronized.

## Supported layers

OpenStreetMap is the default raster tile layer. Supported free layers include:

- `tile_layer_id = 1` — OpenStreetMap
- `tile_layer_id = 2` — OpenTopoMap
- `tile_layer_id = 3` — Esri World Imagery

The map response includes the required attribution HTML for the selected tile layer, and the frontend renders that attribution automatically.

Set the default tileset in frontmatter:

```toml
[maps.yard.style]
tile_layer_id = 2
aspect_ratio = "16:9"
```

The rendered map also includes a tileset selector when more than one supported layer is available. Switching the selector changes the current view immediately; it does not rewrite frontmatter. Use `MapService.SetMapStyle` when the page's saved default should change.

## Map controls

The map fills the page content width and uses a `16:9` aspect ratio by default so it fits well on laptop screens. Set `maps.<name>.style.aspect_ratio` to another `width:height` value, such as `"3:2"` or `"4:3"`, when a page needs more vertical route context — track-heavy pages benefit from taller ratios since polylines are rendered lazily after the first paint.
- Pinch on touch devices to zoom.
- Use Ctrl + scroll on desktop to zoom. Plain scroll over the map shows a short hint instead of unexpectedly zooming the map while reading the page.

Paid or keyed providers such as Mapbox, MapTiler, and Google Maps are intentionally not supported by this macro. Use [[help-macro-google-maps-embed]] only when you need a Google Maps embed iframe.

## Tags and the layer control

Markers, polygons, circles, and tracks can all carry a `tags` list. The rendered map shows a native Leaflet layer control listing every distinct real tag across all overlays, plus a virtual `untagged` entry for overlays with no tags. Every entry is toggle-able and all are visible by default.

Toggling a tag shows or hides every overlay carrying that tag — this only changes the current view, it never edits the page. An overlay with multiple tags stays visible when **any** of its tags is enabled (OR semantics). Assigning a real tag to an overlay removes it from the virtual `untagged` group.

## GPS track overlays

A GPS **track** is a fourth overlay type whose large payload is stored as a content-addressed wiki file (via `FileStorageService`), while the map frontmatter holds only a reference: `file_hash`, `filename`, and `format` (`GPX` or `GeoJSON`). Track coordinates are never stored in frontmatter.

Supported formats:

- **GPX** — both `<trk>/<trkseg>/<trkpt>` track data and `<rte>/<rtept>` route data, tolerating Garmin/Rever namespaced extensions.
- **GeoJSON** — `LineString`, `MultiLineString`, `Feature`, and `FeatureCollection` geometries.

KML import is not supported yet and is deferred to a later change.

A read-only `MapService.GetTrackGeometry` RPC parses the referenced file server-side and returns normalized, simplified track segments. Very large tracks are simplified with Douglas-Peucker down to a point ceiling so the returned geometry stays renderable.

## Lazy loading

Track geometry is fetched lazily after the initial map paint, so a large track does not block the first render. The widget loads track geometry once the map is visible (via an `IntersectionObserver`), then draws each segment as a Leaflet polyline alongside the markers, polygons, and circles. If a track file cannot be parsed, the failed track label is shown in the layer control rather than crashing the map.

## Uploading and downloading tracks

Tracks can be attached from two surfaces:

- **Widget upload** — tap or activate the map to reveal a tools panel (mirroring how image controls are revealed), then choose "Add GPS track". A popover provides a file picker (`.gpx` or `.geojson`), a label field defaulting to the chosen filename, and an optional comma-separated tags field. Submitting uploads the file via `FileStorageService.UploadFile`, attaches it as a track via `MapService.AddTrack` (format inferred from the extension), and reloads the map. Upload and attach errors are surfaced in the popover, never silently swallowed.
- **Agent upload** — agents upload the file with the existing `api_v1_FileStorageService_UploadFile` tool, then attach and fully configure it (color, popup, tags, order, file replacement) with the `MapService` track tools below.

Each attached track's popup includes a download link of the form `/uploads/<file_hash>?filename=<filename>` so the original file can be retrieved.

## Agent API

Use the generated MCP tools for `MapService`:

| Tool | Purpose |
|------|---------|
| `api_v1_MapService_SetMapView` | Create or update the initial center and zoom. |
| `api_v1_MapService_SetMapStyle` | Set the supported tile layer. |
| `api_v1_MapService_AddMarker` / `UpdateMarker` / `MoveMarker` / `DeleteMarker` | Mutate markers. |
| `api_v1_MapService_AddPolygon` / `UpdatePolygon` / `DeletePolygon` | Mutate polygons. |
| `api_v1_MapService_AddCircle` / `UpdateCircle` / `DeleteCircle` | Mutate circles. |
| `api_v1_MapService_AddTrack` / `UpdateTrack` / `DeleteTrack` | Mutate GPS track overlays (file_hash + filename + format + color + tags). |
| `api_v1_MapService_GetTrackGeometry` | Read-only server-side parse of a track file into simplified segments. |
| `api_v1_MapService_ReorderElement` | Update sparse element order. |
| `api_v1_MapService_ReplaceMarkers` | Reconcile imported marker lists while preserving stable UIDs. |
| `api_v1_MapService_DeleteMap` | Delete the map and its metadata. |
| `api_v1_MapService_GetMap` | Read a map for rendering or detailed inspection. |
| `api_v1_MapService_ListMaps` | List compact map outlines on a page. |
| `api_v1_MapService_ListMapElements` | Read compact element outlines with optional type and bounding-box filters. |
| `api_v1_MapService_GetElement` | Drill into one marker, polygon, circle, or track by UID. |
| `api_v1_MapService_FindMarkers` | Search marker labels and popup markdown. |

Pass `expected_updated_at` from a prior read when applying edits that should fail on concurrent changes.

## Popups

Marker, polygon, circle, and track popups accept Markdown. Wiki links are rendered by the same server markdown path used by chat and page previews, so normal wiki links work inside popup content. Track popups also show the computed distance and a download link for the original file.
