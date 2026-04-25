// Package eager: checklist data-model migration.
//
// MOVES legacy checklist items into the reserved wiki.checklists.*
// subtree, in line with ADR-0010. After migration, a page has:
//
//   - All items at wiki.checklists.<name>.items[] (slice of full ChecklistItem
//     maps with uid/text/checked/tags/sort_order/created_at/updated_at/
//     completed_at/completed_by/automated).
//   - wiki.checklists.<name>.{sync_token, updated_at, tombstones,
//     migrated_data_model = true}.
//   - NO checklists.<name> subtree at all.
//
// Re-runnable: pages where every list has wiki.checklists.<name>.items
// as a slice are skipped. Pages migrated by an earlier draft of this
// code (where items were split between checklists.* and wiki.checklists.*)
// are detected (wiki.checklists.<list>.items is a map[string]any rather
// than []any) and re-migrated cleanly.
//
// The tag-syntax migration in checklist_tag_syntax_migration.go is the
// template — see that file for the cross-cutting concerns (system-page
// guard, scan job pattern, frontmatter parsing).
package eager

import (
	"fmt"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/brendanjerwin/simple_wiki/internal/syspage"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// migratedDataModelKey is the per-list flag set after promoting items
// to the new shape. Lives under wiki.checklists.<name>.
const migratedDataModelKey = "migrated_data_model"

// dataModelSortOrderStep matches checklistmutator.SortOrderStep — the
// conventional sparse-spacing between adjacent items. Defined locally
// to avoid the migrations package depending on the mutator.
const dataModelSortOrderStep int64 = 1000

// Frontmatter keys repeated across the migration — declared once so the
// linter doesn't trip on "string appears N times" and so any rename is
// a single-edit change.
const (
	checklistsFMKey = "checklists"
	updatedAtFMKey  = "updated_at"
	wikiFMKey       = "wiki"
	itemsFMKey      = "items"
	uidFMKey        = "uid"
	textFMKey       = "text"
	checkedFMKey    = "checked"
	tagsFMKey       = "tags"
	sortOrderFMKey  = "sort_order"
	descriptionFMKey = "description"
	dueFMKey        = "due"
	createdAtFMKey  = "created_at"
	completedAtFMKey = "completed_at"
	completedByFMKey = "completed_by"
	automatedFMKey  = "automated"
	syncTokenFMKey  = "sync_token"
)

// ChecklistDataModelMigrationScanJob walks the data dir and enqueues a
// per-page job for every page that has a checklists subtree where at
// least one list still needs migration to the wiki.checklists.* shape.
type ChecklistDataModelMigrationScanJob struct {
	scanner       DataDirScanner
	coordinator   *jobs.JobQueueCoordinator
	readerMutator wikipage.PageReaderMutator
	ulids         ulid.Generator
}

// NewChecklistDataModelMigrationScanJob constructs the scan job.
func NewChecklistDataModelMigrationScanJob(
	scanner DataDirScanner,
	coordinator *jobs.JobQueueCoordinator,
	readerMutator wikipage.PageReaderMutator,
	ulids ulid.Generator,
) *ChecklistDataModelMigrationScanJob {
	return &ChecklistDataModelMigrationScanJob{
		scanner:       scanner,
		coordinator:   coordinator,
		readerMutator: readerMutator,
		ulids:         ulids,
	}
}

// GetName returns the job name for queue logging.
func (*ChecklistDataModelMigrationScanJob) GetName() string {
	return "ChecklistDataModelMigrationScanJob"
}

// Execute scans .md files and enqueues per-page migration jobs.
func (j *ChecklistDataModelMigrationScanJob) Execute() error {
	if !j.scanner.DataDirExists() {
		return nil
	}

	files, err := j.scanner.ListMDFiles()
	if err != nil {
		return fmt.Errorf("list .md files: %w", err)
	}

	seen := make(map[string]struct{})
	for _, filename := range files {
		identifier, fm, ok := extractDataModelMigrationFrontmatter(j.scanner, filename)
		if !ok {
			continue
		}
		if _, dup := seen[identifier]; dup {
			continue
		}
		seen[identifier] = struct{}{}

		if !pageNeedsDataModelMigration(fm) {
			continue
		}

		migrationJob := NewChecklistDataModelMigrationJob(j.readerMutator, j.ulids, identifier)
		if err := j.coordinator.EnqueueJob(migrationJob); err != nil {
			return fmt.Errorf("enqueue checklist data-model migration for %s: %w", identifier, err)
		}
	}
	return nil
}

// extractDataModelMigrationFrontmatter mirrors the helper used by the
// tag-syntax scan; pulls just the frontmatter from a .md file.
func extractDataModelMigrationFrontmatter(scanner DataDirScanner, filename string) (string, map[string]any, bool) {
	mdData, err := scanner.ReadMDFile(filename)
	if err != nil {
		return "", nil, false
	}

	content := string(mdData)
	if !strings.HasPrefix(content, "+++") {
		return "", nil, false
	}

	parts := strings.SplitN(content, "+++", tomlFrontmatterParts)
	if len(parts) < tomlFrontmatterParts {
		return "", nil, false
	}

	fm := map[string]any{}
	if err := toml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return "", nil, false
	}

	id, ok := fm["identifier"].(string)
	if !ok || id == "" {
		return "", nil, false
	}
	return id, fm, true
}

