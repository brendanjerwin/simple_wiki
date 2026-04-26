package checklistmutator

import (
	"sort"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Single-namespace persistence layout (per ADR-0010).
//
// All checklist data lives at fm["wiki"]["checklists"][listName]:
//
//   - items[]: array of full ChecklistItem maps (uid/text/checked/tags/
//     sort_order/description/due/alarm_payload PLUS created_at/updated_at/
//     completed_at/completed_by/automated)
//   - sync_token: int
//   - updated_at: RFC3339 string (collection ETag)
//   - tombstones[]: array of {uid, deleted_at, gc_after}
//   - migrated_data_model: bool (set by the eager migration)
//
// The legacy fm["checklists"][listName] subtree is only consulted as a
// fall-through when reading; the eager migration moves it into the
// reserved namespace and deletes it. Bare reads on un-migrated pages
// surface the legacy items in-memory but never persist them under
// `checklists.*`.

const (
	wikiKey            = "wiki"
	checklistsKey      = "checklists"
	itemsKey           = "items"
	tombstonesKey      = "tombstones"
	syncTokenKey       = "sync_token"
	updatedAtKey       = "updated_at"
	uidKey             = "uid"
	textKey            = "text"
	checkedKey         = "checked"
	tagsKey            = "tags"
	sortOrderKey       = "sort_order"
	descriptionKey     = "description"
	dueKey             = "due"
	alarmPayloadKey    = "alarm_payload"
	createdAtKey       = "created_at"
	completedAtKey     = "completed_at"
	completedByKey     = "completed_by"
	automatedKey       = "automated"
)

// decodeChecklist reads the named checklist out of fm. Reads from
// wiki.checklists.<list> first; falls back to legacy checklists.<list>
// items[] when the wiki-managed subtree is empty (un-migrated page).
// Items missing a uid get an empty Uid string in the response — the
// mutator's readChecklistForMutation promotes them on next mutation.
func decodeChecklist(fm wikipage.FrontMatter, listName string, clock Clock) *apiv1.Checklist {
	out := &apiv1.Checklist{Name: listName}
	now := clock.Now()

	wikiList := readMap(readMap(fm, wikiKey), checklistsKey)
	wikiList = readMap(wikiList, listName)

	if wikiList != nil {
		if syncToken, ok := readInt64(wikiList, syncTokenKey); ok {
			out.SyncToken = syncToken
		}
		if t, ok := readTimestamp(wikiList, updatedAtKey); ok {
			out.UpdatedAt = t
		}
		out.Tombstones = decodeTombstones(readSlice(wikiList, tombstonesKey))
		for _, raw := range readSlice(wikiList, itemsKey) {
			itemMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			out.Items = append(out.Items, decodeItem(itemMap, now))
		}
	}

	if len(out.Items) == 0 {
		// Fall-through: page hasn't been migrated yet. Legacy items live
		// at checklists.<list>.items[] without uids. Surface them so
		// reads work; the next mutation will move + persist.
		legacyList := readMap(readMap(fm, checklistsKey), listName)
		for _, raw := range readSlice(legacyList, itemsKey) {
			itemMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			out.Items = append(out.Items, decodeLegacyItem(itemMap, now))
		}
	}

	sortItems(out.Items)
	return out
}

// decodeItem reads a single fully-shaped item from wiki.checklists.*.
func decodeItem(itemMap map[string]any, now time.Time) *apiv1.ChecklistItem {
	item := &apiv1.ChecklistItem{
		Uid:       stringValue(itemMap, uidKey),
		Text:      stringValue(itemMap, textKey),
		Checked:   boolValue(itemMap, checkedKey),
		Tags:      stringSlice(itemMap, tagsKey),
		SortOrder: int64Value(itemMap, sortOrderKey),
		Automated: boolValue(itemMap, automatedKey),
	}
	if v := stringValue(itemMap, descriptionKey); v != "" {
		item.Description = &v
	}
	if t, ok := readTimestampValue(itemMap[dueKey]); ok {
		item.Due = t
	}
	if v := stringValue(itemMap, alarmPayloadKey); v != "" {
		item.AlarmPayload = &v
	}
	if t, ok := readTimestamp(itemMap, createdAtKey); ok {
		item.CreatedAt = t
	}
	if t, ok := readTimestamp(itemMap, updatedAtKey); ok {
		item.UpdatedAt = t
	}
	if t, ok := readTimestamp(itemMap, completedAtKey); ok {
		item.CompletedAt = t
	}
	if v := stringValue(itemMap, completedByKey); v != "" {
		item.CompletedBy = &v
	}
	if item.CreatedAt == nil {
		item.CreatedAt = timestamppb.New(now)
	}
	if item.UpdatedAt == nil {
		item.UpdatedAt = timestamppb.New(now)
	}
	return item
}

// decodeLegacyItem reads a pre-migration item from checklists.<list>.items[].
// Such items lack uid and per-item metadata; we synthesize timestamps in
// memory so reads work, but leave Uid empty so the mutator's promotion
// step can detect and assign one on next persist.
func decodeLegacyItem(itemMap map[string]any, now time.Time) *apiv1.ChecklistItem {
	item := &apiv1.ChecklistItem{
		Uid:       "", // intentionally empty; promoted on next mutation
		Text:      stringValue(itemMap, textKey),
		Checked:   boolValue(itemMap, checkedKey),
		Tags:      stringSlice(itemMap, tagsKey),
		SortOrder: int64Value(itemMap, sortOrderKey),
		CreatedAt: timestamppb.New(now),
		UpdatedAt: timestamppb.New(now),
	}
	if v := stringValue(itemMap, descriptionKey); v != "" {
		item.Description = &v
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
		if syncToken, ok := readInt64(m, syncTokenKey); ok {
			t.SyncToken = syncToken
		}
		out = append(out, t)
	}
	return out
}

// encodeChecklist writes the proto Checklist back into fm under
// wiki.checklists.<list>.* and removes any legacy checklists.<list>
// subtree on the same write — once a list is owned by the funnel, the
// reserved namespace is its only home.
func encodeChecklist(fm wikipage.FrontMatter, listName string, checklist *apiv1.Checklist) {
	wikiList := ensureMap(ensureMap(ensureMap(fm, wikiKey), checklistsKey), listName)

	rawItems := make([]any, 0, len(checklist.Items))
	for _, item := range checklist.Items {
		rawItems = append(rawItems, encodeItem(item))
	}
	wikiList[itemsKey] = rawItems
	wikiList[syncTokenKey] = checklist.SyncToken
	if checklist.UpdatedAt != nil {
		wikiList[updatedAtKey] = checklist.UpdatedAt.AsTime().Format(time.RFC3339Nano)
	}
	if len(checklist.Tombstones) > 0 {
		wikiList[tombstonesKey] = encodeTombstones(checklist.Tombstones)
	} else {
		delete(wikiList, tombstonesKey)
	}

	// Remove the legacy checklists.<list> subtree if present — the
	// reserved namespace is now this list's only home. The migration's
	// post-promote write does the same; this catches any cases where
	// ChecklistService mutates a legacy-shape page directly without the
	// migration job having swept it yet.
	if legacy := readMap(fm, checklistsKey); legacy != nil {
		delete(legacy, listName)
		if len(legacy) == 0 {
			delete(fm, checklistsKey)
		}
	}
}

func encodeItem(item *apiv1.ChecklistItem) map[string]any {
	out := map[string]any{
		uidKey:       item.Uid,
		textKey:      item.Text,
		checkedKey:   item.Checked,
		sortOrderKey: item.SortOrder,
		automatedKey: item.Automated,
	}
	if len(item.Tags) > 0 {
		out[tagsKey] = stringSliceToAny(item.Tags)
	}
	if item.Description != nil && *item.Description != "" {
		out[descriptionKey] = *item.Description
	}
	if item.Due != nil {
		out[dueKey] = item.Due.AsTime().Format(time.RFC3339Nano)
	}
	if item.AlarmPayload != nil && *item.AlarmPayload != "" {
		out[alarmPayloadKey] = *item.AlarmPayload
	}
	if item.CreatedAt != nil {
		out[createdAtKey] = item.CreatedAt.AsTime().Format(time.RFC3339Nano)
	}
	if item.UpdatedAt != nil {
		out[updatedAtKey] = item.UpdatedAt.AsTime().Format(time.RFC3339Nano)
	}
	if item.CompletedAt != nil {
		out[completedAtKey] = item.CompletedAt.AsTime().Format(time.RFC3339Nano)
	}
	if item.CompletedBy != nil {
		out[completedByKey] = *item.CompletedBy
	}
	return out
}

func encodeTombstones(tombstones []*apiv1.Tombstone) []any {
	sorted := append([]*apiv1.Tombstone(nil), tombstones...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].DeletedAt.AsTime().Before(sorted[j].DeletedAt.AsTime())
	})
	out := make([]any, 0, len(sorted))
	for _, t := range sorted {
		m := map[string]any{
			uidKey:       t.Uid,
			syncTokenKey: t.SyncToken,
		}
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

// listChecklistNames returns every list name on the page — union of
// names that appear under wiki.checklists.* and the legacy checklists.*
// (the latter only matters until the migration sweeps the page).
func listChecklistNames(fm wikipage.FrontMatter) []string {
	wikiLists := readMap(readMap(fm, wikiKey), checklistsKey)
	legacyLists := readMap(fm, checklistsKey)
	seen := make(map[string]struct{})
	for name := range wikiLists {
		seen[name] = struct{}{}
	}
	for name := range legacyLists {
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
