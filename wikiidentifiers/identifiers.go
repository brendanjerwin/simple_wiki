// Package wikiidentifiers provides identifier munging and normalization for wiki pages.
package wikiidentifiers

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/stoewer/go-strcase"
	"golang.org/x/text/unicode/norm"
)

var uuidRegex = regexp.MustCompile("[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}")

// dunderscore is the double underscore suffix preserved for backwards compatibility.
const dunderscore = "__"

// underscore is the single underscore character used for separation.
const underscore = "_"

// MungeIdentifier converts an identifier to a consistent, URL-safe format.
// It supports Unicode letters and digits from any language while sanitizing
// problematic characters.
//
// Returns an error if the input produces an empty result after sanitization.
//
// The function applies these transformations in order:
//  1. NFKC normalization (handles combining chars, stylized variants)
//  2. Remove adversarial Unicode (zero-width, bidi controls, etc.)
//  3. Replace problematic chars with underscore (punctuation, symbols, whitespace)
//  4. Apply snake_case conversion (or just lowercase for UUIDs)
//  5. Keep only allowed chars (Unicode letters, digits, underscore, hyphen)
//  6. Collapse consecutive underscores to single underscore
//  7. Restore trailing dunderscore if original input had it (backwards compat)
//  8. Trim leading underscores
//
// Invariants (returns error if violated):
//   - Result contains no adversarial Unicode
//   - Result is idempotent: MungeIdentifier(result) == result
//   - Result is URL-safe: url.PathUnescape(url.PathEscape(result)) == result
func MungeIdentifier(identifier string) (string, error) {
	result, err := mungeInternal(identifier)
	if err != nil {
		return "", err
	}

	// Invariant: result must not contain any adversarial chars
	if containsAdversarialUnicode(result) {
		return "", fmt.Errorf("internal error: result %q contains adversarial unicode after processing", result)
	}

	// Invariant: idempotency - munging again must produce same result
	remunged, err := mungeInternal(result)
	if err != nil {
		return "", fmt.Errorf("internal error: result %q produces error on re-munge: %w", result, err)
	}
	if remunged != result {
		return "", fmt.Errorf("internal error: result %q is not idempotent (remunges to %q)", result, remunged)
	}

	// Invariant: URL-safe - must round-trip through path encoding without data loss
	escaped := url.PathEscape(result)
	unescaped, err := url.PathUnescape(escaped)
	if err != nil || unescaped != result {
		return "", fmt.Errorf("internal error: result %q is not URL-safe (escapes to %q)", result, escaped)
	}

	return result, nil
}

// mungeInternal performs the actual munging without invariant checks.
// This is used by MungeIdentifier to verify idempotency without infinite recursion.
func mungeInternal(identifier string) (string, error) {
	// Check if ORIGINAL input has trailing dunderscore (2+ underscores at end)
	// We preserve this for backwards compatibility, but only if it was explicit in input
	originalHasTrailingDunderscore := hasTrailingDunderscore(identifier)

	// Check if ORIGINAL input has exactly one leading underscore followed by non-underscore
	// (e.g., "_foo" should preserve the leading underscore, but "___foo" should not)
	originalHasSingleLeadingUnderscore := len(identifier) >= 2 &&
		identifier[0] == '_' && identifier[1] != '_'

	// Check if ORIGINAL input has exactly one trailing underscore preceded by non-underscore
	// (not dunderscore which is handled separately)
	originalHasSingleTrailingUnderscore := len(identifier) >= 2 &&
		identifier[len(identifier)-1] == '_' &&
		identifier[len(identifier)-2] != '_' &&
		!originalHasTrailingDunderscore

	// 1. Apply NFKC normalization (handles combining chars, stylized variants)
	normalized := norm.NFKC.String(identifier)

	// 2. Remove adversarial Unicode characters
	cleaned := removeAdversarialUnicode(normalized)

	// 3. Replace problematic chars with underscore
	cleaned = replaceProblematicChars(cleaned)

	// 4. Apply strcase.SnakeCase (or just lowercase for UUIDs)
	var result string
	if uuidRegex.MatchString(cleaned) {
		result = strings.ToLower(cleaned)
	} else {
		result = strings.ToLower(strcase.SnakeCase(cleaned))
	}

	// 5. Keep only allowed characters (Unicode letters, digits, underscore, hyphen)
	result = keepAllowedChars(result)

	// 6. Collapse consecutive underscores to single underscore
	result = collapseUnderscores(result)

	// 7. Trim leading and trailing underscores
	result = strings.Trim(result, underscore)

	// 8. Restore single leading underscore if original input had it
	if originalHasSingleLeadingUnderscore && !strings.HasPrefix(result, underscore) {
		result = underscore + result
	}

	// 9. Restore trailing underscore/dunderscore if original input had it
	if originalHasTrailingDunderscore && !strings.HasSuffix(result, dunderscore) {
		result = result + dunderscore
	} else if originalHasSingleTrailingUnderscore && !strings.HasSuffix(result, underscore) {
		result = result + underscore
	}

	// 10. Validate non-empty
	if result == "" {
		return "", errors.New("identifier cannot be empty after sanitization")
	}

	return result, nil
}

