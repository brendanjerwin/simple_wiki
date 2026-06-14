# GPX fixtures for issue 1116

Real Rever exports (the two download styles Rever offers — "For Garmin, TomTom,
etc."), **anonymized** for this public repo. Every coordinate was translated by a
constant offset to a fictional area (route shape preserved for simplification
testing), `<time>` elements stripped, and personal name/ride name replaced with
`Example Rider` / `Example Ride`. The GPX structure, the `creator="REVER"`
attribute, and the Garmin namespaced extensions are left intact so they exercise
the real parser paths.

| File | Style | Exercises |
|------|-------|-----------|
| `rever-turn-by-turn-route.gpx` | Turn-by-turn **route** | `<rte>/<rtept>` plus Garmin `gpxx:RoutePointExtension` (`gpxx:rpt`) and `<wpt>` waypoints — confirms route extraction and graceful handling of namespaced extensions. |
| `rever-track.gpx` | **Track** | `<trk>/<trkseg>/<trkpt>` plus `<wpt>` waypoints — confirms track-segment extraction. |

Both are GPX 1.1, `creator="REVER https://a.rever.co"`.

Use in Step 0 (de-risking spikes) and fold into `internal/trackgeom`'s tests
during Step 2 — copy/move them to `internal/trackgeom/testdata/` there.
