package wikipage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// profileIdentifierPrefix is the single-segment prefix attached to every
// profile-page identifier. Keeping the prefix consistent lets the index pick
// out profile pages by simple key prefix without per-user enumeration.
const profileIdentifierPrefix = "profile_"

// profileIdentifierHashLen is the number of hex characters of the login's
// SHA-256 digest that we append to the sanitized login to break collisions.
// 8 hex chars = 32 bits of disambiguation, plenty for any plausible
// per-tailnet user count and short enough to keep the URL readable.
const profileIdentifierHashLen = 8

// nonAlphanumeric matches any run of characters outside [a-z0-9]. The
// transformation lowercases first, so this need not match A-Z.
var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// ProfileIdentifierFor derives the page identifier for a user's profile from
// their login name (typically an email).
//
// The identifier is single-segment, lowercased, with any run of
// non-alphanumeric characters collapsed to a single underscore, and a
// short hash suffix that disambiguates logins which would otherwise
// sanitize to the same string (e.g. user@example.com vs user.example@com).
// The `profile_` prefix is fixed so the namespace is grep-friendly and so a
// login of literally "template" cannot collide with the shipped
// `profile_template` system page. Examples:
//
//	brendanjerwin@gmail.com   → profile_brendanjerwin_gmail_com_<hash>
//	alice+tag@example.co.uk   → profile_alice_tag_example_co_uk_<hash>
//
// Returns an error when login is empty or contains no alphanumeric
// characters at all (so we never produce the bare `profile_<hash>`
// identifier).
func ProfileIdentifierFor(login string) (PageIdentifier, error) {
	if login == "" {
		return "", errors.New("ProfileIdentifierFor: login is empty")
	}
	sanitized := nonAlphanumeric.ReplaceAllString(strings.ToLower(login), "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		return "", fmt.Errorf("ProfileIdentifierFor: login %q has no alphanumeric characters", login)
	}
	digest := sha256.Sum256([]byte(login))
	hashSuffix := hex.EncodeToString(digest[:])[:profileIdentifierHashLen]
	return PageIdentifier(profileIdentifierPrefix + sanitized + "_" + hashSuffix), nil
}
