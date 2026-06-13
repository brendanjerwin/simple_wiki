# Issue 989 Map Macro with MapService MCP Surface

Implement issue #989 as a full vertical slice for first-class wiki maps. The work adds a `Map` macro, a typed `MapService` API/MCP surface, a central map mutator funnel, reserved namespace protection for map data, Leaflet-based rendering, tests, Storybook coverage, and help documentation.

Accepted facts are in [facts.md](facts.md).

The approved execution plan is in [plan.md](plan.md).

Done means all accepted facts are satisfied, generated files are committed, and verification includes focused development tests plus `devbox run go:test`, `devbox run fe:test`, and relevant map e2e checks.
