# Facts

- The goal implements issue #989 as a full vertical slice, not as a backend-only or frontend-only partial.
- No acceptance criteria from issue #989 are planned deferrals for this goal.
- The Map template macro emits a wiki-map component for a named map on the current page.
- The wiki-map component fetches map data through MapService.GetMap and renders the map with Leaflet.
- Markers, polygons, and circles render from maps.<name> frontmatter data.
- OpenStreetMap raster is the default tile layer, and supported free tile layers render required attribution automatically.
- The MapService API and MCP surface cover map view/style updates, marker/polygon/circle mutation, element reads, list reads, marker search, reordering, replacement, and map deletion as described in issue #989.
- All map mutations go through a central map mutator funnel that atomically updates user-mutable maps.<name> data and wiki-managed agent.maps.<name> metadata.
- The mutator funnel validates latitude, longitude, polygon point count, circle radius, and tile layer IDs.
- Map element metadata is derived by the service layer, including timestamps, stable UIDs, sync tokens, creator attribution, and automated-agent attribution where the current identity subsystem supports it.
- Generic frontmatter mutation paths reject external writes touching maps.* or agent.maps.* with InvalidArgument guidance naming MapService.
- MapService read methods support context-efficient reads, including outline element lists, bbox filtering, single-element drill-down, marker search, and GetMap selectivity flags.
- Marker popups render markdown, including wiki links, in the user-facing map experience.
- The implementation includes Storybook coverage for the wiki-map component using the real component, not mock HTML.
- The implementation includes help_macro_map documentation that presents MapService as the only map mutation entry point.
- The goal excludes paid or keyed external map services such as Mapbox, MapTiler, and Google Maps.
- Verification includes generated files, focused tests during development, devbox run go:test, devbox run fe:test, and relevant e2e checks for the map feature.
