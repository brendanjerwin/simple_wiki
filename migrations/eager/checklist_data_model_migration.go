// Package eager: checklist data-model migration.
//
// Promotes legacy checklist items (no uid, no per-item metadata) to the
// new shape introduced in #984: ULID per item, server-stamped
// created_at/updated_at, and per-list sync bookkeeping under the
// reserved wiki.checklists.* namespace. Each list gets stamped with
// wiki.checklists.<name>.migrated_data_model = true so subsequent scans
// skip it.
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
)

// ChecklistDataModelMigrationScanJob walks the data dir and enqueues a
// per-page job for every page that has a checklists subtree where at
// least one list lacks the migrated_data_model flag.
type ChecklistDataModelMigrationScanJob struct {
	scanner       DataDirScanner
	coordinator   *jobs.JobQueueCoordinator
	readerMutator wikipage.PageReaderMutator
	ulids         ulid.Generator
}

// NewChecklistDataModelMigrationScanJob constructs the scan job. The ulid
// generator is injected so tests can assert specific assignments; in
// production it should be ulid.NewSystemGenerator().
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

// Execute scans .md files and enqueues per-page migration jobs for pages
// that need promoting.
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

// pageNeedsDataModelMigration reports whether the page has at least one
// checklist that lacks the wiki.checklists.<name>.migrated_data_model
// flag. System pages are skipped (they ship with the wiki binary and
// cannot be edited via the gRPC API anyway).
func pageNeedsDataModelMigration(fm map[string]any) bool {
	if syspage.IsSystemPage(fm) {
		return false
	}
	checklists, ok := fm[checklistsFMKey].(map[string]any)
	if !ok {
		return false
	}
	wikiChecklists := readNestedMap(fm, wikiFMKey, checklistsFMKey)
	for name := range checklists {
		flag := dataModelMigratedFlag(wikiChecklists, name)
		if !flag {
			return true
		}
	}
	return false
}

// dataModelMigratedFlag returns the migrated_data_model flag for a
// specific list, false when missing.
func dataModelMigratedFlag(wikiChecklists map[string]any, name string) bool {
	if wikiChecklists == nil {
		return false
	}
	listMeta, ok := wikiChecklists[name].(map[string]any)
	if !ok {
		return false
	}
	flag, ok := listMeta[migratedDataModelKey].(bool)
	return ok && flag
}

// ChecklistDataModelMigrationJob promotes a single page's checklists to
// the new shape and stamps the flag on every list it touched.
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

// Execute reads the page's frontmatter, promotes legacy checklist items
// to the new shape, and writes back via the standard write path.
func (j *ChecklistDataModelMigrationJob) Execute() error {
	id := wikipage.PageIdentifier(j.identifier)
	_, fm, err := j.readerMutator.ReadFrontMatter(id)
	if err != nil {
		return fmt.Errorf("read frontmatter for %s: %w", j.identifier, err)
	}
	if fm == nil {
		return nil
	}

	if !promoteChecklistDataModel(fm, j.ulids) {
		return nil
	}

	if err := j.readerMutator.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf("write migrated frontmatter for %s: %w", j.identifier, err)
	}
	return nil
}

// promoteChecklistDataModel walks every checklists.<name> on the page,
// promoting items to the new shape. Returns true when any change was
// made (callers persist only when changes happened).
func promoteChecklistDataModel(fm map[string]any, ulids ulid.Generator) bool {
	checklists, ok := fm[checklistsFMKey].(map[string]any)
	if !ok {
		return false
	}
	wikiChecklists := ensureNestedMap(fm, wikiFMKey, checklistsFMKey)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	changed := false
	for name, raw := range checklists {
		listMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if promoteList(name, listMap, wikiChecklists, ulids, now) {
			checklists[name] = listMap
			changed = true
		}
	}

	if changed {
		fm[checklistsFMKey] = checklists
	}
	return changed
}

// promoteList promotes one named checklist on the page. Returns true if
// the list was newly migrated; false if it was already stamped.
func promoteList(name string, listMap, wikiChecklists map[string]any, ulids ulid.Generator, now string) bool {
	wikiList := ensureMapInParent(wikiChecklists, name)
	if alreadyMigrated(wikiList) {
		return false
	}

	wikiItems := ensureMapInParent(wikiList, itemsFMKey)
	items, ok := listMap[itemsFMKey].([]any)
	if !ok {
		items = nil
	}
	for idx, raw := range items {
		itemMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uid := promoteItemUserData(itemMap, idx, ulids)
		stampItemMetadata(wikiItems, uid, now)
		items[idx] = itemMap
	}
	listMap[itemsFMKey] = items

	stampListMetadata(wikiList, now)
	return true
}

func alreadyMigrated(wikiList map[string]any) bool {
	flag, ok := wikiList[migratedDataModelKey].(bool)
	return ok && flag
}

// promoteItemUserData fills in uid + sort_order on the item, returning
// the (possibly newly assigned) uid.
func promoteItemUserData(itemMap map[string]any, idx int, ulids ulid.Generator) string {
	uid, ok := itemMap["uid"].(string)
	if !ok {
		uid = ""
	}
	if uid == "" {
		uid = ulids.NewULID()
		itemMap["uid"] = uid
	}
	if _, hasOrder := itemMap["sort_order"]; !hasOrder {
		itemMap["sort_order"] = int64(idx+1) * dataModelSortOrderStep
	}
	return uid
}

// stampItemMetadata writes per-item wiki-managed metadata keyed by uid.
// Pre-existing items can't be retroactively attributed; the migration
// records automated=false so eventual sync is conservative.
func stampItemMetadata(wikiItems map[string]any, uid, now string) {
	meta := ensureMapInParent(wikiItems, uid)
	if _, has := meta["created_at"]; !has {
		meta["created_at"] = now
	}
	if _, has := meta[updatedAtFMKey]; !has {
		meta[updatedAtFMKey] = now
	}
	if _, has := meta["automated"]; !has {
		meta["automated"] = false
	}
}

// stampListMetadata sets the migrated flag and initializes per-list
// sync state if absent.
func stampListMetadata(wikiList map[string]any, now string) {
	wikiList[migratedDataModelKey] = true
	if _, has := wikiList["sync_token"]; !has {
		wikiList["sync_token"] = int64(0)
	}
	if _, has := wikiList[updatedAtFMKey]; !has {
		wikiList[updatedAtFMKey] = now
	}
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
