package bridge

import (
	"strconv"

	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

const (
	sortOrderBase    = 10
	sortOrderBitSize = 64
)

// WikiToKeep converts a wiki ChecklistItem to a Keep ListItem Node ready
// to be sent on the changes endpoint. parentNoteID is the bound Keep
// note's serverId; keepItemID is the per-binding mapping (empty for
// brand-new items — the server assigns an ID and echoes it back).
//
// Tags ride inside the text as #tag markers (mirrors the CalDAV bridge's
// strategy from #983; same parser via internal/hashtags). Description,
// due, and alarm are wiki-only — Keep ListItem has no equivalent fields.
func WikiToKeep(item *apiv1.ChecklistItem, parentNoteID, keepItemID string) protocol.Node {
	text := encodeTextWithTags(item.GetText(), item.GetTags())

	n := protocol.Node{
		ID:        keepItemID,
		ParentID:  parentNoteID,
		Type:      protocol.NodeTypeListItem,
		Text:      text,
		Checked:   item.GetChecked(),
		SortValue: strconv.FormatInt(item.GetSortOrder(), sortOrderBase),
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
// Tags are extracted from the text via the same hashtag parser the rest
// of the wiki uses. The wiki UID is *not* set here — caller assigns one
// when adding a brand-new item or looks one up via the per-binding
// id_map.
func KeepToWiki(node protocol.Node) *apiv1.ChecklistItem {
	tags := hashtags.Extract(node.Text)
	sortOrder, _ := strconv.ParseInt(node.SortValue, sortOrderBase, sortOrderBitSize)

	item := &apiv1.ChecklistItem{
		Text:      node.Text,
		Checked:   node.Checked,
		Tags:      tags,
		SortOrder: sortOrder,
	}
	if !node.Timestamps.Created.IsZero() {
		item.CreatedAt = timestamppb.New(node.Timestamps.Created)
	}
	if !node.Timestamps.Updated.IsZero() {
		item.UpdatedAt = timestamppb.New(node.Timestamps.Updated)
	}
	return item
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
