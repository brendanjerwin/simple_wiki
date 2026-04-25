package checklistmutator

import (
	"sort"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// User-facing checklist data lives at fm["checklists"][listName].
//   - items: []map (each with text, checked, tags, sort_order, etc.)
// Wiki-managed metadata lives at fm["wiki"]["checklists"][listName].
//   - sync_token: int
//   - items: map[uid]map (each with created_at, updated_at, completed_at,
//     completed_by, automated)
//   - tombstones: []map (uid, deleted_at, gc_after)
//   - migrated_data_model: bool (set by the eager migration in slice 9)
//
// decodeChecklist reads both subtrees and returns a unified proto Checklist.
// Items missing a uid are assigned synthetic ULIDs in-memory (the caller
// must persist to make them durable).
//
// encodeChecklist is the inverse — it splits the proto Checklist back into
// the two-tier frontmatter shape and writes the result into fm.

const (
	userChecklistsKey = "checklists"
	wikiKey           = "wiki"
	itemsKey          = "items"
	tombstonesKey     = "tombstones"
	syncTokenKey      = "sync_token"
	updatedAtKey      = "updated_at"
	uidKey            = "uid"
)

// decodeChecklist reads the named checklist from fm. Missing user data
// returns an empty Checklist (the caller will populate it). Missing
// metadata is allowed — items synthesize defaults.
func decodeChecklist(fm wikipage.FrontMatter, listName string, clock Clock) *apiv1.Checklist {
	out := &apiv1.Checklist{Name: listName}

	userList := readMap(readMap(fm, userChecklistsKey), listName)
	wikiList := readMap(readMap(readMap(fm, wikiKey), userChecklistsKey), listName)
	wikiItems := readMap(wikiList, itemsKey)

	if userList == nil {
		// No user data; still surface metadata if present (e.g. tombstones
		// from a list whose items were all deleted).
		out.Items = nil
	} else {
		rawItems := readSlice(userList, itemsKey)
		now := clock.Now()
		for _, raw := range rawItems {
			itemMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			out.Items = append(out.Items, decodeItem(itemMap, wikiItems, now))
		}
	}
	sortItems(out.Items)

	if wikiList != nil {
		if syncToken, ok := readInt64(wikiList, syncTokenKey); ok {
			out.SyncToken = syncToken
		}
		if t, ok := readTimestamp(wikiList, updatedAtKey); ok {
			out.UpdatedAt = t
		}
		out.Tombstones = decodeTombstones(readSlice(wikiList, tombstonesKey))
	}

	return out
}

// decodeItem reads a single item from its user-data map plus any matching
// wiki-managed metadata at wikiItems[uid]. now is used as a fallback for
// created_at/updated_at when the item lacks a uid (synthetic) — those
// values won't persist until the next mutation that calls encodeChecklist.
func decodeItem(itemMap map[string]any, wikiItems map[string]any, now time.Time) *apiv1.ChecklistItem {
	uid := stringValue(itemMap, uidKey)
	item := &apiv1.ChecklistItem{
		Uid:       uid,
		Text:      stringValue(itemMap, "text"),
		Checked:   boolValue(itemMap, "checked"),
		Tags:      stringSlice(itemMap, "tags"),
		SortOrder: int64Value(itemMap, "sort_order"),
	}
	if v := stringValue(itemMap, "description"); v != "" {
		item.Description = &v
	}
	if t, ok := readTimestampValue(itemMap["due"]); ok {
		item.Due = t
	}
	if v := stringValue(itemMap, "alarm_payload"); v != "" {
		item.AlarmPayload = &v
	}

	// Wiki-managed metadata, when present, lives at wiki.checklists.<list>.items.<uid>.
	if uid != "" {
		if meta, ok := readMap(wikiItems, uid), true; ok && meta != nil {
			if t, ok := readTimestamp(meta, "created_at"); ok {
				item.CreatedAt = t
			}
			if t, ok := readTimestamp(meta, "updated_at"); ok {
				item.UpdatedAt = t
			}
			if t, ok := readTimestamp(meta, "completed_at"); ok {
				item.CompletedAt = t
			}
			if v := stringValue(meta, "completed_by"); v != "" {
				item.CompletedBy = &v
			}
			item.Automated = boolValue(meta, "automated")
		}
	}

	// Synthesize created_at/updated_at when missing — legacy items pre-
	// migration. The caller will persist these on the next write.
	if item.CreatedAt == nil {
		item.CreatedAt = timestamppb.New(now)
	}
	if item.UpdatedAt == nil {
		item.UpdatedAt = timestamppb.New(now)
	}
	return item
}

// decodeTombstones reads the tombstones slice from wiki-managed data.
func decodeTombstones(raw []any) []*apiv1.Tombstone {
	if len(raw) == 0 {
		return nil
	}
	out := make([]*apiv1.Tombstone, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		t := &apiv1.Tombstone{Uid: stringValue(m, uidKey)}
		if ts, ok := readTimestamp(m, "deleted_at"); ok {
			t.DeletedAt = ts
		}
		if ts, ok := readTimestamp(m, "gc_after"); ok {
			t.GcAfter = ts
		}
		out = append(out, t)
	}
	return out
}

// encodeChecklist writes checklist back into fm, splitting into the
// user-data and wiki-managed subtrees. The user data ends up at
// fm.checklists.<list>.items[]; metadata lands under fm.wiki.checklists.<list>.
func encodeChecklist(fm wikipage.FrontMatter, listName string, checklist *apiv1.Checklist) {
	userList := ensureMap(ensureMap(fm, userChecklistsKey), listName)
	wikiList := ensureMap(ensureMap(ensureMap(fm, wikiKey), userChecklistsKey), listName)

	// Encode user-mutable items.
	rawItems := make([]any, 0, len(checklist.Items))
	wikiItems := make(map[string]any, len(checklist.Items))
	for _, item := range checklist.Items {
		rawItems = append(rawItems, encodeItemUserData(item))
		wikiItems[item.Uid] = encodeItemMetadata(item)
	}
	userList[itemsKey] = rawItems
	wikiList[itemsKey] = wikiItems

	// Encode list-level wiki-managed fields.
	wikiList[syncTokenKey] = checklist.SyncToken
	if checklist.UpdatedAt != nil {
		wikiList[updatedAtKey] = checklist.UpdatedAt.AsTime().Format(time.RFC3339Nano)
	}
	if len(checklist.Tombstones) > 0 {
		wikiList[tombstonesKey] = encodeTombstones(checklist.Tombstones)
	} else {
		delete(wikiList, tombstonesKey)
	}
}

func encodeItemUserData(item *apiv1.ChecklistItem) map[string]any {
	out := map[string]any{
		uidKey:      item.Uid,
		"text":       item.Text,
		"checked":    item.Checked,
		"sort_order": item.SortOrder,
	}
	if len(item.Tags) > 0 {
		out["tags"] = stringSliceToAny(item.Tags)
	}
	if item.Description != nil && *item.Description != "" {
		out["description"] = *item.Description
	}
	if item.Due != nil {
		out["due"] = item.Due.AsTime().Format(time.RFC3339Nano)
	}
	if item.AlarmPayload != nil && *item.AlarmPayload != "" {
		out["alarm_payload"] = *item.AlarmPayload
	}
	return out
}

func encodeItemMetadata(item *apiv1.ChecklistItem) map[string]any {
	out := map[string]any{
		"automated": item.Automated,
	}
	if item.CreatedAt != nil {
		out["created_at"] = item.CreatedAt.AsTime().Format(time.RFC3339Nano)
	}
	if item.UpdatedAt != nil {
		out["updated_at"] = item.UpdatedAt.AsTime().Format(time.RFC3339Nano)
	}
	if item.CompletedAt != nil {
		out["completed_at"] = item.CompletedAt.AsTime().Format(time.RFC3339Nano)
	}
	if item.CompletedBy != nil {
		out["completed_by"] = *item.CompletedBy
	}
	return out
}

func encodeTombstones(tombstones []*apiv1.Tombstone) []any {
	// Sort tombstones by deleted_at for determinism.
	sorted := append([]*apiv1.Tombstone(nil), tombstones...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].DeletedAt.AsTime().Before(sorted[j].DeletedAt.AsTime())
	})
	out := make([]any, 0, len(sorted))
	for _, t := range sorted {
		m := map[string]any{uidKey: t.Uid}
		if t.DeletedAt != nil {
			m["deleted_at"] = t.DeletedAt.AsTime().Format(time.RFC3339Nano)
		}
		if t.GcAfter != nil {
			m["gc_after"] = t.GcAfter.AsTime().Format(time.RFC3339Nano)
		}
		out = append(out, m)
	}
	return out
}

