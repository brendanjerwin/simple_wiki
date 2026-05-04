package v1

import (
	"strings"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// utf8 byte boundary constants — mirrors the UTF-8 encoding specification.
const (
	utf8TwoByteMin  = 0x80 // first byte value that starts a multi-byte sequence
	utf8ThreeByteMin = 0xE0 // first byte value that starts a 3-byte sequence
	utf8FourByteMin  = 0xF0 // first byte value that starts a 4-byte sequence
	utf8ThreeByteLen = 3    // byte length of a 3-byte UTF-8 sequence
	utf8FourByteLen  = 4    // byte length of a 4-byte UTF-8 sequence
)

// itoaBufLen is the maximum decimal digits in a 64-bit integer.
const itoaBufLen = 20

// itoaBase is the numeric base used when converting integers to strings.
const itoaBase = 10

// maxATXHeadingLevel is the deepest ATX heading level (######).
const maxATXHeadingLevel = 6

// minFenceRunLen is the minimum number of fence characters (``` or ~~~) required.
const minFenceRunLen = 3

// maxFenceIndent is the maximum leading-space indentation allowed before a fence marker.
const maxFenceIndent = 3

// headingSlugger generates slugs that match goldmark's AutoHeadingID algorithm,
// including duplicate-suffix tracking across all headings in a document.
type headingSlugger struct {
	seen map[string]int
}

func newHeadingSlugger() *headingSlugger {
	return &headingSlugger{seen: make(map[string]int)}
}

// slug converts heading text to a kebab-case anchor ID using goldmark's algorithm:
//   - ASCII alphanumerics are lowercased and kept.
//   - Spaces, hyphens, and underscores become '-'.
//   - All other bytes (including multi-byte UTF-8) are dropped.
//   - If the result is empty, "heading" is used.
//   - Duplicate slugs get a "-N" suffix (N starting at 1).
func (s *headingSlugger) slug(text string) string {
	result := goldmarkSlugify(text)
	if count, exists := s.seen[result]; exists {
		s.seen[result] = count + 1
		return result + "-" + itoa(count+1)
	}
	s.seen[result] = 0
	return result
}

// goldmarkSlugify converts text to a slug using goldmark's algorithm (no duplicate tracking).
func goldmarkSlugify(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		c := text[i]
		l := utf8Len(c)
		i += int(l)
		if l != 1 {
			// Multi-byte UTF-8 character — goldmark skips these in slug generation.
			continue
		}
		if isASCIIAlphaNumeric(c) {
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		} else if c == ' ' || c == '-' || c == '_' {
			b.WriteByte('-') //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
		// All other single-byte characters are dropped.
	}
	result := b.String()
	if result == "" {
		return "heading"
	}
	return result
}

// utf8Len returns the number of bytes in the UTF-8 sequence beginning with b.
func utf8Len(b byte) uint8 {
	switch {
	case b < utf8TwoByteMin:
		return 1
	case b < utf8ThreeByteMin:
		return 2
	case b < utf8FourByteMin:
		return utf8ThreeByteLen
	default:
		return utf8FourByteLen
	}
}

// isASCIIAlphaNumeric reports whether b is an ASCII letter or digit.
func isASCIIAlphaNumeric(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

// itoa converts a non-negative integer to its decimal string representation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [itoaBufLen]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%itoaBase)
		n /= itoaBase
	}
	return string(buf[pos:])
}

// rawHeading is an intermediate representation of a heading found during parsing.
type rawHeading struct {
	level      int
	text       string
	lineStart  int // byte offset of the first byte of the heading line (the '#')
	lineEnd    int // byte offset immediately after the '\n' terminating the heading line (or EOF)
}

// parseHeadings parses markdown content, extracts all ATX headings (# through ######)
// that are not inside fenced code blocks, and returns them as []*apiv1.PageHeading
// with goldmark-compatible slugs and section byte spans.
//
// byte_offset is the byte position in markdown where the section body begins (after
// the heading line). byte_length spans from byte_offset to the start of the next
// heading at the same or higher level (lower heading number), or to EOF.
func parseHeadings(markdown string) []*apiv1.PageHeading {
	raws := extractRawHeadings(markdown)
	if len(raws) == 0 {
		return nil
	}

	slugger := newHeadingSlugger()
	total := int64(len(markdown))

	headings := make([]*apiv1.PageHeading, len(raws))
	for i, raw := range raws {
		bodyStart := int64(raw.lineEnd)

		// Section body ends at the next heading at the same or higher level (lower number),
		// or at EOF.
		bodyEnd := total
		for j := i + 1; j < len(raws); j++ {
			if raws[j].level <= raw.level {
				bodyEnd = int64(raws[j].lineStart)
				break
			}
		}

		headings[i] = &apiv1.PageHeading{
			Level:      int32(raw.level),
			Text:       raw.text,
			Slug:       slugger.slug(raw.text),
			ByteOffset: bodyStart,
			ByteLength: bodyEnd - bodyStart,
		}
	}

	return headings
}

