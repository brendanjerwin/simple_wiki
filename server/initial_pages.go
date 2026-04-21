package server

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// helpAlertsPage is the source content for the help/alerts wiki page.
// It documents the GitHub-style alert/admonition syntax supported by the wiki.
const helpAlertsPage = `# Alerts

Alerts are styled callout boxes for highlighting important information. They use GitHub-style blockquote syntax with a type marker on the first line.

## Syntax

` + "```markdown" + `
> [!NOTE]
> Useful information that users should know, even when skimming content.

> [!TIP]
> Helpful advice for doing things better or more easily.

> [!IMPORTANT]
> Key information users need to know to achieve their goal.

> [!WARNING]
> Urgent info that needs immediate user attention to avoid problems.

> [!CAUTION]
> Advises about risks or negative outcomes of certain actions.
` + "```" + `

## Rendered Examples

> [!NOTE]
> This is a note with useful information.

> [!TIP]
> Here is a helpful tip for doing things better.

> [!IMPORTANT]
> This is important information you should not miss.

> [!WARNING]
> This is a warning about something that needs immediate attention.

> [!CAUTION]
> This is a caution about potential risks.

## Notes

- The type marker (` + "`[!NOTE]`" + `, ` + "`[!TIP]`" + `, etc.) must be the first line of the blockquote.
- Type names are case-insensitive: ` + "`[!note]`" + ` and ` + "`[!NOTE]`" + ` are equivalent.
- Content follows on subsequent lines within the same blockquote block.
- Five types are supported: **NOTE**, **TIP**, **IMPORTANT**, **WARNING**, **CAUTION**.
`

// initialPages maps wiki page identifiers to their seed content.
// Pages listed here are created automatically on first server startup if they do not exist.
var initialPages = map[string]string{
	"help/alerts": helpAlertsPage,
}

// SeedInitialPages creates any missing initial wiki pages.
// It checks each page in the initialPages map and only writes content
// for pages that have never been saved to disk, preserving any user edits.
func (s *Site) SeedInitialPages() error {
	for identifier, content := range initialPages {
		page, err := s.ReadPage(wikipage.PageIdentifier(identifier))
		if err != nil {
			return fmt.Errorf("failed to check page %s: %w", identifier, err)
		}
		if page.WasLoadedFromDisk {
			continue
		}
		if err := s.UpdatePageContent(wikipage.PageIdentifier(identifier), content); err != nil {
			return fmt.Errorf("failed to seed page %s: %w", identifier, err)
		}
		s.Logger.Info("Seeded initial wiki page: %s", identifier)
	}
	return nil
}
