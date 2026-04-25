package v1

import (
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// reservedNamespaces lists the top-level frontmatter keys that generic
// Frontmatter writes (Merge/Replace/RemoveKeyAtPath) must reject. New
// reservations land here; `frontmatter.go` consults this registry rather
// than hardcoding individual key names.
//
// Each entry's value is the InvalidArgument error message returned to a
// caller that tries to mutate the namespace via the generic API. The
// message names the dedicated service the caller should use instead.
//
// The reservation is wholesale: registering top-level key "wiki" reserves
// `wiki.*` in its entirety. Future occupants under a registered key
// inherit the reservation without code changes.
//
// See ADR-0009 (the pattern) and ADR-0010 (the wiki.* namespace).
var reservedNamespaces = map[string]string{
	"agent": "the 'agent' top-level frontmatter namespace is reserved; use AgentMetadataService instead",
}

// isReservedTopLevel reports whether key is a reserved top-level frontmatter
// key.
func isReservedTopLevel(key string) bool {
	_, ok := reservedNamespaces[key]
	return ok
}

// reservedKeyInMap returns the first reserved top-level key found in fm,
// or "" when fm contains no reserved keys. Callers use the returned key
// to look up the right rejection message.
func reservedKeyInMap(fm map[string]any) string {
	if fm == nil {
		return ""
	}
	for key := range fm {
		if isReservedTopLevel(key) {
			return key
		}
	}
	return ""
}

// reservedKeyOnPath returns the reserved top-level key when the path's
// first component targets a reserved key, or "" otherwise. Used by
// RemoveKeyAtPath to reject removals targeting reserved.* descendants.
func reservedKeyOnPath(path []*apiv1.PathComponent) string {
	if len(path) == 0 {
		return ""
	}
	keyComp, ok := path[0].Component.(*apiv1.PathComponent_Key)
	if !ok {
		return ""
	}
	if isReservedTopLevel(keyComp.Key) {
		return keyComp.Key
	}
	return ""
}

// reservedNamespaceError builds the InvalidArgument response for a given
// reserved key, naming the dedicated service the caller should redirect to.
func reservedNamespaceError(key string) error {
	msg, ok := reservedNamespaces[key]
	if !ok {
		// Belt-and-braces: callers should only invoke this with keys returned
		// by isReservedTopLevel/reservedKeyInMap/reservedKeyOnPath.
		return status.Errorf(codes.InvalidArgument, "the '%s' top-level frontmatter namespace is reserved", key)
	}
	return status.Error(codes.InvalidArgument, msg)
}

// preserveReservedSubtrees carries every reserved subtree from existing
// into incoming so a ReplaceFrontmatter call that omits a reserved key
// does not silently destroy its contents. Callers must already have
// rejected payloads that include any reserved key (via reservedKeyInMap +
// reservedNamespaceError) — this function only handles the carry-forward
// of subtrees the caller did not mention.
func preserveReservedSubtrees(existing, incoming map[string]any) {
	if existing == nil || incoming == nil {
		return
	}
	for key := range reservedNamespaces {
		if existingValue, ok := existing[key]; ok {
			incoming[key] = existingValue
		}
	}
}

// stripReservedKeys returns a copy of fm with all reserved top-level keys
// removed. Used by GetFrontmatter responses (and read-back paths in
// Merge/Replace) so generic frontmatter editors don't expose reserved
// subtrees that they can't write back.
func stripReservedKeys(fm map[string]any) map[string]any {
	if fm == nil {
		return nil
	}
	out := make(map[string]any, len(fm))
	for k, v := range fm {
		if isReservedTopLevel(k) {
			continue
		}
		out[k] = v
	}
	return out
}
