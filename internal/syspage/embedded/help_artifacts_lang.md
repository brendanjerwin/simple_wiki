+++
identifier = "help_artifacts_lang"

[wiki]
system = true
+++

#help #agents #artifacts

# Artifact Apps: OpenUI Lang

Wiki pages can host an interactive **artifact** — an OpenUI Lang program rendered
above the page's prose. Artifacts are useful for trackers, dashboards, forms, and
any other app that needs persistent state and reactive UI.

## How it works

An artifact lives in a fenced code block at the top of a page's markdown body:

````
```wiki-artifact
root = Stack([header, btn])
header = Heading("My Counter", 2)
btn = Button("Increment")
```
````

The wiki renders this block as a live app above the page prose. The raw fence
is never shown to readers. Any prose below the fence renders normally.

## ArtifactService — writing programs

Use **`api_v1_ArtifactService_SetArtifact`** to create or update the program on a
page. The server validates the source, stamps `wiki.artifact` metadata (version,
version_hash, library, updated_at, updated_by, automated), and enforces the
placement guard.

### Optimistic concurrency (OCC)

Pass `expected_version_hash` from a prior `GetArtifact` to guard against
concurrent edits. Omit it when writing to a brand-new page.

### Placement guard

If the page already has ≥ 1 KB of non-artifact prose, `SetArtifact` returns
`FailedPrecondition` unless you set `confirm_replace_content = true`. The safer
alternatives named in the error are:

- Create a new dedicated page for the artifact.
- Keep the app-above-prose layout (the fence goes at the top; prose below is fine).

### Example

```json
{
  "page": "my_tracker",
  "source": "root = Stack([header, btn])\nheader = Heading(\"My Tracker\", 2)\nbtn = Button(\"Reset\")",
  "expected_version_hash": "sha256:abc123"
}
```

## OpenUI Lang: component reference (wiki-core@1)

The wiki ships the **wiki-core@1** library. All programs must produce a single
root expression named `root` (the parser defaults to `Stack` as root).

### Layout

| Component | Signature | Description |
|-----------|-----------|-------------|
| `Stack` | `Stack([child, …])` | Vertical flex column |
| `Row` | `Row([child, …])` | Horizontal flex row |
| `Card` | `Card([title?], [child, …])` | Bordered container |

### Display

| Component | Signature | Description |
|-----------|-----------|-------------|
| `Heading` | `Heading("text", level?)` | h1–h6 heading |
| `Text` | `Text("content")` | Inline text span |
| `Badge` | `Badge("label")` | Small status badge |
| `Divider` | `Divider()` | Horizontal rule |

### Input

| Component | Signature | Description |
|-----------|-----------|-------------|
| `Button` | `Button("label", disabled?)` | Clickable button |
| `TextInput` | `TextInput(value?, placeholder?, label?)` | Text field |

### Language basics

```
# Variable declarations
$count = 0

# Component with reactive state
root = Stack([
  Heading("Count: " + $count, 2),
  btn
])
btn = Button("Increment", @click: $count = $count + 1)
```

- `$varName` — reactive state variable; persists server-side via ArtifactStateService.
- `@click:` — action prop; evaluates an expression when the user clicks.
- String concatenation: `"Hello, " + name`.
- Ternary: `$done ? "Done" : "Pending"`.

## ArtifactStateService — persistent state

Reactive state (`$variables`) is stored server-side, cross-device, using
ArtifactStateService. All state is a flat `map<string, string>`.

| Tool | When to use |
|------|-------------|
| `GetArtifactState` | Read current state before merging |
| `MergeArtifactState` | Shallow-patch one or more keys |
| `DeleteArtifactState` | Reset the app to its initial state |

Pass `expected_state_version_hash` from `GetArtifactState` to guard concurrent
writes (`Aborted` on mismatch). Omit for unconditional writes.

## ClearArtifact

**`api_v1_ArtifactService_ClearArtifact`** removes the fence and `wiki.artifact`
subtree, reverting the page to plain markdown. Any prose below the old fence is
preserved. Pass `expected_version_hash` for OCC, or omit to clear unconditionally.

## WatchArtifact / WatchArtifactState

Both services provide server-streaming watch methods. The browser uses them for
hot-swap (no page reload needed when you update a program). Agents do not need to
call Watch; it is for the browser host.

## For Agents

### When to create an artifact

- User asks for an interactive app, tracker, form, or dashboard.
- User mentions "build a thing that…", "make a widget…", or "I want to track…".

### Placement rules

1. **Prefer a new page** dedicated to the artifact. Name it clearly (e.g.,
   `my_tracker`, `daily_habits`).
2. **App-above-prose is fine**: put the fence at the top; prose documentation or
   instructions below it are preserved and rendered underneath the app.
3. **Never silently replace prose**: if the target page has ≥ 1 KB of non-artifact
   content, either use a new page or ask the user before setting
   `confirm_replace_content = true`.

### Workflow

```
1. GetArtifact (to read current source + version_hash, or confirm page is empty)
2. Draft the OpenUI Lang program
3. SetArtifact (with expected_version_hash from step 1)
4. Confirm success: check SetArtifactResponse.metadata for the new version
```

For state reads/writes outside the program source:

```
1. GetArtifactState (to read current state + state_version_hash)
2. MergeArtifactState (with expected_state_version_hash from step 1)
```

### Reserved namespaces

Generic frontmatter tools (`MergeFrontmatter`, `RemoveKeyAtPath`, etc.) are
**blocked** for `wiki.artifact.*` and `wiki.artifact_state.*`. Always use
ArtifactService and ArtifactStateService instead.
