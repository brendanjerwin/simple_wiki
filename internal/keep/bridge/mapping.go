package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

const (
	sortOrderBase    = 10
	sortOrderBitSize = 64
)

// descriptionSeparator separates the head line (text + tag suffixes)
// from the trailing description in a Keep LIST_ITEM's Text field.
// Newline + em-dash + space — chosen because em-dash is vanishingly
// rare at the start of a Keep item line in natural input, so reverse-
// parsing on inbound sync is unambiguous. Tested on Keep mobile: the
// app renders the embedded newline as a wrapped second line beneath
// the checkbox, which matches the wiki-side "context line" UX.
const descriptionSeparator = "\n— "

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
func WikiToKeep(item *apiv1.ChecklistItem, parentNoteServerID, keepItemID string) protocol.Node {
	headLine := encodeTextWithTags(item.GetText(), item.GetTags())
	text := headLine
	if d := item.GetDescription(); d != "" {
		text = headLine + descriptionSeparator + d
	}

	n := protocol.Node{
		ID:             keepItemID,
		ParentID:       parentNoteServerID,
		ParentServerID: parentNoteServerID,
		Type:           protocol.NodeTypeListItem,
		Text:           text,
		Checked:        item.GetChecked(),
		SortValue:      strconv.FormatInt(item.GetSortOrder(), sortOrderBase),
	}
	if item.GetUpdatedAt() != nil {
		n.Timestamps.Updated = item.GetUpdatedAt().AsTime()
	}
	if item.GetCreatedAt() != nil {
		n.Timestamps.Created = item.GetCreatedAt().AsTime()
	}
	return n
}

// KeepToWiki converts a LIST_ITEM Keep node to a wiki ChecklistItem.
// Splits the Text on the first description separator: left side is the
// head line (text + #tag suffixes); right side is the description.
// Tags are extracted from the head line via the same hashtag parser
// the rest of the wiki uses. The wiki UID is *not* set here — caller
// assigns one for brand-new items or looks it up via the per-binding
// ItemIDMap. Returns an error rather than silently coercing a malformed
// SortValue to 0 — silent coercion would corrupt the wiki ordering
// on inbound sync without leaving any trace.
func KeepToWiki(node protocol.Node) (*apiv1.ChecklistItem, error) {
	head, description := splitDescription(node.Text)
	tags := hashtags.Extract(head)
	sortOrder, err := parseSortValue(node.SortValue)
	if err != nil {
		return nil, fmt.Errorf("keep node %q: %w", node.ServerID, err)
	}

	item := &apiv1.ChecklistItem{
		Text:      head,
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

// splitDescription returns the head line and description portion of a
// Keep LIST_ITEM Text field. If no separator is present, the entire
// input is the head line and description is empty.
func splitDescription(text string) (head, description string) {
	idx := strings.Index(text, descriptionSeparator)
	if idx < 0 {
		return text, ""
	}
	return text[:idx], text[idx+len(descriptionSeparator):]
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
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f), nil
	}
	return 0, fmt.Errorf("sortValue %q is neither an integer nor a float", s)
}

// encodeTextWithTags appends any tags not already present in text as
// trailing " #tag" markers. Already-inline #tags survive untouched.
func encodeTextWithTags(text string, tags []string) string {
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