// removeAdversarialUnicode removes characters that should never appear in identifiers.
func removeAdversarialUnicode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !isAdversarial(r) {
			_, _ = b.WriteRune(r) // WriteRune on strings.Builder never fails
		}
	}
	return b.String()
}

// isAdversarial returns true for characters that should be completely removed.
// These are invisible characters that could be used for spoofing.
// Note: Whitespace-like control chars (tab, newline, CR) are NOT adversarial -
// they get replaced with underscore in replaceProblematicChars.
func isAdversarial(r rune) bool {
	// Format characters (Cf) - includes zero-width chars, bidi controls
	if unicode.Is(unicode.Cf, r) {
		return true
	}
	// Private use (Co)
	if unicode.Is(unicode.Co, r) {
		return true
	}
	// Surrogates (Cs) - should not appear in valid Go strings but check anyway
	if unicode.Is(unicode.Cs, r) {
		return true
	}
	// Non-printable control chars (NUL, etc.) are adversarial - but NOT tab/newline/CR
	// which are common whitespace-like chars that users might paste
	if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
		return true
	}
	return false
}

// replaceProblematicChars replaces separators, punctuation, control chars, and symbols with underscore.
// It preserves Unicode letters, digits, and combining marks (for Thai, Arabic, etc.).
func replaceProblematicChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		var toWrite rune
		switch {
		case r == '_':
			toWrite = r // Keep underscore
		case r == '-':
			toWrite = r // Keep hyphen (for UUIDs)
		case unicode.IsLetter(r):
			toWrite = r // Keep all letters (any language)
		case unicode.IsDigit(r):
			toWrite = r // Keep all digits (any numeral system)
		case unicode.IsMark(r):
			toWrite = r // Keep combining marks (for Thai, Arabic, etc.)
		default:
			toWrite = '_' // Replace everything else with underscore
		}
		_, _ = b.WriteRune(toWrite) // WriteRune on strings.Builder never fails
	}
	return b.String()
}

// keepAllowedChars ensures only allowed characters remain.
// Allowed: Unicode letters, digits, combining marks, underscore, and hyphen.
func keepAllowedChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_' || r == '-' {
			_, _ = b.WriteRune(r) // WriteRune on strings.Builder never fails
		}
		// Silently drop anything else
	}
	return b.String()
}

// collapseUnderscores collapses consecutive underscores to a single underscore.
// Uses a single-pass algorithm for efficiency.
func collapseUnderscores(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	prevWasUnderscore := false
	for _, r := range s {
		if r == '_' {
			if !prevWasUnderscore {
				_, _ = b.WriteRune('_') // WriteRune on strings.Builder never fails
			}
			prevWasUnderscore = true
		} else {
			_, _ = b.WriteRune(r) // WriteRune on strings.Builder never fails
			prevWasUnderscore = false
		}
	}
	return b.String()
}

// hasTrailingDunderscore checks if string ends with 2+ underscores.
// Used to detect explicit trailing dunderscore in original input.
func hasTrailingDunderscore(s string) bool {
	if len(s) < 2 {
		return false
	}
	return s[len(s)-1] == '_' && s[len(s)-2] == '_'
}

// containsAdversarialUnicode checks if string has any adversarial chars.
// Used for invariant checking.
func containsAdversarialUnicode(s string) bool {
	for _, r := range s {
		if isAdversarial(r) {
			return true
		}
		// Also check for chars that shouldn't survive processing
		// Allow: letters, digits, marks (combining chars), underscore, hyphen
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && r != '_' && r != '-' {
			return true
		}
	}
	return false
}
