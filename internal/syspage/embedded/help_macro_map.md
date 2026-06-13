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

[[maps.demo.markers]]
uid = "marker-met"
label = "The Met"
lat = 40.7794
lon = -73.9632
popup_markdown = "A second marker loaded from `MapService.GetMap`."
color = "#dc2626"

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

The component reads data through `MapService.GetMap` and renders markers, polygons, and circles with Leaflet.

## Live demo

{{ Map "demo" }}

## Data model

Map data lives in two coordinated frontmatter trees:

- `maps.<name>` — user-mutable map view, style, markers, polygons, and circles.
- `agent.maps.<name>` — wiki-managed metadata such as `updated_at`, `sync_token`, stable element `uid`, timestamps, creator attribution, automated-agent attribution, and sparse `sort_order`.

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
```

The rendered map also includes a tileset selector when more than one supported layer is available. Switching the selector changes the current view immediately; it does not rewrite frontmatter. Use `MapService.SetMapStyle` when the page's saved default should change.

## Map controls

The map fills the page content width and uses a square aspect ratio so route and area context have enough vertical room.

- Drag or swipe to pan.
- Pinch on touch devices to zoom.
- Use Ctrl + scroll on desktop to zoom. Plain scroll over the map shows a short hint instead of unexpectedly zooming the map while reading the page.

Paid or keyed providers such as Mapbox, MapTiler, and Google Maps are intentionally not supported by this macro. Use [[help-macro-google-maps-embed]] only when you need a Google Maps embed iframe.

## Agent API

Use the generated MCP tools for `MapService`:

| Tool | Purpose |
|------|---------|
| `api_v1_MapService_SetMapView` | Create or update the initial center and zoom. |
| `api_v1_MapService_SetMapStyle` | Set the supported tile layer. |
| `api_v1_MapService_AddMarker` / `UpdateMarker` / `MoveMarker` / `DeleteMarker` | Mutate markers. |
| `api_v1_MapService_AddPolygon` / `UpdatePolygon` / `DeletePolygon` | Mutate polygons. |
| `api_v1_MapService_AddCircle` / `UpdateCircle` / `DeleteCircle` | Mutate circles. |
| `api_v1_MapService_ReorderElement` | Update sparse element order. |
| `api_v1_MapService_ReplaceMarkers` | Reconcile imported marker lists while preserving stable UIDs. |
| `api_v1_MapService_DeleteMap` | Delete the map and its metadata. |
| `api_v1_MapService_GetMap` | Read a map for rendering or detailed inspection. |
| `api_v1_MapService_ListMaps` | List compact map outlines on a page. |
| `api_v1_MapService_ListMapElements` | Read compact element outlines with optional type and bounding-box filters. |
| `api_v1_MapService_GetElement` | Drill into one marker, polygon, or circle by UID. |
| `api_v1_MapService_FindMarkers` | Search marker labels and popup markdown. |

Pass `expected_updated_at` from a prior read when applying edits that should fail on concurrent changes.

## Popups

Marker, polygon, and circle popups accept Markdown. Wiki links are rendered by the same server markdown path used by chat and page previews, so normal wiki links work inside popup content.
