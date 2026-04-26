// Package eager: system/template namespace migration.
//
// Moves the legacy top-level `system` and `template` frontmatter flags into
// the reserved `wiki.*` namespace (per #997 and ADR-0010). Each migrated
// page is stamped `wiki.migrated_namespaces = true` for idempotency so
// repeat scans skip it.
//
// The checklist data-model migration in checklist_data_model_migration.go
// is the template for this one — see that file for the cross-cutting
// concerns (system-page guard, scan job pattern, frontmatter parsing).
package eager

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// migratedNamespacesKey is the per-page flag set under wiki.* after the
// legacy system/template keys have been moved. Its presence is the signal
// that no further work is needed on this page.
const migratedNamespacesKey = "migrated_namespaces"

// Frontmatter keys touched by this migration.
const (
	legacySystemKey   = "system"
	legacyTemplateKey = "template"
	wikiNamespaceKey  = "wiki"
	systemFlagKey     = "system"
	templateFlagKey   = "template"
)

// SystemTemplateNamespaceMigrationScanJob walks the data dir and enqueues a
// per-page job for every page that still has a legacy top-level `system` or
// `template` frontmatter key.
type SystemTemplateNamespaceMigrationScanJob struct {
	scanner       DataDirScanner
	coordinator   *jobs.JobQueueCoordinator
	readerMutator wikipage.PageReaderMutator
}

// NewSystemTemplateNamespaceMigrationScanJob constructs the scan job.
func NewSystemTemplateNamespaceMigrationScanJob(
	scanner DataDirScanner,
	coordinator *jobs.JobQueueCoordinator,
	readerMutator wikipage.PageReaderMutator,
) *SystemTemplateNamespaceMigrationScanJob {
	return &SystemTemplateNamespaceMigrationScanJob{
		scanner:       scanner,
		coordinator:   coordinator,
		readerMutator: readerMutator,
	}
}

// GetName returns the job name for queue logging.
func (*SystemTemplateNamespaceMigrationScanJob) GetName() string {
	return "SystemTemplateNamespaceMigrationScanJob"
}

// Execute scans .md files and enqueues per-page migration jobs.
func (j *SystemTemplateNamespaceMigrationScanJob) Execute() error {
	if !j.scanner.DataDirExists() {
		return nil
	}

	files, err := j.scanner.ListMDFiles()
	if err != nil {
		return fmt.Errorf("list .md files: %w", err)
	}

	seen := make(map[string]struct{})
	for _, filename := range files {
		identifier, fm, ok := extractNamespaceMigrationFrontmatter(j.scanner, filename)
		if !ok {
			continue
		}
		if _, dup := seen[identifier]; dup {
			continue
		}
		seen[identifier] = struct{}{}

		if !pageNeedsNamespaceMigration(fm) {
			continue
		}

		migrationJob := NewSystemTemplateNamespaceMigrationJob(j.readerMutator, identifier)
		if err := j.coordinator.EnqueueJob(migrationJob); err != nil {
			return fmt.Errorf("enqueue namespace migration for %s: %w", identifier, err)
		}
	}
	return nil
}

// SystemTemplateNamespaceMigrationJob promotes a single page's legacy
// system/template flags into wiki.* and removes the legacy keys.
type SystemTemplateNamespaceMigrationJob struct {
	readerMutator wikipage.PageReaderMutator
	identifier    string
}

// NewSystemTemplateNamespaceMigrationJob constructs the per-page job.
func NewSystemTemplateNamespaceMigrationJob(rw wikipage.PageReaderMutator, id string) *SystemTemplateNamespaceMigrationJob {
	return &SystemTemplateNamespaceMigrationJob{readerMutator: rw, identifier: id}
}

// GetName returns the job name (includes the identifier for queue tracing).
func (j *SystemTemplateNamespaceMigrationJob) GetName() string {
	return fmt.Sprintf("SystemTemplateNamespaceMigrationJob-%s", j.identifier)
}

// Execute reads the page's frontmatter, moves legacy flags into wiki.*, and
// writes back via the standard write path.
func (j *SystemTemplateNamespaceMigrationJob) Execute() error {
	id := wikipage.PageIdentifier(j.identifier)
	_, fm, err := j.readerMutator.ReadFrontMatter(id)
	if err != nil {
		return fmt.Errorf("read frontmatter for %s: %w", j.identifier, err)
	}
	if fm == nil {
		fm = wikipage.FrontMatter{}
	}

	wikiSubtree, _ := fm[wikiNamespaceKey].(map[string]any)
	if wikiSubtree == nil {
		wikiSubtree = map[string]any{}
	}

	// Move legacy keys into the reserved subtree only when the new key is
	// not already authoritative there. The new key always wins.
	if legacyVal, hasLegacy := fm[legacySystemKey]; hasLegacy {
		if _, hasNew := wikiSubtree[systemFlagKey]; !hasNew {
			wikiSubtree[systemFlagKey] = legacyVal
		}
		delete(fm, legacySystemKey)
	}
	if legacyVal, hasLegacy := fm[legacyTemplateKey]; hasLegacy {
		if _, hasNew := wikiSubtree[templateFlagKey]; !hasNew {
			wikiSubtree[templateFlagKey] = legacyVal
		}
		delete(fm, legacyTemplateKey)
	}

	wikiSubtree[migratedNamespacesKey] = true
	fm[wikiNamespaceKey] = wikiSubtree

	if err := j.readerMutator.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf("write migrated frontmatter for %s: %w", j.identifier, err)
	}
	return nil
}

// pageNeedsNamespaceMigration reports whether the page still has either
// legacy top-level flag and therefore needs a rewrite.
//
// Pages that have already been migrated (no legacy keys present) are
// skipped — `wiki.migrated_namespaces` is the recovery marker but its
// absence is not by itself a reason to migrate (a brand-new page may have
// been authored against the new layout without ever owning a legacy key).
func pageNeedsNamespaceMigration(fm map[string]any) bool {
	if fm == nil {
		return false
	}
	if _, ok := fm[legacySystemKey]; ok {
		return true
	}
	if _, ok := fm[legacyTemplateKey]; ok {
		return true
	}
	return false
}

// extractNamespaceMigrationFrontmatter pulls just the frontmatter from a
// .md file. Mirrors the helpers in the sibling migrations.
func extractNamespaceMigrationFrontmatter(scanner DataDirScanner, filename string) (string, map[string]any, bool) {
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
