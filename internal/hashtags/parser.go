// Package hashtags provides extraction and normalization of inline #tag tokens
// from page bodies and other free text.
//
// Grammar (intentionally close to mainstream "hashtag" conventions):
//   - A tag begins with `#` only when the preceding rune is a tag boundary —
//     start-of-string, whitespace, or punctuation other than `[` and `(` (so a
//     URL fragment like `[link](#anchor)` does not match).
//   - Tag characters: Unicode letters, digits, hyphen, underscore.
//   - Escape: `\#tag` is treated as a literal `#tag` and not extracted.
//   - Code: `#tag` inside fenced code blocks (```) or inline code spans (`)
//     is not extracted.
//   - Length: tags are capped at maxTagLen runes for index hygiene.
//
// Normalization is intentionally NOT identifier munging. MungeIdentifier
// snake_cases and collapses adjacent underscores, which would silently merge
// `#home-lab` and `#home_lab`. We instead apply NFKC + lowercase only, then
// drop disallowed characters. Hyphens and underscores are preserved as written.
package hashtags

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// maxTagLen is the upper bound on a single tag's length (in runes), enforced
// during extraction. Tags longer than this are truncated and still included.
const maxTagLen = 64

// fenceMarker is the triple-backtick delimiter for a Markdown fenced code block.
const fenceMarker = "```"

// Extract returns the unique set of normalized hashtags found in body. Tags
// are returned in first-occurrence order to keep output deterministic.
func Extract(body string) []string {
	state := newExtractState([]rune(body))
	for state.i < len(state.runes) {
		state.advance()
	}
	return state.result
}

// extractState carries the iterator state for Extract. Splitting it out
// keeps the main loop short enough to satisfy revive's cognitive-complexity
// budget while leaving the grammar self-contained in one place.
type extractState struct {
	runes        []rune
	i            int
	prev         rune
	inInlineCode bool
	inFence      bool
	seen         map[string]struct{}
	result       []string
}

func newExtractState(runes []rune) *extractState {
	return &extractState{
		runes: runes,
		prev:  ' ', // start-of-string counts as a boundary
		seen:  make(map[string]struct{}),
	}
}

// advance consumes the next rune (or token) and updates state accordingly.
func (s *extractState) advance() {
	if !s.inInlineCode && atFenceMarker(s.runes, s.i, s.prev) {
		s.inFence = !s.inFence
		s.i += len(fenceMarker)
		s.prev = '`'
		return
	}

	r := s.runes[s.i]

	if s.inFence {
		s.consume(r)
		return
	}

	if r == '`' {
		s.inInlineCode = !s.inInlineCode
		s.consume(r)
		return
	}

	if s.inInlineCode {
		s.consume(r)
		return
	}

	if r == '\\' && s.i+1 < len(s.runes) && s.runes[s.i+1] == '#' {
		s.prev = s.runes[s.i+1]
		s.i += 2
		return
	}

	if r == '#' && IsTagBoundary(s.prev) && s.tryConsumeTag() {
		return
	}

	s.consume(r)
}

// tryConsumeTag reads a tag at runes[i+1:]. Returns true if a tag was
// consumed (state advanced past it); false otherwise.
func (s *extractState) tryConsumeTag() bool {
	tag, consumed := readTag(s.runes[s.i+1:])
	if tag == "" {
		return false
	}
	if normalized := Normalize(tag); normalized != "" {
		if _, dup := s.seen[normalized]; !dup {
			s.seen[normalized] = struct{}{}
			s.result = append(s.result, normalized)
		}
	}
	s.i += 1 + consumed
	if s.i < len(s.runes) {
		s.prev = s.runes[s.i-1]
	}
	return true
}

// consume advances past r without recognizing it as a tag.
func (s *extractState) consume(r rune) {
	s.prev = r
	s.i++
}

// atFenceMarker reports whether runes[i:] starts with ``` and the marker is
// at the beginning of a line (preceded by start-of-string or a newline).
func atFenceMarker(runes []rune, i int, prev rune) bool {
	if i+len(fenceMarker) > len(runes) {
		return false
	}
	if string(runes[i:i+len(fenceMarker)]) != fenceMarker {
		return false
	}
	// First fence marker on first line: prev is the synthetic ' ' boundary.
	// Subsequent markers must be preceded by a newline.
	if i == 0 {
		return true
	}
	return prev == '\n'
}

// readTag consumes characters from runes that are valid tag chars and returns
// the tag (without leading `#`) plus the number of runes consumed.
func readTag(runes []rune) (string, int) {
	end := 0
	for end < len(runes) && isTagChar(runes[end]) {
		end++
	}
	if end == 0 {
		return "", 0
	}
	return string(runes[:end]), end
}

// isTagChar reports whether r is a permissible character inside a tag body.
func isTagChar(r rune) bool {
	if r == '-' || r == '_' {
		return true
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Normalize applies the canonical form to a single raw tag (without the
// leading `#`). Result is suitable for use as an index key.
func Normalize(tag string) string {
	if tag == "" {
		return ""
	}
	folded := strings.ToLower(norm.NFKC.String(tag))

	var b strings.Builder
	b.Grow(len(folded))
	for _, r := range folded {
		if isTagChar(r) {
			_, _ = b.WriteRune(r)
		}
	}

	cleaned := b.String()
	cleanedRunes := []rune(cleaned)
	if len(cleanedRunes) > maxTagLen {
		cleanedRunes = cleanedRunes[:maxTagLen]
	}
	return string(cleanedRunes)
}

// IsTagBoundary reports whether prev is a rune that may immediately precede a
// `#` for the `#` to count as the start of a tag. Start-of-string is the
// caller's responsibility — they should pass a synthetic whitespace rune in
// that case.
func IsTagBoundary(prev rune) bool {
	if prev == '[' || prev == '(' {
		return false
	}
	if unicode.IsSpace(prev) {
		return true
	}
	if unicode.IsLetter(prev) || unicode.IsDigit(prev) {
		return false
	}
	if unicode.IsPunct(prev) || unicode.IsSymbol(prev) {
		return true
	}
	return false
}
