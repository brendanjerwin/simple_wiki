# Project Context

## Purpose

simple_wiki is a lightweight, self-hosted personal wiki for note-taking and knowledge management.

**Key capabilities:**
- Markdown editing with auto-save
- Wiki-link syntax (`[[PageName]]`) for easy page linking
- Full-text search (Bleve-powered)
- Version history for all pages
- YAML/TOML frontmatter for structured metadata
- Android app via Capacitor for offline access

## Tech Stack

### Backend
- **Go** (latest) - Primary backend language
- **Gin** - HTTP web framework
- **gRPC/gRPC-Web** - API layer (Protocol Buffers)
- **Bleve** - Full-text search
- **Goldmark** - Markdown rendering (with mermaid, emoji, wikilinks extensions)

### Frontend
- **TypeScript** - Frontend language
- **Lit 3** - Web components
- **Bun** - Package manager and bundler
- **Connect RPC** - gRPC-Web client

### Mobile
- **Capacitor 7** - Android app packaging

### Development
- **Devbox** - Reproducible development environment
- **Ginkgo/Gomega** - Go BDD testing
- **Web Test Runner** - Frontend testing (Chai/Sinon)
- **Storybook/Chromatic** - Component documentation and visual regression
- **Playwright** - E2E testing
- **buf** - Protocol buffer tooling

## Project Conventions

**All conventions are documented in `CONVENTIONS.md`.** Key areas covered:

- **Code Style**: Clean code principles, naming conventions, IoC patterns
- **Testing**: TDD workflow, context-specification style, assertion patterns
- **Error Handling**: Selective exception handling, error wrapping patterns
- **Frontend**: All JS in `static/js/`, co-located tests, Storybook guidelines

**Run before committing:** `devbox run lint:everything`

## Architecture Patterns

Architecture decisions are documented in ADRs at `docs/adrs/`:

| ADR | Decision |
|-----|----------|
| 0001 | gRPC and gRPC-Web APIs with buf toolchain |
| 0002 | Devbox for dependency management |
| 0003 | Connect-ES for frontend gRPC |
| 0004 | Error handling architecture (selective exception handling) |
| 0005 | Rolling migrations for frontmatter transformations |
| 0006 | Parallel multi-index background architecture |

**System design overview:** `docs/system_design.md`

## Domain Context

- **Pages**: Markdown files with optional TOML/YAML frontmatter
- **Frontmatter**: Structured metadata (strings, string lists, nested maps only)
- **Wikilinks**: `[[PageName]]` auto-links between pages
- **Page Storage**: `.md` files (current) + `.json` files (version history)
- **Filename Hashing**: Page names are "munged" for equivalency (e.g., "Home" = "home")
- **Indexing**: Pages indexed on save for search and template queries

## Important Constraints

- Generated files must be committed (Go generate, frontend builds)
- Deploy only tagged releases, never branches
- All tests/linters must pass: `devbox run lint:everything`
- Use `devbox run` scripts, not raw commands (`go test`, `bun`, etc.)

## External Dependencies

- **Chromatic** - Visual regression testing for Storybook
- **GitHub Actions** - CI/CD
- **Android SDK** - Mobile builds (optional: `devbox run android:setup`)

## Key Documentation

| Document | Purpose |
|----------|---------|
| `CONVENTIONS.md` | Code style, testing, error handling conventions |
| `README.md` | User-facing documentation, getting started |
| `docs/system_design.md` | Data storage and indexing overview |
| `docs/android-development.md` | Mobile app development guide |
| `docs/adrs/` | Architecture Decision Records |
