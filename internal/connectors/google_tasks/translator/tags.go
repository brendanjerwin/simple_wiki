package translator

import (
	"strings"

	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
)

// EncodeTitleWithTags appends any tags not already present in title as
// trailing " #tag" markers. Already-inline #tags survive untouched.
//
// Mirrors `internal/connectors/google_keep/translator.EncodeTextWithTags`
// — the convention is identical across both connectors so the same
// inline-tag round-trip works regardless of which side a checklist
// happens to be subscribed to.
func EncodeTitleWithTags(title string, tags []string) string {
	if len(tags) == 0 {
		return title
	}
	existing := make(map[string]struct{}, len(tags))
	for _, t := range hashtags.Extract(title) {
		existing[t] = struct{}{}
	}
	out := title
	for _, t := range tags {
		normalized := hashtags.Normalize(t)
		if _, ok := existing[normalized]; ok {
			continue
		}
		out += " #" + t
	}
	return out
}

// TitleAndTagsFromText extracts whitespace-delimited "#tag" tokens from
// a Tasks `title` value, returning the cleaned title (with the tag
// tokens stripped) and the normalized tag list.
//
// The inverse of EncodeTitleWithTags. Round-trip:
//
//	EncodeTitleWithTags("Buy milk", []string{"urgent"})
//	  → "Buy milk #urgent"
//	TitleAndTagsFromText("Buy milk #urgent")
//	  → ("Buy milk", []string{"urgent"})
//
// Tag extraction uses `internal/hashtags` so the rules match the rest
// of the wiki (escapes, code spans, etc. — none of which should appear
// in a Tasks title in practice, but the shared parser keeps behavior
// uniform).
func TitleAndTagsFromText(text string) (title string, tags []string) {
	tags = hashtags.Extract(text)
	title = stripHashtagTokens(text)
	return title, tags
}

// stripHashtagTokens removes whitespace-delimited "#tag" tokens from
// text and collapses the surrounding whitespace. Mirrors the wiki's
// hashtag convention: #-prefixed tokens are always tags, never literal
// content.
func stripHashtagTokens(s string) string {
	fields := strings.Fields(s)
	cleaned := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f, "#") {
			continue
		}
		cleaned = append(cleaned, f)
	}
	return strings.Join(cleaned, " ")
}