// pageNeedsDataModelMigration reports whether the page has any checklist
// list that still needs migrating. A list needs migration if either:
//
//   - It exists at the legacy `checklists.<name>` location (regardless
//     of whether `wiki.checklists.<name>.migrated_data_model` is set —
//     a stale flag from the old two-tier draft must not block re-migration).
//   - It exists under `wiki.checklists.<name>` but `items` is not a slice
//     (e.g. the old draft persisted `items` as a map keyed by uid).
//
// System pages are skipped (they ship with the wiki binary and cannot
// be edited via the gRPC API anyway).
func pageNeedsDataModelMigration(fm map[string]any) bool {
	if syspage.IsSystemPage(fm) {
		return false
	}

	if legacyHasAny(fm) {
		return true
	}
	wikiChecklists := readNestedMap(fm, wikiFMKey, checklistsFMKey)
	for _, raw := range wikiChecklists {
		listMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		// Items must be a slice in the new shape. A map indicates the
		// old draft and needs re-migration.
		if _, isMap := listMap[itemsFMKey].(map[string]any); isMap {
			return true
		}
	}
	return false
}

// legacyHasAny reports whether the page still has any checklists.<name>
// subtree (un-migrated, or partially migrated by the old draft).
func legacyHasAny(fm map[string]any) bool {
	legacy, ok := fm[checklistsFMKey].(map[string]any)
	if !ok {
		return false
	}
	for _, raw := range legacy {
		if _, ok := raw.(map[string]any); ok {
			return true
		}
	}
	return false
}

// ChecklistDataModelMigrationJob promotes a single page's checklists to
// the new shape, moves them under wiki.checklists.*, and removes the
// legacy checklists.* subtree on the same write.
type ChecklistDataModelMigrationJob struct {
	readerMutator wikipage.PageReaderMutator
	ulids         ulid.Generator
	identifier    string
}

// NewChecklistDataModelMigrationJob constructs the per-page job.
func NewChecklistDataModelMigrationJob(rw wikipage.PageReaderMutator, ulids ulid.Generator, id string) *ChecklistDataModelMigrationJob {
	return &ChecklistDataModelMigrationJob{readerMutator: rw, ulids: ulids, identifier: id}
}

// GetName returns the job name (includes the identifier for queue tracing).
func (j *ChecklistDataModelMigrationJob) GetName() string {
	return fmt.Sprintf("ChecklistDataModelMigrationJob-%s", j.identifier)
}

// Execute reads the page's frontmatter, moves all checklist data into
// wiki.checklists.*, and writes back via the standard write path.
func (j *ChecklistDataModelMigrationJob) Execute() error {
	id := wikipage.PageIdentifier(j.identifier)
	_, fm, err := j.readerMutator.ReadFrontMatter(id)
	if err != nil {
		return fmt.Errorf("read frontmatter for %s: %w", j.identifier, err)
	}
	if fm == nil {
		return nil
	}

	if !migrateChecklistsIntoWikiNamespace(fm, j.ulids) {
		return nil
	}

	if err := j.readerMutator.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf("write migrated frontmatter for %s: %w", j.identifier, err)
	}
	return nil
}

