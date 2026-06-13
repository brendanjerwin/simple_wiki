+++
identifier = "help_macro_map"

[wiki]
system = true
+++

#help #templating #maps

# Map macro

The Map macro renders a first-class interactive wiki map from map data on the current page.

```
{{ Map "map-name" }}
```

Example:

```
{{ Map "yard" }}
```

The rendered page contains:

```html
<wiki-map name="yard" page="current-page"></wiki-map>
```

The component reads data through `MapService.GetMap` and renders markers, polygons, and circles with Leaflet.

## Data model

Map data lives in two coordinated frontmatter trees:

- `maps.<name>` — user-mutable map view, style, markers, polygons, and circles.
- `agent.maps.<name>` — wiki-managed metadata such as `updated_at`, `sync_token`, stable element `uid`, timestamps, creator attribution, automated-agent attribution, and sparse `sort_order`.

Generic Frontmatter writes to `maps.*` and `agent.*` are rejected. Use `MapService` for all mutations so the two trees stay synchronized.

## Supported layers

OpenStreetMap is the default raster tile layer. Supported free layers include:

- OpenStreetMap
- OpenTopoMap
- Esri World Imagery

The map response includes the required attribution HTML for the selected tile layer, and the frontend renders that attribution automatically.

Paid or keyed providers such as Mapbox, MapTiler, and Google Maps are intentionally not supported by this macro. Use [[help-macro-map-embed]] only when you need a Google Maps embed iframe.

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
