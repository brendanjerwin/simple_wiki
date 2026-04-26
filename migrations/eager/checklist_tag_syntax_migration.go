// Package eager: checklist tag-syntax migration.
//
// Rewrites legacy `:tag` segments inside checklist item text to the canonical
// `#tag` form. The change is one-shot: each migrated checklist subtree is
// stamped with `migrated_tags_syntax = true` so subsequent scans can skip it
// without re-parsing.
package eager

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
)

// migratedFlagKey is the per-checklist frontmatter flag the migration sets
// after rewriting items. Its presence is the signal that no further work is
// needed on that subtree.
const migratedFlagKey = "migrated_tags_syntax"

// legacyTagPattern matches the OLD `:tag` grammar: a `:` preceded by start-of
// -line or whitespace, followed by tag chars (letters/digits/`-`/`_`).
//
// Go's RE2 does not support lookbehind, so we capture the leading boundary
// and emit it back via a backreference at substitution time.
var legacyTagPattern = regexp.MustCompile(`(^|\s):([\p{L}\p{N}\-_]+)`)

// ChecklistTagSyntaxMigrationScanJob walks the data dir, parses each page's
// frontmatter, and enqueues a per-page migration job for every page that has
// a checklist subtree containing legacy `:tag` items and lacks the migrated
// flag.
type ChecklistTagSyntaxMigrationScanJob struct {
	scanner       DataDirScanner
	coordinator   *jobs.JobQueueCoordinator
	readerMutator wikipage.PageReaderMutator
}

// NewChecklistTagSyntaxMigrationScanJob constructs the scan job.
func NewChecklistTagSyntaxMigrationScanJob(
	scanner DataDirScanner,
	coordinator *jobs.JobQueueCoordinator,
	readerMutator wikipage.PageReaderMutator,
) *ChecklistTagSyntaxMigrationScanJob {
	return &ChecklistTagSyntaxMigrationScanJob{
		scanner:       scanner,
		coordinator:   coordinator,
		readerMutator: readerMutator,
	}
}

// GetName returns the job name for queue logging.
func (*ChecklistTagSyntaxMigrationScanJob) GetName() string {
	return "ChecklistTagSyntaxMigrationScanJob"
}

// Execute scans .md files, looks at each page's checklists subtree, and
// enqueues per-page migration jobs for those that need rewriting.
func (j *ChecklistTagSyntaxMigrationScanJob) Execute() error {
	if !j.scanner.DataDirExists() {
		return nil
	}

	files, err := j.scanner.ListMDFiles()
	if err != nil {
		return fmt.Errorf("list .md files: %w", err)
	}

	seen := make(map[string]struct{})
	for _, filename := range files {
		identifier, fm, ok := j.extractIdentifierAndFrontmatter(filename)
		if !ok {
			continue
		}
		if _, dup := seen[identifier]; dup {
			continue
		}
		seen[identifier] = struct{}{}

		if !pageNeedsChecklistMigration(fm) {
			continue
		}

		migrationJob := NewChecklistTagSyntaxMigrationJob(j.readerMutator, identifier)
		if err := j.coordinator.EnqueueJob(migrationJob); err != nil {
			return fmt.Errorf("enqueue checklist tag migration for %s: %w", identifier, err)
		}
	}
	return nil
}

