package wikipage

import "strings"

// IdentifierKey is the top-level frontmatter key that holds the page's stable
// identifier. Exported because both the gRPC handlers and the template-cloning
// helpers must agree on its name when excluding it from copies.
const IdentifierKey = "identifier"

// Leaf keys looked up under the reserved wiki.* namespace by the helpers
// in this file.
const (
	templateFlagKey = "template"
	systemFlagKey   = "system"
	wikiNamespace   = "wiki"
)

// IsTemplatePage reports whether fm flags the page as a template usable by
// the page-creation flow. The authoritative location is the reserved
// nested key wiki.template; the eager namespace migration moves any legacy
// top-level template flag here at startup so this is the only place the
// helper has to look.
func IsTemplatePage(fm FrontMatter) bool {
	return readReservedBoolFlag(fm, templateFlagKey)
}

// IsSystemPage reports whether fm flags the page as system-owned (i.e.
// shipped from the binary by internal/syspage and not user-editable). The
// authoritative location is the reserved nested key wiki.system; the eager
// namespace migration moves any legacy top-level system flag here at
// startup so this is the only place the helper has to look.
func IsSystemPage(fm FrontMatter) bool {
	return readReservedBoolFlag(fm, systemFlagKey)
}

// readReservedBoolFlag returns the boolean interpretation of a flag stored
// under wiki.<flagKey>. Returns false on any unparseable value or when the
// wiki subtree is missing or not a map.
func readReservedBoolFlag(fm FrontMatter, flagKey string) bool {
	if fm == nil {
		return false
	}
	wikiSubtree, ok := fm[wikiNamespace].(map[string]any)
	if !ok {
		return false
	}
	v, present := wikiSubtree[flagKey]
	if !present {
		return false
	}
	return coerceBool(v)
}

// coerceBool interprets the truthy variants TOML/JSON/YAML decoding can
// produce for a boolean flag: native bool, the string "true" (case-insensitive),
// or any non-zero numeric value. Unrecognised types yield false.
func coerceBool(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

// ApplyTemplate copies frontmatter keys from src into dest, skipping any
// key that the new page must own itself: the identifier key, and every
// reserved top-level namespace (wiki.*, agent.*).
//
// The reserved-namespace exclusion means a template marked as a template
// (wiki.template) or system page (wiki.system) does not infect a new page
// created from it; nor does a template carry over a previous owner's
// agent.* state, checklist data, etc.
//
// Existing reserved-namespace values in dest are preserved (not deleted).
// Existing non-reserved values in dest are overwritten when src has the
// same key.
func ApplyTemplate(dest, src FrontMatter) {
	if dest == nil || src == nil {
		return
	}
	for k, v := range src {
		if k == IdentifierKey {
			continue
		}
		if IsReservedTopLevelKey(k) {
			continue
		}
		dest[k] = v
	}
}