// listChecklistNames returns every list name on the page (the union of
// names that appear under user-data checklists.* and wiki-managed
// wiki.checklists.*). Used by GetChecklists.
func listChecklistNames(fm wikipage.FrontMatter) []string {
	userLists := readMap(fm, userChecklistsKey)
	wikiLists := readMap(readMap(fm, wikiKey), userChecklistsKey)
	seen := make(map[string]struct{})
	for name := range userLists {
		seen[name] = struct{}{}
	}
	for name := range wikiLists {
		seen[name] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// readMap returns m[key] as a map[string]any, or nil when missing/wrong-type.
func readMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}

// readSlice returns m[key] as []any, or nil when missing/wrong-type.
func readSlice(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	v, ok := m[key].([]any)
	if !ok {
		return nil
	}
	return v
}

// readInt64 returns m[key] as int64. TOML decodes integers as int64
// directly, but JSON-via-structpb routes through float64 — handle both.
func readInt64(m map[string]any, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	switch v := m[key].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	}
	return 0, false
}

// readTimestamp parses an RFC3339Nano string at m[key] into a Timestamp.
func readTimestamp(m map[string]any, key string) (*timestamppb.Timestamp, bool) {
	if m == nil {
		return nil, false
	}
	return readTimestampValue(m[key])
}

func readTimestampValue(v any) (*timestamppb.Timestamp, bool) {
	switch s := v.(type) {
	case string:
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, false
		}
		return timestamppb.New(t), true
	case time.Time:
		return timestamppb.New(s), true
	}
	return nil, false
}

// stringValue returns m[key] as a string, or empty when missing/wrong-type.
func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

// boolValue returns m[key] as a bool, or false when missing/wrong-type.
func boolValue(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key].(bool)
	if !ok {
		return false
	}
	return v
}

// int64Value returns m[key] as an int64 with the same float64 fallback as
// readInt64.
func int64Value(m map[string]any, key string) int64 {
	v, _ := readInt64(m, key)
	return v
}

// stringSlice returns m[key] as []string, accepting both []string and []any.
func stringSlice(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, raw := range v {
			if s, ok := raw.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// stringSliceToAny converts []string to []any for the TOML/structpb-friendly
// frontmatter shape.
func stringSliceToAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

// ensureMap returns m[key] as a map[string]any, creating an empty one if
// missing.
func ensureMap(m map[string]any, key string) map[string]any {
	if existing, ok := m[key].(map[string]any); ok {
		return existing
	}
	created := make(map[string]any)
	m[key] = created
	return created
}
