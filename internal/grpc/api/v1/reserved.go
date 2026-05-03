package v1

import (
	"fmt"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// reservedNamespaceMessages is the gRPC-layer's user-facing rejection message
// for each reserved namespace. The keys here MUST match
// wikipage.ReservedTopLevelKeys exactly — the init() block below enforces
// that invariant at process start so a missing entry is impossible to ship.
//
// Keep the messages generic — the frontmatter package should not reach into
// any feature's vocabulary. Callers find the right dedicated service via
// `wiki-cli list` / `wiki-cli describe` or the embedded help corpus.
//
// See ADR-0009 (the pattern) and ADR-0010 (the wiki.* namespace).
var reservedNamespaceMessages = map[string]string{
	"agent": "the 'agent' top-level frontmatter namespace is reserved; use the appropriate dedicated service (see ADR-0009)",
	"wiki":  "the 'wiki' top-level frontmatter namespace is reserved; use the appropriate dedicated service (see ADR-0009 and ADR-0010)",
}

func init() {
	for _, key := range wikipage.ReservedTopLevelKeys() {
		if _, ok := reservedNamespaceMessages[key]; !ok {
			panic(fmt.Sprintf("reserved namespace %q has no rejection message in reservedNamespaceMessages; add one", key))
		}
	}
	for key := range reservedNamespaceMessages {
		if !wikipage.IsReservedTopLevelKey(key) {
			panic(fmt.Sprintf("reservedNamespaceMessages has stale entry %q not present in wikipage.ReservedTopLevelKeys", key))
		}
	}
}

// reservedKeyInMap returns the first reserved top-level key found in fm,
// or "" when fm contains no reserved keys. Callers use the returned key
// to look up the right rejection message.
func reservedKeyInMap(fm map[string]any) string {
	if fm == nil {
		return ""
	}
	for key := range fm {
		if wikipage.IsReservedTopLevelKey(key) {
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
	if wikipage.IsReservedTopLevelKey(keyComp.Key) {
		return keyComp.Key
	}
	return ""
}

// reservedNamespaceError builds the InvalidArgument response for a given
// reserved key, naming the dedicated service the caller should redirect to.
func reservedNamespaceError(key string) error {
	msg, ok := reservedNamespaceMessages[key]
	if !ok {
		// Belt-and-braces: callers should only invoke this with keys returned
		// by reservedKeyInMap/reservedKeyOnPath. The init() invariant guarantees
		// every reserved key has a message, so reaching this branch means a
		// caller passed a non-reserved key.
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
	for _, key := range wikipage.ReservedTopLevelKeys() {
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
		if wikipage.IsReservedTopLevelKey(k) {
			continue
		}
		out[k] = v
	}
	return out
}
