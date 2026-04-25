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
	checklists, ok := fm["checklists"].(map[string]any)
	if !ok {
		return false
	}
	wikiChecklists := readNestedMap(fm, "wiki", "checklists")
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
// fills in missing uid/sort_order, populates wiki.checklists.<name>.items.<uid>.{created_at, updated_at},
// and stamps wiki.checklists.<name>.migrated_data_model = true. Returns
// true when any change was made (callers persist only when changes happened).
func promoteChecklistDataModel(fm map[string]any, ulids ulid.Generator) bool {
	checklists, ok := fm["checklists"].(map[string]any)
	if !ok {
		return false
	}
	wikiChecklists := ensureNestedMap(fm, "wiki", "checklists")
	now := time.Now().UTC().Format(time.RFC3339Nano)

	changed := false
	for name, raw := range checklists {
		listMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		wikiList, _ := wikiChecklists[name].(map[string]any)
		if wikiList == nil {
			wikiList = make(map[string]any)
			wikiChecklists[name] = wikiList
		}

		// Skip lists that have already been migrated.
		if flag, ok := wikiList[migratedDataModelKey].(bool); ok && flag {
			continue
		}

		wikiItems, _ := wikiList["items"].(map[string]any)
		if wikiItems == nil {
			wikiItems = make(map[string]any)
			wikiList["items"] = wikiItems
		}

		items, _ := listMap["items"].([]any)
		for idx, raw := range items {
			itemMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			uid, _ := itemMap["uid"].(string)
			if uid == "" {
				uid = ulids.NewULID()
				itemMap["uid"] = uid
			}
			if _, hasOrder := itemMap["sort_order"]; !hasOrder {
				itemMap["sort_order"] = int64(idx+1) * dataModelSortOrderStep
			}
			meta, _ := wikiItems[uid].(map[string]any)
			if meta == nil {
				meta = make(map[string]any)
				wikiItems[uid] = meta
			}
			if _, has := meta["created_at"]; !has {
				meta["created_at"] = now
			}
			if _, has := meta["updated_at"]; !has {
				meta["updated_at"] = now
			}
			if _, has := meta["automated"]; !has {
				// Pre-existing items can't be retroactively attributed; mark as
				// automated=false so the eventual sync is conservative.
				meta["automated"] = false
			}
			items[idx] = itemMap
		}
		listMap["items"] = items
		checklists[name] = listMap

		// Stamp the migrated flag — whether or not any item needed work, the
		// list is now under the new model.
		wikiList[migratedDataModelKey] = true
		// Initialize sync_token if absent.
		if _, has := wikiList["sync_token"]; !has {
			wikiList["sync_token"] = int64(0)
		}
		if _, has := wikiList["updated_at"]; !has {
			wikiList["updated_at"] = now
		}
		changed = true
	}

	if changed {
		fm["checklists"] = checklists
	}
	return changed
}

// readNestedMap walks fm[k1][k2][...] returning the deepest map or nil.
func readNestedMap(fm map[string]any, keys ...string) map[string]any {
	cur := fm
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		next, _ := cur[k].(map[string]any)
		if next == nil {
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
		next, _ := cur[k].(map[string]any)
		if next == nil {
			next = make(map[string]any)
			cur[k] = next
		}
		cur = next
	}
	return cur
}
