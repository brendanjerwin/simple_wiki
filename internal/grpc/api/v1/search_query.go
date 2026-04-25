package v1

import (
	"strings"

	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
)

// parsedSearchQuery is the result of splitting a user search query into its
// `#tag` filter portion and free-text portion. Empty fields are returned as
// nil/empty so callers can short-circuit on simple queries.
type parsedSearchQuery struct {
	// requiredTags are the normalized tag values pulled from `#tag` tokens in
	// the user input. Each must be present on a result page.
	requiredTags []string

	// freeTextTokens are the non-`#tag` tokens, used both for the body/title
	// match and for the tag-boost should-clause.
	freeTextTokens []string
}

// parseUserSearchQuery splits the user's search query into a structured form.
// `#tag` tokens become AND requirements; everything else is free text.
func parseUserSearchQuery(input string) parsedSearchQuery {
	var parsed parsedSearchQuery

	for _, token := range strings.Fields(input) {
		if isTagToken(token) {
			normalized := hashtags.Normalize(token[1:])
			if normalized != "" {
				parsed.requiredTags = appendUnique(parsed.requiredTags, normalized)
			}
			continue
		}
		// A bare `#` (no tag chars) is dropped entirely — there's nothing useful
		// to search for and including it as free-text would surface every page
		// that happens to contain a `#`.
		if token == "#" {
			continue
		}
		parsed.freeTextTokens = append(parsed.freeTextTokens, token)
	}

	return parsed
}

// isTagToken reports whether s starts with `#` followed by at least one
// character that's not whitespace.
func isTagToken(s string) bool {
	return len(s) >= 2 && s[0] == '#'
}

// appendUnique appends s to slice only when slice does not already contain s.
func appendUnique(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}