// extractIdentifierAndFrontmatter parses just the TOML frontmatter from a .md
// file, returning the identifier (from the frontmatter) and the parsed
// frontmatter. Returns ok=false on any parse error or missing identifier so
// the scan can skip silently rather than abort.
func (j *ChecklistTagSyntaxMigrationScanJob) extractIdentifierAndFrontmatter(filename string) (string, map[string]any, bool) {
	mdData, err := j.scanner.ReadMDFile(filename)
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

// pageNeedsChecklistMigration reports whether the given frontmatter has at
// least one `checklists.<name>` subtree that:
//   - has not been flagged migrated, AND
//   - contains an item whose `text` matches the legacy `:tag` grammar.
//
// Pages without `checklists` always return false. System pages (system =
// true) always return false because they ship with the wiki binary and are
// owned by syspage.Sync; the system-page guard rejects user writes to them
// at the gRPC layer, and any rewrite here would be undone on the next
// startup sync. Skipping is purely defensive.
func pageNeedsChecklistMigration(fm map[string]any) bool {
	if wikipage.IsSystemPage(fm) {
		return false
	}
	checklists, ok := fm["checklists"].(map[string]any)
	if !ok {
		return false
	}
	for _, list := range checklists {
		listMap, ok := list.(map[string]any)
		if !ok {
			continue
		}
		if flag, isBool := listMap[migratedFlagKey].(bool); isBool && flag {
			continue
		}
		items, ok := listMap["items"].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, isString := itemMap["text"].(string)
			if !isString {
				continue
			}
			if legacyTagPattern.MatchString(text) {
				return true
			}
		}
	}
	return false
}

// ChecklistTagSyntaxMigrationJob rewrites a single page's checklist items
// from `:tag` to `#tag` and stamps `migrated_tags_syntax = true` on every
// affected checklist subtree.
type ChecklistTagSyntaxMigrationJob struct {
	readerMutator wikipage.PageReaderMutator
	identifier    string
}

// NewChecklistTagSyntaxMigrationJob constructs the per-page migration job.
func NewChecklistTagSyntaxMigrationJob(rw wikipage.PageReaderMutator, id string) *ChecklistTagSyntaxMigrationJob {
	return &ChecklistTagSyntaxMigrationJob{readerMutator: rw, identifier: id}
}

// GetName returns the job name (includes the identifier for queue tracing).
func (j *ChecklistTagSyntaxMigrationJob) GetName() string {
	return fmt.Sprintf("ChecklistTagSyntaxMigrationJob-%s", j.identifier)
}

// Execute reads the page's frontmatter, rewrites every checklist item's text
// from `:tag` to `#tag`, marks each checklist as migrated, and writes the
// frontmatter back via the standard write path.
func (j *ChecklistTagSyntaxMigrationJob) Execute() error {
	id := wikipage.PageIdentifier(j.identifier)
	_, fm, err := j.readerMutator.ReadFrontMatter(id)
	if err != nil {
		return fmt.Errorf("read frontmatter for %s: %w", j.identifier, err)
	}
	if fm == nil {
		return nil
	}

	if !rewriteChecklistTags(fm) {
		return nil
	}

	if err := j.readerMutator.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf("write migrated frontmatter for %s: %w", j.identifier, err)
	}
	return nil
}

// rewriteChecklistTags walks `fm.checklists.*.items[].text`, replacing
// `:tag` segments with `#tag`, and marks every modified checklist subtree
// with the migrated flag. Returns true if any change was made.
func rewriteChecklistTags(fm map[string]any) bool {
	checklists, ok := fm["checklists"].(map[string]any)
	if !ok {
		return false
	}

	changed := false
	for name, list := range checklists {
		listMap, ok := list.(map[string]any)
		if !ok {
			continue
		}
		if flag, isBool := listMap[migratedFlagKey].(bool); isBool && flag {
			continue
		}

		listChanged := false
		items, ok := listMap["items"].([]any)
		if ok {
			for _, item := range items {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				text, isString := itemMap["text"].(string)
				if !isString {
					continue
				}
				if rewritten := legacyTagPattern.ReplaceAllString(text, "$1#$2"); rewritten != text {
					itemMap["text"] = rewritten
					listChanged = true
				}
			}
		}

		// Always stamp migrated even if no items needed rewriting — the flag
		// means "we've already considered this subtree under the new grammar"
		// so future scans can skip it.
		listMap[migratedFlagKey] = true
		checklists[name] = listMap
		if listChanged {
			changed = true
		}
	}

	if changed {
		fm["checklists"] = checklists
	}
	return changed
}
