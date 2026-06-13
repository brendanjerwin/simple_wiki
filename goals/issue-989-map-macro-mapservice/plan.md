# Plan

## Approach

Implement issue #989 as one full vertical slice. Build the backend contract first, because the macro, MCP tools, CLI exposure, and web component should all depend on `MapService` rather than raw frontmatter. Keep all map writes behind a single mutator funnel and reserve `maps.*` plus `agent.maps.*` before adding UI behavior.

Use the existing checklist and survey implementations as local patterns:

- `templating/templating.go` for macro registration and validation stubs.
- `api/proto/api/v1/checklist.proto` and `api/proto/api/v1/survey.proto` for typed service shape and MCP descriptions.
- `server/checklistmutator/` and `server/surveymutator/` for dedicated mutation funnels.
- `internal/grpc/api/v1/checklist.go`, `internal/grpc/api/v1/survey.go`, and `internal/grpc/api/v1/server.go` for service wiring.
- `wikipage/reserved_namespaces.go` and `internal/grpc/api/v1/reserved.go` for generic frontmatter rejection.
- `static/js/web-components/wiki-checklist.ts`, `wiki-survey.ts`, and their tests/stories for frontend patterns.

## Steps

1. Add backend data types and service contract.
   - Add `api/proto/api/v1/map.proto` with `MapService`, map config, tile layer, marker, polygon, circle, element metadata, selectivity, bbox, and mutation request/response messages.
   - Use `api.v1` MCP doc extensions on service and RPCs so generated tools have useful descriptions.
   - Include all methods from issue #989: `SetMapView`, `SetMapStyle`, marker/polygon/circle add/update/delete/move where applicable, `ReorderElement`, `ReplaceMarkers`, `DeleteMap`, `GetMap`, `ListMaps`, `ListMapElements`, `GetElement`, and `FindMarkers`.
   - Verification: proto compiles through `devbox run go:generate`; generated Go, Connect, JS, and MCP files are committed.

2. Reserve map namespaces before exposing mutation.
   - Add `maps` to `wikipage/reserved_namespaces.go`.
   - Add the corresponding `reservedNamespaceMessages` entry in `internal/grpc/api/v1/reserved.go`, naming `MapService`.
   - Confirm generic `MergeFrontmatter`, `ReplaceFrontmatter`, and `RemoveKeyAtPath` reject `maps.*` and `agent.maps.*` via the existing reserved-key path.
   - Update ADR/help registry text if the current ADR 0009/0010 wording lists reserved namespaces explicitly.
   - Verification: focused Go tests around reserved namespace rejection and preservation, plus `devbox run go:test`.

3. Build the map mutator funnel.
   - Add `server/mapmutator/` with injected page reader/mutator, clock, and UID generator.
   - Model user-mutable state under `maps.<name>` and wiki-managed state under `agent.maps.<name>`.
   - Serialize mutations per page, load current frontmatter, compute after-state, diff by `uid`, and write both subtrees atomically in one `WriteFrontMatter`.
   - Stamp `created_at`, `updated_at`, `created_by`, `automated`, map-level `updated_at`, and `sync_token` from server clock plus `tailscale.IdentityValue`.
   - Strip or ignore wiki-managed input fields; keep `uid` immutable.
   - Validate lat/lon bounds, polygon point count, positive circle radius, and known tile layer IDs.
   - Implement sparse `sort_order` behavior and `ReplaceMarkers` matching by `(label, lat-rounded-6, lon-rounded-6)`.
   - Verification: unit tests in `server/mapmutator/` for add/update/delete, metadata preservation, sync token increments, validation failures, stale `expected_updated_at`, reorder collision behavior, and `ReplaceMarkers` reconcile behavior.

4. Wire `MapService` into gRPC, Connect, Vanguard, MCP, and CLI.
   - Add `internal/grpc/api/v1/map.go` handlers that validate required fields, enforce `requireUserMutable` and `requireAuthorized`, call `server/mapmutator`, and map mutator errors to appropriate gRPC status codes.
   - Extend `internal/grpc/api/v1/server.go` with a `mapMutator` dependency, `WithMapMutator`, and `RegisterMapServiceServer`.
   - Instantiate the mutator in `internal/bootstrap/server.go`, include `api.v1.MapService` in Vanguard service names, and register it with the API server.
   - Add a `MapServiceClient` to `cmd/wiki-cli/mcp.go`, register generated MCP forwarding, and update MCP regression tests to assert representative map tools are present.
   - Add a human-facing `wiki-cli map` command only if the repo convention expects one for first-class service parity; otherwise document that MCP is the agent surface and frontend is the human surface.
   - Verification: focused gRPC handler tests, MCP registration tests, `devbox run go:generate`, `devbox run go:test`.