// migrateChecklistsIntoWikiNamespace performs the move + promote. Returns
// true when any change was made.
//
// One logical pass per list. For each list, the input may be in one of
// three shapes:
//
//   - Legacy only: checklists.<list>.items[] exists, no wiki.checklists.<list>.
//   - Old-draft split: checklists.<list>.items[] exists, wiki.checklists.<list>.items
//     is a uid-keyed map of metadata.
//   - Old-draft metadata-only: no checklists.<list>, wiki.checklists.<list>.items
//     is a uid-keyed map (items got fully deleted before any new-shape migration).
//   - New shape: no checklists.<list>, wiki.checklists.<list>.items is a slice.
//
// Build the full item slice once per list, walking legacy items if any,
// joining per-uid metadata from the old-draft map when present, then
// write the slice back. Then drop the legacy subtree on any change.
func migrateChecklistsIntoWikiNamespace(fm map[string]any, ulids ulid.Generator) bool {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	changed := false

	wikiChecklists := ensureNestedMap(fm, wikiFMKey, checklistsFMKey)
	legacyChecklists, hasLegacy := fm[checklistsFMKey].(map[string]any)
	if !hasLegacy {
		legacyChecklists = nil
	}

	// Union of list names across both namespaces.
	names := make(map[string]struct{})
	for name := range legacyChecklists {
		names[name] = struct{}{}
	}
	for name := range wikiChecklists {
		names[name] = struct{}{}
	}

	for name := range names {
		legacyList, ok := legacyChecklists[name].(map[string]any)
		if !ok {
			legacyList = nil
		}
		wikiList := ensureMapInParent(wikiChecklists, name)
		if migrateOneList(legacyList, wikiList, ulids, now) {
			changed = true
		}
	}

	// Drop the legacy subtree wholesale once any migration touched the
	// page — the reserved namespace is now the only home for checklists.
	if changed && legacyChecklists != nil {
		delete(fm, checklistsFMKey)
	}

	return changed
}

// migrateOneList consolidates one list's data under wikiList. Returns
// true if the wikiList's shape changed.
func migrateOneList(legacyList, wikiList map[string]any, ulids ulid.Generator, now string) bool {
	existingItemsSlice, hasSlice := wikiList[itemsFMKey].([]any)
	existingItemsMap, hasMap := wikiList[itemsFMKey].(map[string]any)
	hasLegacyItems := legacyList != nil && hasItems(legacyList)
	flagAlreadySet := boolFromMap(wikiList, migratedDataModelKey)

	// Already in the right shape and no legacy data to absorb: just
	// idempotency-stamp and move on.
	if hasSlice && !hasLegacyItems && !hasMap && flagAlreadySet {
		return ensureFlagAndDefaults(wikiList, now)
	}

	// Build the new item slice from whatever sources are available.
	merged := []any{}
	switch {
	case hasLegacyItems:
		legacyItems, ok := legacyList[itemsFMKey].([]any)
		if !ok {
			legacyItems = nil
		}
		merged = mergeLegacyAndDraftMetadata(legacyItems, existingItemsMap, ulids, now)
	case hasMap:
		merged = convertDraftMapToSlice(existingItemsMap, now)
	case hasSlice:
		// New-shape items already a slice. Keep them, but still let the
		// flag-stamping path run.
		merged = existingItemsSlice
	default:
		// No items anywhere — start with an empty slice so the encoded
		// list at least has a valid shape.
	}

	wikiList[itemsFMKey] = merged
	_ = ensureFlagAndDefaults(wikiList, now)
	return true
}

// hasItems reports whether legacyList[items] is a non-empty []any.
func hasItems(legacyList map[string]any) bool {
	items, ok := legacyList[itemsFMKey].([]any)
	return ok && len(items) > 0
}

// ensureFlagAndDefaults stamps migrated_data_model + sync_token defaults
// when missing. Returns true if anything changed.
func ensureFlagAndDefaults(wikiList map[string]any, now string) bool {
	changed := false
	if !boolFromMap(wikiList, migratedDataModelKey) {
		wikiList[migratedDataModelKey] = true
		changed = true
	}
	if _, has := wikiList[syncTokenFMKey]; !has {
		wikiList[syncTokenFMKey] = int64(0)
		changed = true
	}
	if _, has := wikiList[updatedAtFMKey]; !has {
		wikiList[updatedAtFMKey] = now
		changed = true
	}
	return changed
}

// mergeLegacyAndDraftMetadata builds a new full-item slice from legacy
// items[] and any per-uid old-draft metadata map. Items get ULIDs and
// sort_orders if missing; metadata fields are pulled from oldDraftMeta
// when the uid matches.
func mergeLegacyAndDraftMetadata(legacyItems []any, oldDraftMeta map[string]any, ulids ulid.Generator, now string) []any {
	out := make([]any, 0, len(legacyItems))
	for idx, raw := range legacyItems {
		itemMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, promoteAndJoinItem(itemMap, oldDraftMeta, idx, ulids, now))
	}
	return out
}

