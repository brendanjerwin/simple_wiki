package templating

// CustomElement describes a custom HTML element that a templating macro
// may emit into rendered page content. Both the goldmark sanitizer and
// the per-element pass-through tests consume this list — keep it in sync
// with the macros below.
//
// Why this lives here, not in goldmarkrenderer: the macros are the
// authoritative emitters, and changes to a macro's output should land
// next to the registry that the sanitizer reads. Drift was the original
// failure mode (KeepConnect macro shipped, bluemonday allowlist didn't
// follow, sanitizer ate the element silently). Single-sourcing here
// means a new macro author touches one list, not two.
//
// Custom elements emitted by goldmark plugins (wiki-image, wiki-hashtag,
// wiki-table, collapsible-heading) are NOT macros and stay in their
// respective packages.
type CustomElement struct {
	// Name is the HTML element name (e.g. "wiki-checklist").
	Name string
	// Attrs is the list of attribute names the element accepts. nil/empty
	// means "no attributes" (uses bluemonday's AllowNoAttrs).
	Attrs []string
}

// MacroCustomElements returns every custom element that a templating
// macro may emit. The goldmark sanitizer iterates this list to build its
// allowlist; the corresponding sanitizer pass-through tests iterate it
// to verify each element survives sanitization.
func MacroCustomElements() []CustomElement {
	return []CustomElement{
		{Name: "wiki-checklist", Attrs: []string{"list-name", "page"}},
		{Name: "wiki-survey", Attrs: []string{"name", "page"}},
		{Name: "wiki-blog", Attrs: []string{"blog-id", "max-articles", "page", "hide-new-post"}},
		{Name: "keep-connect"},
		{Name: "keep-bind-button", Attrs: []string{"page", "list-name"}},
	}
}
