package wikipage

import (
	"fmt"
	"regexp"
	"strings"
)

// profileIdentifierPrefix is the single-segment prefix attached to every
// profile-page identifier. Keeping the prefix consistent lets the index pick
// out profile pages by simple key prefix without per-user enumeration.
const profileIdentifierPrefix = "profile_"

// nonAlphanumeric matches any run of characters outside [a-z0-9]. The
// transformation lowercases first, so this need not match A-Z.
var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// ProfileIdentifierFor derives the page identifier for a user's profile from
// their login name (typically an email).
//
// The identifier is single-segment and stable: lowercased, with any run of
// non-alphanumeric characters collapsed to a single underscore, and the
// `profile_` prefix attached. Examples:
//
//	brendanjerwin@gmail.com   → profile_brendanjerwin_gmail_com
//	alice+tag@example.co.uk   → profile_alice_tag_example_co_uk
//
// Returns an error when login is empty or contains no alphanumeric
// characters at all (so we never produce the bare `profile_` identifier
// or one that ends up colliding with the prefix).
func ProfileIdentifierFor(login string) (PageIdentifier, error) {
	if login == "" {
		return "", fmt.Errorf("ProfileIdentifierFor: login is empty")
	}
	sanitized := nonAlphanumeric.ReplaceAllString(strings.ToLower(login), "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		return "", fmt.Errorf("ProfileIdentifierFor: login %q has no alphanumeric characters", login)
	}
	return PageIdentifier(profileIdentifierPrefix + sanitized), nil
}
