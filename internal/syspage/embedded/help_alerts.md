+++
identifier = "help_alerts"
title = "help-alerts"
system = true
+++

#help

# Alerts

Alerts are styled callout boxes for highlighting important content. They use GitHub-flavored blockquote syntax — a blockquote whose first line is `[!TYPE]` renders as a colored box with an icon and label.

## Syntax

```
> [!NOTE]
> Your content here.
```

### Alert Types

| Type | Appearance | Use for |
|------|-----------|---------|
| `[!NOTE]` | Blue | Supplementary information worth knowing |
| `[!TIP]` | Green | Helpful advice or shortcuts |
| `[!IMPORTANT]` | Purple | Key information readers shouldn't miss |
| `[!WARNING]` | Yellow | Potentially harmful consequences |
| `[!CAUTION]` | Red | Negative outcomes or dangerous actions |

### Example

```
> [!NOTE]
> Alerts work in any page body — no special mode required.

> [!TIP]
> You can have multiple paragraphs inside an alert by adding a blank `>` line between them.

> [!IMPORTANT]
> The `[!TYPE]` marker must be on its own line. Writing `[!NOTE] extra text` on the same line won't trigger alert rendering.

> [!WARNING]
> Unknown types like `[!UNKNOWN]` fall back silently to a plain blockquote.

> [!CAUTION]
> This action cannot be undone.
```

### Multi-Paragraph Alerts

```
> [!NOTE]
> First paragraph of the note.
>
> Second paragraph — add a blank `>` line to separate.
```

## Rules

- The `[!TYPE]` marker must be the **only content on the first line** of the blockquote
- Type names are case-sensitive: `[!NOTE]` works; `[!note]` does not
- Unknown type markers silently fall back to a plain `<blockquote>` — no error
- Alerts support full Markdown inside: bold, code, links, lists

## Accessibility

Each alert renders with `role="note"` on its container. The icon is marked `aria-hidden="true"` so screen readers skip the decorative glyph and read the content directly.