// convertDraftMapToSlice turns an old-draft items map (keyed by uid)
// into the new slice shape.
func convertDraftMapToSlice(itemsMap map[string]any, now string) []any {
	out := make([]any, 0, len(itemsMap))
	for uid, raw := range itemsMap {
		meta, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		full := map[string]any{
			uidFMKey:       uid,
			textFMKey:      stringFromMap(meta, textFMKey),
			checkedFMKey:   boolFromMap(meta, checkedFMKey),
			sortOrderFMKey: int64Fallback(meta, sortOrderFMKey),
			automatedFMKey: boolFromMap(meta, automatedFMKey),
		}
		if v := stringFromMap(meta, createdAtFMKey); v != "" {
			full[createdAtFMKey] = v
		} else {
			full[createdAtFMKey] = now
		}
		if v := stringFromMap(meta, updatedAtFMKey); v != "" {
			full[updatedAtFMKey] = v
		} else {
			full[updatedAtFMKey] = now
		}
		if v := stringFromMap(meta, completedAtFMKey); v != "" {
			full[completedAtFMKey] = v
		}
		if v := stringFromMap(meta, completedByFMKey); v != "" {
			full[completedByFMKey] = v
		}
		out = append(out, full)
	}
	return out
}

// promoteAndJoinItem turns a legacy item map into the new full-item map.
// Assigns uid + sort_order if missing, fills in timestamps from old-draft
// metadata when available, otherwise stamps `now`. Pre-existing items are
// recorded as automated=false (no retroactive attribution).
func promoteAndJoinItem(itemMap map[string]any, oldDraftMeta map[string]any, idx int, ulids ulid.Generator, now string) map[string]any {
	uid := stringFromMap(itemMap, uidFMKey)
	if uid == "" {
		uid = ulids.NewULID()
	}
	sortOrder, hasSortOrder := int64FromMap(itemMap, sortOrderFMKey)
	if !hasSortOrder {
		sortOrder = int64(idx+1) * dataModelSortOrderStep
	}

	out := map[string]any{
		uidFMKey:       uid,
		textFMKey:      stringFromMap(itemMap, textFMKey),
		checkedFMKey:   boolFromMap(itemMap, checkedFMKey),
		sortOrderFMKey: sortOrder,
		automatedFMKey: false,
	}

	// Carry user-mutable optional fields if present in the legacy item.
	if v := stringFromMap(itemMap, descriptionFMKey); v != "" {
		out[descriptionFMKey] = v
	}
	if v := stringFromMap(itemMap, dueFMKey); v != "" {
		out[dueFMKey] = v
	}
	if tags, ok := itemMap[tagsFMKey].([]any); ok && len(tags) > 0 {
		out[tagsFMKey] = tags
	}

	// Pull per-item metadata from the old-draft split (if any).
	createdAt := now
	updatedAt := now
	if meta, ok := oldDraftMeta[uid].(map[string]any); ok {
		if v := stringFromMap(meta, createdAtFMKey); v != "" {
			createdAt = v
		}
		if v := stringFromMap(meta, updatedAtFMKey); v != "" {
			updatedAt = v
		}
		if v := stringFromMap(meta, completedAtFMKey); v != "" {
			out[completedAtFMKey] = v
		}
		if v := stringFromMap(meta, completedByFMKey); v != "" {
			out[completedByFMKey] = v
		}
		if v, ok := meta[automatedFMKey].(bool); ok {
			out[automatedFMKey] = v
		}
	}
	out[createdAtFMKey] = createdAt
	out[updatedAtFMKey] = updatedAt
	return out
}

// readNestedMap walks fm[k1][k2][...] returning the deepest map or nil.
func readNestedMap(fm map[string]any, keys ...string) map[string]any {
	cur := fm
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		next, ok := cur[k].(map[string]any)
		if !ok || next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// ensureNestedMap is the mutating sibling of readNestedMap — creates
// intermediate maps as needed and returns the deepest map.
func ensureNestedMap(fm map[string]any, keys ...string) map[string]any {
	cur := fm
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok || next == nil {
			next = make(map[string]any)
			cur[k] = next
		}
		cur = next
	}
	return cur
}

// ensureMapInParent returns parent[key] as a map[string]any, creating
// an empty map if missing or wrong type.
func ensureMapInParent(parent map[string]any, key string) map[string]any {
	existing, ok := parent[key].(map[string]any)
	if ok {
		return existing
	}
	created := make(map[string]any)
	parent[key] = created
	return created
}

// stringFromMap returns m[key] as string or empty.
func stringFromMap(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

// boolFromMap returns m[key] as bool or false.
func boolFromMap(m map[string]any, key string) bool {
	v, ok := m[key].(bool)
	if !ok {
		return false
	}
	return v
}

// int64FromMap returns (value, true) when the key holds an int64/int/float64.
func int64FromMap(m map[string]any, key string) (int64, bool) {
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

// int64Fallback is the no-ok form of int64FromMap.
func int64Fallback(m map[string]any, key string) int64 {
	v, _ := int64FromMap(m, key)
	return v
}
