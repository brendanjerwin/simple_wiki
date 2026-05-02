package translator

import "strings"

// wikiUIDMarkerPrefix is the literal byte sequence the wiki appends to
// the Tasks `notes` field to encode the wiki ULID. It begins with a
// newline so it's separated from any user-authored description, then
// uses a zero-width space (U+200B) followed by an em-dash + space so
// the line renders nearly invisibly in Google Tasks's UI but remains
// reverse-parsable.
//
// The exact form is:
//
//	"\n​— wiki:uid="
//
// Concrete tradeoff: Tasks's mobile UI does render multi-line `notes`
// as wrapped text, so a fully invisible marker is impossible — the
// zero-width prefix is the best we can do. Documented as a known
// limitation in help_google_tasks.md (per plan §3, "Outbound
// idempotence — Marker-loss robustness"): users should not manually
// delete the line.
const wikiUIDMarkerPrefix = "\n​— wiki:uid="

// WikiUIDMarker returns the literal string that should be appended to
// Tasks `notes` to encode a wiki ULID for outbound idempotence. The
// returned string starts with a newline; callers concatenate it
// directly to the user-authored description.
func WikiUIDMarker(uid string) string {
	if uid == "" {
		return ""
	}
	return wikiUIDMarkerPrefix + uid
}

// StripWikiUIDMarker removes the trailing wiki uid marker from a Tasks
// `notes` value, if present, and returns the cleaned notes plus the
// extracted uid. The third return value is true iff a marker was found
// and removed.
//
// The marker is matched only at the *trailing* position; markers in
// the middle of the notes (e.g., user pasted a wiki excerpt into the
// description) are left intact, since those don't represent the
// wiki↔Tasks identity binding.
func StripWikiUIDMarker(notes string) (cleaned string, uid string, found bool) {
	idx := strings.LastIndex(notes, wikiUIDMarkerPrefix)
	if idx < 0 {
		return notes, "", false
	}
	suffix := notes[idx+len(wikiUIDMarkerPrefix):]
	// The uid runs to end-of-string (no trailing newline). If the
	// captured uid contains an internal newline, we treat the marker
	// as not-trailing and refuse to strip — guards against a marker
	// followed by user-edited content. ULIDs are alphanumeric
	// uppercase only, so any other character indicates corruption.
	if strings.ContainsAny(suffix, "\r\n") {
		return notes, "", false
	}
	return notes[:idx], suffix, true
}
