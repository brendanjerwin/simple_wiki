// Package translator is the Anti-Corruption Layer (DDD sense) between
// the wiki's ChecklistItem domain model and Google Keep's wire-protocol
// node shape. The functions here are pure: no I/O, no clock, no
// orchestrator state — fed a wiki item or a Keep node, they return the
// other shape.
//
// The sibling sync package owns orchestration (Connector, BindingStore,
// debounce, cron, jobs); the sibling gateway package owns the wire
// protocol. Translation never reaches across either of those boundaries
// and never holds connector state, which is why it sits in its own
// package and not on a struct method.
package translator

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
)

const (
	sortOrderBase    = 10
	sortOrderBitSize = 64
)

// DescriptionSeparator separates the head line (text + tag suffixes)
// from the trailing description in a Keep LIST_ITEM's Text field.
// Newline + em-dash + space — chosen because em-dash is vanishingly
// rare at the start of a Keep item line in natural input, so reverse-
// parsing on inbound sync is unambiguous. Tested on Keep mobile: the
// app renders the embedded newline as a wrapped second line beneath
// the checkbox, which matches the wiki-side "context line" UX.
const DescriptionSeparator = "\n— "

// WikiToKeep converts a wiki ChecklistItem to a Keep ListItem Node ready
// to be sent on the changes endpoint. parentNoteServerID is the bound
// Keep note's serverId (used to set both ParentID and ParentServerID,
// since Keep's incremental-edit protocol requires both); keepItemID is
// the per-binding ItemIDMap value for an existing item, or empty when
// pushing a brand-new item (the server assigns an ID and echoes it back).
//
// Tags ride inline inside the head line as #tag markers (mirrors the
// CalDAV bridge's strategy from #983; same parser via internal/hashtags).
// Description rides as a trailing "\n— <description>" suffix on the
// same Text field. Due and alarm are wiki-only — Keep LIST_ITEM has no
// equivalent.
func WikiToKeep(item *apiv1.ChecklistItem, parentNoteServerID, keepItemID string) gateway.Node {
	headLine := EncodeTextWithTags(item.GetText(), item.GetTags())
	text := headLine
	if d := item.GetDescription(); d != "" {
		text = headLine + DescriptionSeparator + d
	}

	n := gateway.Node{
		ID:             keepItemID,
		ParentID:       parentNoteServerID,
		ParentServerID: parentNoteServerID,
		Type:           gateway.NodeTypeListItem,
		Text:           text,
		Checked:        item.GetChecked(),
		SortValue:      strconv.FormatInt(item.GetSortOrder(), sortOrderBase),
	}
	// For an existing item (keepItemID is a server-side id), also
	// populate ServerID so the wire shape is {id, serverId} like
	// gkeepapi emits. Keep's backend reportedly 500s on update-style
	// nodes that have id-set-to-server-id but no serverId field.
	// Brand-new pushes leave keepItemID empty; the caller assigns a
	// fresh client_id and ServerID stays empty (server fills it in
	// the response).
	if keepItemID != "" {
		n.ServerID = keepItemID
	}
	// Timestamps: Created is only set for brand-new items (gkeepapi
	// touch() never re-stamps created on updates; sending an older
	// `created` than the server's known value triggers stage3 HTTP
	// 500 "Unknown Error"). Updated is always stamped — the caller
	// passes the sync-time clock — to mirror gkeepapi touch() which
	// sets timestamps.updated = now() on every dirty mutation.
	// Caller is responsible for passing clock-now via a pre-step;
	// here we read the wiki item's UpdatedAt as the input but the
	// connector's SyncToKeep overrides it to clock.Now() for pushes.
	if item.GetUpdatedAt() != nil {
		n.Timestamps.Updated = item.GetUpdatedAt().AsTime()
	}
	if keepItemID == "" && item.GetCreatedAt() != nil {
		n.Timestamps.Created = item.GetCreatedAt().AsTime()
	}
	return n
}

// KeepToWiki converts a LIST_ITEM Keep node to a wiki ChecklistItem.
// Splits the Text on the first description separator: left side is the
// head line (text + #tag suffixes); right side is the description.
// Tags are extracted from the head line via the same hashtag parser
// the rest of the wiki uses, then the #tag tokens are STRIPPED from
// Text so the wiki side stores clean text + tags array (round-trip
// stable: clean text → encode → text+#tags → push → pull → strip →
// clean text again, byte-identical to the wiki's original input).
//
// The wiki UID is *not* set here — caller assigns one for brand-new
// items or looks it up via the per-binding ItemIDMap. Returns an
// error rather than silently coercing a malformed SortValue to 0 —
// silent coercion would corrupt the wiki ordering on inbound sync
// without leaving any trace.
func KeepToWiki(node gateway.Node) (*apiv1.ChecklistItem, error) {
	head, description := splitDescription(node.Text)
	tags := hashtags.Extract(head)
	cleanText := stripHashtagTokens(head)
	sortOrder, err := parseSortValue(node.SortValue)
	if err != nil {
		return nil, fmt.Errorf("keep node %q: %w", node.ServerID, err)
	}

	item := &apiv1.ChecklistItem{
		Text:      cleanText,
		Checked:   node.Checked,
		Tags:      tags,
		SortOrder: sortOrder,
	}
	if description != "" {
		d := description
		item.Description = &d
	}
	if !node.Timestamps.Created.IsZero() {
		item.CreatedAt = timestamppb.New(node.Timestamps.Created)
	}
	if !node.Timestamps.Updated.IsZero() {
		item.UpdatedAt = timestamppb.New(node.Timestamps.Updated)
	}
	return item, nil
}

