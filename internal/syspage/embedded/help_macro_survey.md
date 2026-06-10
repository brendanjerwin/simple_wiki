+++
identifier = "help_macro_survey"
title = "help-macro-survey"

[wiki]
system = true
+++

#help #macros

# {{.Title}}

The Survey macro renders an interactive form for collecting per-user responses. Survey data is stored under the reserved `surveys.<name>` frontmatter namespace and mutated through `SurveyService`. Responses are attributed to the logged-in user (identifier provided by the configured authentication provider) and can be edited after submission (upsert pattern).

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

- **Form fields**: Renders `choice` (dropdown), `text`, `number`, and `boolean` field types based on frontmatter config
- **Pre-fill**: If the current user has already submitted, their previous response is pre-filled for editing
- **Upsert on resubmit**: Submitting again replaces the user's previous response (no duplicates)
- **Authentication required**: Unauthenticated visitors see a "please log in" message — the macro relies on the configured authentication provider's user identity (any string user identifier works)

## Stored Data Structure

Survey config and responses live under `surveys.<survey-name>`, but generic Frontmatter writes to `surveys` are rejected. Use `SurveyService` to read or mutate this data:

```toml
+++
title = "My Page"

[surveys.team-preferences]
question = "How would you like to work this week?"

[[surveys.team-preferences.fields]]
name = "location"
type = "choice"
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
| `choice` | Dropdown; requires `options` array. (Alias: `select`.) |
| `text` | Free-form text input |
| `number` | Numeric input; optional `min` / `max` |
| `boolean` | Checkbox |

### Field Labeling

- Use `label` on a field to render a custom prompt verbatim.
- If `label` is omitted, the UI auto-humanizes `name` from snake_case/kebab-case to Title Case.

Example:

```toml
[[surveys.team-preferences.fields]]
name = "protein_preference"
type = "text"
# Renders as "Protein Preference" if label is omitted

[[surveys.team-preferences.fields]]
name = "protein_notes"
type = "text"
label = "What's your preferred protein?"
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `user` | string | User identifier from the auth provider (typically email or username) |
| `anonymous` | bool | Reserved; currently always `false` in v1 |
| `submitted_at` | RFC3339 timestamp | When the response was submitted/last updated |
| `values` | object | Map of `field.name` → submitted value |

## For Agents

### Reading Survey Responses

Use `api_v1_SurveyService_GetSurvey` or `api_v1_SurveyService_ListResponses`:

```json
{ "page": "my-page", "name": "team-preferences" }
```

### Defining a New Survey

Use `api_v1_SurveyService_UpsertSurvey` to create/update the survey shell, then `AddField`, `UpdateField`, `RemoveField`, or `ReorderField` for field edits. These operations do not overwrite existing responses.

### Submitting a Response

Use `api_v1_SurveyService_SubmitResponse`. The wiki derives `user` from the caller identity and `submitted_at` from server time.

### Important Notes

- Generic Frontmatter APIs reject the reserved `surveys` namespace.
- Only one response per user: resubmissions update `submitted_at` and `values` in place.
- Authentication is required to submit; read access follows standard page visibility.
- `anonymous` is reserved for future use — in v1, all responses are attributed.