// extractRawHeadings scans markdown line by line, tracking fenced code block state,
// and returns all ATX headings found outside of code fences.
func extractRawHeadings(markdown string) []rawHeading {
	var headings []rawHeading
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0

	pos := 0
	for pos < len(markdown) {
		lineStart := pos
		lineEnd := nextLineEnd(markdown, pos)

		line := markdown[pos:lineEnd]
		trimmed := strings.TrimRight(line, "\n\r")

		if fenceMatch, ch, n := isFenceMarker(trimmed); fenceMatch {
			if !inFence {
				inFence = true
				fenceChar = ch
				fenceLen = n
			} else if ch == fenceChar && n >= fenceLen {
				inFence = false
				fenceChar = 0
				fenceLen = 0
			}
		} else if !inFence {
			if level, text, ok := parseATXHeading(trimmed); ok {
				headings = append(headings, rawHeading{
					level:     level,
					text:      text,
					lineStart: lineStart,
					lineEnd:   lineEnd,
				})
			}
		}

		pos = lineEnd
	}

	return headings
}

// nextLineEnd returns the byte offset just past the end of the line starting at pos.
// The returned offset includes the '\n' if present. At EOF without a newline,
// it returns len(markdown).
func nextLineEnd(markdown string, pos int) int {
	for i := pos; i < len(markdown); i++ {
		if markdown[i] == '\n' {
			return i + 1
		}
	}
	return len(markdown)
}

// isFenceMarker reports whether line is a code-fence open/close marker.
// It returns (true, fenceChar, fenceRunLength) when it is, or (false, 0, 0) otherwise.
// The check mirrors goldmark: the line may have 0–3 spaces of indent before the fence run.
func isFenceMarker(line string) (bool, byte, int) {
	// Strip up to 3 leading spaces of indentation (standard Markdown rule).
	indent := 0
	for indent < maxFenceIndent && indent < len(line) && line[indent] == ' ' {
		indent++
	}
	stripped := line[indent:]

	if len(stripped) == 0 {
		return false, 0, 0
	}

	ch := stripped[0]
	if ch != '`' && ch != '~' {
		return false, 0, 0
	}

	n := 1
	for n < len(stripped) && stripped[n] == ch {
		n++
	}
	if n < minFenceRunLen {
		return false, 0, 0
	}

	// For backtick fences, the info string must not contain a backtick.
	if ch == '`' {
		info := stripped[n:]
		if strings.ContainsRune(info, '`') {
			return false, 0, 0
		}
	}

	return true, ch, n
}

// parseATXHeading parses an ATX heading line (e.g. "## Hello World").
// Returns (level, text, true) on success, or (0, "", false) if the line is not a heading.
// The heading marker must be followed by a space or be an empty heading (just hashes).
func parseATXHeading(line string) (int, string, bool) {
	if len(line) == 0 || line[0] != '#' {
		return 0, "", false
	}

	level := 0
	for level < len(line) && level < maxATXHeadingLevel && line[level] == '#' {
		level++
	}

	rest := line[level:]

	if len(rest) == 0 {
		// Bare "###" — valid empty heading.
		return level, "", true
	}

	if rest[0] != ' ' && rest[0] != '\t' {
		// "#word" is not a heading — the '#' run must be followed by whitespace.
		return 0, "", false
	}

	// Trim leading whitespace after the '#' run.
	text := strings.TrimLeft(rest, " \t")

	// Trim trailing '#' characters and whitespace (ATX closing sequence).
	text = strings.TrimRight(text, " \t")
	if len(text) > 0 && text[len(text)-1] == '#' {
		trimmed := strings.TrimRight(text, "#")
		trimmed = strings.TrimRight(trimmed, " \t")
		text = trimmed
	}

	return level, text, true
}
