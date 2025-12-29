---
name: use-devbox-run-scripts
enabled: true
event: bash
action: warn
conditions:
  - field: command
    operator: regex_match
    pattern: ^(go\s+test|go\s+build|go\s+generate|go\s+vet|bun\s+(run\s+)?test|bun\s+(run\s+)?build|bun\s+(run\s+)?lint|bun\s+run\s+storybook|eslint|revive|staticcheck|markdownlint|playwright\s+test|gh\s+workflow\s+run\s+deploy)
  - field: command
    operator: not_contains
    pattern: devbox
---

⚠️ **Use devbox scripts for this operation**

This project has devbox scripts configured for common tasks. Using them ensures consistent environment and proper setup.

**Available scripts:**

| Instead of... | Use... |
|---------------|--------|
| `go test ./...` | `devbox run go:test` |
| `go build` | `devbox run build` |
| `go generate ./...` | `devbox run go:generate` |
| `go vet` | `devbox run lint:everything` |
| `revive` / `staticcheck` | `devbox run go:lint` |
| `bun run test` | `devbox run fe:test` |
| `bun run build` | `devbox run lint:everything` |
| `eslint` / `bun run lint` | `devbox run fe:lint` |
| `markdownlint` | `devbox run lint:md` |
| `playwright test` | `devbox run e2e:test` |
| `bun run storybook` | `devbox run storybook` |
| `gh workflow run deploy.yml` | `devbox run deploy` |

**Full validation:** `devbox run lint:everything`

Run `devbox run` to see all available scripts.
