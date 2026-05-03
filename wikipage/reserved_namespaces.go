package wikipage

// IsReservedTopLevelKey reports whether key names a reserved top-level
// frontmatter namespace.
//
// The reserved namespaces are owned by dedicated services (one per namespace)
// and must not be touched by generic frontmatter mutation paths. The
// reservation is wholesale: registering top-level key "wiki" reserves
// `wiki.*` in its entirety. See ADR-0009 (the pattern) and ADR-0010
// (the wiki.* namespace).
//
// Comparison is case-sensitive, matching the underlying TOML/YAML key
// preservation rules.
func IsReservedTopLevelKey(key string) bool {
	_, ok := reservedTopLevelKeys[key]
	return ok
}

// ReservedTopLevelKeys returns the set of reserved top-level frontmatter
// namespaces, in unspecified order. Callers that need to iterate (for
// preserving or stripping reserved subtrees) should use this rather than
// depending on the registry's storage shape.
func ReservedTopLevelKeys() []string {
	keys := make([]string, 0, len(reservedTopLevelKeys))
	for k := range reservedTopLevelKeys {
		keys = append(keys, k)
	}
	return keys
}

// reservedTopLevelKeys is the registry of reserved top-level frontmatter
// namespaces. Add a new entry here to reserve another namespace; downstream
// callers (e.g. the gRPC frontmatter handlers, the template-cloning logic)
// pick the new entry up automatically.
var reservedTopLevelKeys = map[string]struct{}{
	"agent": {},
	"wiki":  {},
}
