---
name: use-devbox-shell
enabled: true
event: bash
action: warn
conditions:
  - field: command
    operator: regex_match
    pattern: (^|&&\s*)(go|ginkgo|buf|bun|bunx|npm|npx|markdownlint|evans|grpcurl|revive|staticcheck|playwright)\s
  - field: command
    operator: not_contains
    pattern: devbox
---

⚠️ **Direct tool invocation detected**

This project uses **devbox** to manage dependencies. Running tools directly may use incorrect versions or miss environment setup.

**Instead of:**
```bash
cd static/js && bun run typecheck
bun install
go test ./...
```

**Use devbox scripts (no cd needed):**
```bash
devbox run fe:typecheck
devbox run fe:test
devbox run go:test
devbox run lint:everything
```

**Or use devbox shell for direct commands:**
```bash
devbox shell -- go test ./...
devbox shell -- bun install
```

Run `devbox run` to see all available scripts.
