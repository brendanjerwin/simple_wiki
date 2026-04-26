package wikipage

import "strings"

// NormalizeListName returns a checklist name safe for use anywhere
// the wiki exposes the name as a URL path component — most importantly
// CalDAV hrefs. The transformation maps each "URL-unsafe" character
// to a single hyphen and collapses runs of hyphens to one.
//
// The unsafe set is conservative: characters that either change URL
// path structure ("/", "\") or are reserved by RFC 3986 with no
// reasonable percent-encoding round-trip in the wild ("?", "#", "%").
// Whitespace and control characters also collapse to "-" so a name
// like "groceries  /  household" produces "groceries-household".
//
// The mapping is one-way and intentionally lossy. Two distinct names
// like "a/b" and "a-b" both normalize to "a-b"; callers that need to
// store both must reject the second at write time. Callers that look
// up a list by name SHOULD pass the input through NormalizeListName
// before any storage lookup so user-typed forms (slashes, mixed
// whitespace) match the normalized form on disk.
func NormalizeListName(name string) string {
	out := make([]rune, 0, len(name))
	prevHyphen := true // suppress leading hyphens
	for _, r := range name {
		if isListNameUnsafe(r) {
			if !prevHyphen {
				out = append(out, '-')
				prevHyphen = true
			}
			continue
		}
		out = append(out, r)
		prevHyphen = false
	}
	// Strip trailing hyphen produced by an unsafe trailing char.
	return strings.TrimRight(string(out), "-")
}

// isListNameUnsafe reports whether a rune is on the list of characters
// NormalizeListName replaces with "-". Centralized so the rule lives
// in one place; tests pin the exact set.
func isListNameUnsafe(r rune) bool {
	switch r {
	case '/', '\\', '?', '#', '%':
		return true
	default:
		return r <= ' ' // ASCII control + space
	}
}
