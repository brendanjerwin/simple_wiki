+++
identifier = "help_macro_survey"
title = "help-macro-survey"

[wiki]
system = true
+++

#help #macros

# {{.Title}}

The Survey macro renders an interactive form for collecting per-user responses, stored in the page's frontmatter. Responses are attributed to the logged-in user (identifier provided by the configured authentication provider) and can be edited after submission (upsert pattern).

## Syntax

```
{{ Survey "survey-name" }}
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `survey-name` | string | Name of the survey, scoped to the current page |

### Example

```
{{ Survey "team-preferences" }}
```

A single page can have multiple surveys with different names.

## UI Features

- **Form fields**: Renders `select` and `text` field types based on frontmatter config
- **Pre-fill**: If the current user has already submitted, their previous response is pre-filled for editing
- **Upsert on resubmit**: Submitting again replaces the user's previous response (no duplicates)
- **Authentication required**: Unauthenticated visitors see a "please log in" message — the macro relies on the configured authentication provider's user identity (any string user identifier works)

## Frontmatter Data Structure

Survey config and responses live under `surveys.<survey-name>`:

```toml
+++
title = "My Page"

[surveys.team-preferences]
question = "How would you like to work this week?"

[[surveys.team-preferences.fields]]
name = "location"
type = "select"
label = "Where will you be?"
options = ["office", "home", "hybrid"]

[[surveys.team-preferences.fields]]
name = "notes"
type = "text"
label = "Anything else?"

[[surveys.team-preferences.responses]]
user = "alice@example.com"
anonymous = false
submitted_at = "2026-04-17T12:00:00Z"

[surveys.team-preferences.responses.values]
location = "home"
notes = "Heads-down sprint."
+++
```

### Field Types

| Type | Description |
|------|-------------|
| `select` | Dropdown; requires `options` array |
| `text` | Free-form text input |

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `user` | string | User identifier from the auth provider (typically email or username) |
| `anonymous` | bool | Reserved; currently always `false` in v1 |
| `submitted_at` | RFC3339 timestamp | When the response was submitted/last updated |
| `values` | object | Map of `field.name` → submitted value |

## For Agents

### Reading Survey Responses

Use `api_v1_Frontmatter_GetFrontmatter`:

```json
{ "page_name": "my-page" }
```

Then extract `surveys.<name>.responses` from the response.

### Defining a New Survey

Write the full `surveys.<name>` subtree via `MergeFrontmatter`. The subtree is replaced wholesale, so include `question`, `fields`, and `responses` (even if empty) in the same call.

### Important Notes

- `MergeFrontmatter` replaces the entire `surveys.<name>` subtree — include existing responses when editing config, or they'll be wiped.
- Only one response per user: resubmissions update `submitted_at` and `values` in place.
- Authentication is required to submit; read access follows standard page visibility.
- `anonymous` is reserved for future use — in v1, all responses are attributed.