// Fingerprint is the per-item content fingerprint used by the
// divergence-rule sync engine. Two of these are equal iff the wiki
// item and Keep node have the same canonical text, the same checked
// flag, and the same SortValue. Used as the "merge-base" by the
// engine's causal divergence classifier (ADR-0015 op-log semantics).
//
// The text component is canonicalized via the same encoder
// `WikiToKeep` uses on the way out, so a wiki item and the Keep node
// the wiki most recently pushed produce identical fingerprints. That
// equality is what closes the "no clock anywhere" property in the
// sync model: divergence is always content-equality, never wall-time.
type Fingerprint struct {
	Text      string
	Checked   bool
	SortValue string
}

// FingerprintWiki returns the canonical fingerprint for a wiki item.
// Mirrors the encoding WikiToKeep performs (head + #tags + "\n— description")
// so a wiki item and its post-push Keep node fingerprint identically.
func FingerprintWiki(item *apiv1.ChecklistItem) Fingerprint {
	if item == nil {
		return Fingerprint{}
	}
	headLine := EncodeTextWithTags(item.GetText(), item.GetTags())
	text := headLine
	if d := item.GetDescription(); d != "" {
		text = headLine + DescriptionSeparator + d
	}
	return Fingerprint{
		Text:      text,
		Checked:   item.GetChecked(),
		SortValue: strconv.FormatInt(item.GetSortOrder(), sortOrderBase),
	}
}

// FingerprintKeep returns the fingerprint for a Keep LIST_ITEM node.
// Reads exactly the three fields the divergence rule cares about; no
// timestamp comparison.
func FingerprintKeep(node gateway.Node) Fingerprint {
	return Fingerprint{
		Text:      node.Text,
		Checked:   node.Checked,
		SortValue: node.SortValue,
	}
}

// FingerprintFromSyncedFields returns the synced-baseline fingerprint
// from the three persisted fields of an item binding. Used by the
// divergence rule to test whether wiki_fp or keep_fp has diverged from
// the last successful sync.
//
// Takes three primitives (not the binding struct) so this package
// stays free of any sync-layer types — translator must not depend on
// sync's types or the package boundary becomes a fiction.
func FingerprintFromSyncedFields(syncedText string, syncedChecked bool, syncedSortValue string) Fingerprint {
	return Fingerprint{
		Text:      syncedText,
		Checked:   syncedChecked,
		SortValue: syncedSortValue,
	}
}

// stripHashtagTokens removes whitespace-delimited "#tag" tokens from
// text and collapses the surrounding whitespace. Mirrors the wiki's
// hashtag convention: #-prefixed tokens are always tags, never literal
// content. Round-trip pair with EncodeTextWithTags:
//
//	encode("Buy apples", []string{"fresh"})       → "Buy apples #fresh"
//	stripHashtagTokens("Buy apples #fresh")        → "Buy apples"
func stripHashtagTokens(s string) string {
	fields := strings.Fields(s)
	cleaned := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f, "#") {
			continue
		}
		cleaned = append(cleaned, f)
	}
	return strings.Join(cleaned, " ")
}

// splitDescription returns the head line and description portion of a
// Keep LIST_ITEM Text field. If no separator is present, the entire
// input is the head line and description is empty.
func splitDescription(text string) (head, description string) {
	idx := strings.Index(text, DescriptionSeparator)
	if idx < 0 {
		return text, ""
	}
	return text[:idx], text[idx+len(DescriptionSeparator):]
}

// parseSortValue parses Keep's SortValue field into the wiki's int64
// SortOrder. Empty input is OK (zero, no error — matches "absent").
// Otherwise integer first, then float fallback (Keep occasionally
// writes float-style values; truncate to int64).
func parseSortValue(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	if n, err := strconv.ParseInt(s, sortOrderBase, sortOrderBitSize); err == nil {
		return n, nil
	}
	if f, err := strconv.ParseFloat(s, sortOrderBitSize); err == nil {
		return int64(f), nil
	}
	return 0, fmt.Errorf("sortValue %q is neither an integer nor a float", s)
}

// EncodeTextWithTags appends any tags not already present in text as
// trailing " #tag" markers. Already-inline #tags survive untouched.
func EncodeTextWithTags(text string, tags []string) string {
	if len(tags) == 0 {
		return text
	}
	existing := make(map[string]struct{}, len(tags))
	for _, t := range hashtags.Extract(text) {
		existing[t] = struct{}{}
	}
	out := text
	for _, t := range tags {
		normalized := hashtags.Normalize(t)
		if _, ok := existing[normalized]; ok {
			continue
		}
		out += " #" + t
	}
	return out
}
