# Google Tasks setup screenshots

Screenshots referenced from `docs/google_tasks_setup.md` live in this directory.

## Filename convention — capture date is part of the filename

**Every screenshot filename MUST include the capture date in `YYYY-MM-DD` format**, e.g. `2026-05-02-gcp-project-creation.png`.

The reason is staleness pressure: the GCP Console UI changes regularly, and a screenshot taken in 2024 may bear no resemblance to what the operator sees in 2026. Embedding the date in the filename means anyone glancing at `git log`, the file tree, or a markdown render can immediately judge whether the visual is current.

### When you update a screenshot

1. **Take the new screenshot.**
2. **Save it under the new dated filename**, e.g. `2026-05-02-gcp-project-creation.png` becomes `2027-01-15-gcp-project-creation.png`.
3. **Update the markdown reference** in `docs/google_tasks_setup.md` to point at the new filename.
4. **Delete the old file** in the same commit.

**Do NOT overwrite a stale filename in place.** If you replace the bytes of `2026-05-02-gcp-project-creation.png` with a 2027 screenshot, the filename now lies, and the staleness signal is gone. The whole point of dated filenames is that the filename itself carries the truth — preserve that invariant.

### Filename format rules

- Date prefix: `YYYY-MM-DD-` (ISO 8601 date, hyphen-separated, hyphen-suffixed).
- Slug: kebab-case description of what's depicted, e.g. `gcp-project-creation`, `oauth-consent-scope-picker`, `credentials-redirect-uri-form`.
- Extension: `.png` preferred (lossless; UI screenshots have hard edges that JPEG smears).

Full example: `2026-05-02-gcp-project-creation.png`.

## What screenshots are needed

The `docs/google_tasks_setup.md` markdown contains placeholder image references with `<!-- TODO: capture screenshot -->` comments. Search for those to find the list of pending captures. Each placeholder names the exact filename to use; following the dated-filename convention is required for the placeholder to resolve correctly once the image is added.