5. Add the `Map` macro.
   - Add a `funcNameMap = "Map"` constant, `BuildMap`, runtime FuncMap registration, validation stub, and typo suggestions in `templating/templating.go`.
   - Render `<wiki-map name="..." page="..."></wiki-map>` with escaped attributes.
   - Add tests beside checklist/survey macro tests for escaped name/page, validation, unknown function suggestions, and coexistence with `MapEmbed`.
   - Verification: focused templating tests, then `devbox run go:test`.

6. Build the frontend map component with Leaflet.
   - Add `leaflet` through `devbox`/Bun in `static/js` so `package.json` and `bun.lock` are updated.
   - Add `static/js/web-components/wiki-map.ts`.
   - Use the generated Connect client to call `MapService.GetMap`; do not read raw frontmatter.
   - Integrate Leaflet in light DOM, render markers, polygons, circles, tile layers, attribution, layer switcher, popups, fit-to-bounds, and reduced-motion behavior.
   - Use custom SVG marker icons to avoid Leaflet default icon asset path problems.
   - Render popup markdown through the existing markdown/wiki-link rendering path, reusing `chat-markdown-renderer` or a smaller shared renderer if needed.
   - Register the component in `static/js/index.ts` or the existing component entrypoint.
   - Verification: `wiki-map.test.ts` stubs the Connect client/network, asserts service usage, rendering state, popup markdown link rendering, tile attribution behavior, error handling, and no raw frontmatter dependency. Run with `devbox run fe:test -- web-components/wiki-map.test.ts`, then `devbox run fe:test`.

7. Add Storybook and e2e coverage.
   - Add `static/js/web-components/wiki-map.stories.ts` using the real `wiki-map` component with mock data/client behavior, not mock HTML.
   - Include states for loading, error, marker/polygon/circle map, multiple tile layers, and interactive event logging where useful.
   - Add `e2e/tests/map.spec.ts` for macro render and visible map behavior.
   - Add `e2e/tests/map-a11y.spec.ts` to verify keyboard/a11y expectations.
   - Verification: `devbox run storybook:build` if Storybook config is touched, and relevant `devbox run e2e:test` target or documented focused Playwright command if the script supports one.

8. Add help and user documentation.
   - Add `internal/syspage/embedded/help_macro_map.md` with syntax, data shape, supported tile layers, UI behavior, agent guidance, and MapService-only mutation guidance.
   - Update `internal/syspage/embedded/help.md` and `help_templating.md` to list the new `Map` macro separately from `MapEmbed`.
   - Include OSM attribution/privacy notes and paid/keyed service exclusion.
   - Verification: system page loader tests and `devbox run go:test`.

9. Final generation and full verification.
   - Run `devbox run go:generate`.
   - Inspect generated diffs and commit all generated files changed by proto/frontend generation.
   - Run `devbox run go:test`.
   - Run `devbox run fe:test`.
   - Run relevant e2e checks for map rendering and a11y.
   - Run `devbox run storybook:build` if stories or Storybook config changed.
   - Confirm `git status --short` contains only intentional source, generated, doc, test, and lockfile changes.

## Risks

- `maps.*` is a user-mutable namespace but also reserved from generic frontmatter writes. This differs from checklist's legacy `checklists.*` pattern and should be documented clearly in ADR/help text.
- `agent.maps.*` lives under the already-reserved `agent` top-level namespace; tests should prove both `maps` and `agent` rejection messages steer callers to `MapService` when map data is involved where practical.
- Leaflet may need test shims for DOM APIs not present in unit tests; prefer isolating Leaflet adapter logic so service behavior and render decisions remain unit-testable.
- OSM/OpenTopoMap/Esri tiles should not be fetched in unit tests or Storybook by default. Stub network calls or provide controllable tile URLs in test stories.
- If `tailscale.IdentityValue` cannot express every desired `ctx.Principal()` behavior yet, implement attribution through the current identity abstraction and document any small adapter needed rather than inventing a separate protocol-aware path inside the mutator.
