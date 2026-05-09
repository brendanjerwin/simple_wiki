// Bind-time seed translations. When subscribing a checklist to an
// existing Keep list, the connector pulls the LIST's current contents
// and reconciles wiki items against existing Keep nodes by base-text
// match. The orchestration (the actual `client.Changes` round trip,
// persistence) lives in sync; this file owns the named transformations
// that operate on the pull response: indexing, normalizing, matching.

package translator

import (
	"strings"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
)

// NormalizeForSeedMatch reduces a LIST_ITEM text to its bare item-name
// portion: strips any "\n— description" suffix and any inline " #tag"
// markers, lowercases, and trims surrounding whitespace. Used only at
// bind time to find loose matches between wiki items (typically tagged
// + described) and existing Keep items (typically plain text).
func NormalizeForSeedMatch(text string) string {
	head, _, _ := strings.Cut(text, DescriptionSeparator)
	// Walk word-by-word; drop tokens that start with '#'.
	fields := strings.Fields(head)
	cleaned := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f, "#") {
			continue
		}
		cleaned = append(cleaned, f)
	}
	return strings.ToLower(strings.Join(cleaned, " "))
}

// SeedWikiItem is the shape MatchWikiItemsToKeepNodes accepts on the
// wiki side. Plain UID + text; the seed match doesn't care about
// checked, tags, description, or sort order — those round-trip cleanly
// through the id_map once the pairing is established.
type SeedWikiItem struct {
	UID  string
	Text string
}

// SeedMatch is the per-pair result of matching a wiki item to a Keep
// node at bind time. ServerID is Keep's stable id; BaseVersion is the
// optimistic-concurrency token Keep requires on subsequent edits;
// ClientID is Keep's client-generated id (the `id` field, distinct
// from `serverId`). All three are required by Keep's outbound push
// protocol on the first post-bind sync.
type SeedMatch struct {
	ServerID    string
	BaseVersion string
	ClientID    string
}

// MatchWikiItemsToKeepNodes pairs each wiki item with a live Keep
// LIST_ITEM under the given list serverID using normalized base-text
// matching. Returns a per-wiki-uid map of SeedMatch values populated
// with the matched Keep node's ServerID, BaseVersion, and ClientID.
//
// Match strategy (pure): index live LIST_ITEMs whose parent matches
// listServerID by NormalizeForSeedMatch'd text, then walk wiki items
// and look each up in the index. Duplicate-base entries on either
// side use first-wins semantics — first wiki item with a given
// normalized text claims the index entry; subsequent wiki items with
// the same normalized text get no match.
//
// Excluded from the match index: nodes that aren't LIST_ITEM, nodes
// whose parent doesn't match listServerID (other lists in the user's
// account), trashed/deleted nodes, and nodes whose normalized text is
// empty.
func MatchWikiItemsToKeepNodes(nodes []gateway.Node, listServerID string, wikiItems []SeedWikiItem) map[string]SeedMatch {
	if listServerID == "" {
		return map[string]SeedMatch{}
	}
	// Build base-text → Keep node index for live LIST_ITEMs whose
	// parent matches our bound list. Retain the full node so the
	// caller can capture BaseVersion and ClientID — both required by
	// Keep's outbound push protocol on subsequent syncs.
	keepByBase := make(map[string]gateway.Node, len(nodes))
	for _, n := range nodes {
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		if n.ParentID != listServerID && n.ParentServerID != listServerID {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		base := NormalizeForSeedMatch(n.Text)
		if base == "" {
			continue
		}
		if _, exists := keepByBase[base]; exists {
			// Duplicate-base Keep items: first match wins; second
			// will look fresh to the sync engine and get a new
			// client_id. User can manually clean up.
			continue
		}
		keepByBase[base] = n
	}
	matches := make(map[string]SeedMatch, len(wikiItems))
	for _, w := range wikiItems {
		if w.UID == "" {
			continue
		}
		base := NormalizeForSeedMatch(w.Text)
		if base == "" {
			continue
		}
		if node, ok := keepByBase[base]; ok {
			matches[w.UID] = SeedMatch{
				ServerID:    node.ServerID,
				BaseVersion: node.BaseVersion,
				ClientID:    node.ID,
			}
		}
	}
	return matches
}

// FindListClientID locates the LIST node carrying the given listServerID
// and returns its client-generated stable id (the `id` field, distinct
// from `serverId`). Returns "" if no matching LIST node is present in
// the pull. Captured at bind time so subsequent outbound LIST updates
// send `id != serverId` — Keep returns stage3 HTTP 500 "Unknown Error"
// when the two are equal.
func FindListClientID(nodes []gateway.Node, listServerID string) string {
	for _, n := range nodes {
		if n.Type == gateway.NodeTypeList && n.ServerID == listServerID {
			return n.ID
		}
	}
	return ""
}

// IndexLabelsByName captures a per-name → MainID map from a pull's
// Labels slice. Skips tombstoned labels (Deleted != zero) and entries
// missing either Name or MainID. Used to seed the binding's persistent
// label map at bind time so MergeKeepLabels on the first post-bind sync
// uses persisted FKs instead of emitting fresh label CRUD entries every
// tick.
func IndexLabelsByName(labels []gateway.LabelEntry) map[string]string {
	out := make(map[string]string, len(labels))
	for _, l := range labels {
		if l.Name == "" || l.MainID == "" {
			continue
		}
		if !l.Deleted.IsZero() {
			continue
		}
		out[l.Name] = l.MainID
	}
	return out
}
