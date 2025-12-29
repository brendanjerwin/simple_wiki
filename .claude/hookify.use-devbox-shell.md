---
name: use-devbox-shell
enabled: true
event: bash
action: warn
conditions:
  - field: command
    operator: regex_match
    pattern: ^(go|ginkgo|buf|bun|bunx|npm|npx|markdownlint|evans|grpcurl|revive|staticcheck|playwright)\s
  - field: command
    operator: not_contains
    pattern: devbox
---

⚠️ **Direct tool invocation detected**

This project uses **devbox** to manage dependencies. Running tools directly may use incorrect versions or miss environment setup.

**Instead of:**
```bash
go test ./...
bun install
```

**Use:**
```bash
devbox shell -- go test ./...
devbox shell -- bun install
```

**Or use devbox scripts:**
```bash
devbox run go:test
devbox run fe:test
devbox run lint:everything
```

Run `devbox run` to see all available scripts.
