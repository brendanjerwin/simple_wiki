// Package eager: connectors subscriptions[] -> bindings[] migration.
//
// Phase 7 of the SyncEngine extraction (#998 precursor): rewrites the
// pre-engine wiki.connectors.<kind>.subscriptions[] frontmatter shape
// into the new wiki.connectors.<kind>.bindings[] shape. Each entry's
// engine-owned fields stay top-level (page, list_name, remote_handle,
// state, last_synced_seq, paused_reason, paused_at, bound_at,
// last_successful_sync_at); every other key on the legacy entry —
// including all adapter-specific bookkeeping — collapses into a new
// adapter_state subtree on the entry.
//
// The migration is idempotent: pages already in the new shape are
// skipped. Pages with both shapes (transitional state from running
// the engine partway through Phase 4-2/4-3 against legacy data) keep
// the new-shape data and drop the legacy key.
//
// The translation rule here is the canonical one — the
// FrontmatterBindingStore's pre-Phase-7 dual-read used the same rule.
// After this migration ships, the dual-read goes away (the binding
// store reads only the new shape) and this file becomes the single
// source of the legacy → new translation. Once every wiki on disk has
// been booted at least once with this migration enabled, the file is
// dead code and can be deleted.
//
// The system_template_namespace_migration.go is the architectural
// template — see that file for the cross-cutting concerns (system-page
// guard, scan job pattern, frontmatter parsing).
package eager

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Frontmatter keys touched by this migration. The connector state
// lives at wiki.connectors.<kind>.* on the user's profile page.
//
// These constants are local to this file (rather than imported from
// internal/connectors/engine) because the migrations package must not
// take a build-time dependency on the engine package, and the engine
// package's pre-Phase-7 legacy-key constants are about to be deleted
// once the dual-read goes away.
const (
	connectorsKeyWiki         = "wiki"
	connectorsKeyConnectors   = "connectors"
	connectorsKeyBindings     = "bindings"      // new shape
	connectorsKeySubscriptions = "subscriptions" // legacy shape
	connectorsKeyAdapterState = "adapter_state"

	// Engine-owned per-binding fields (new shape). These are the keys
	// that stay top-level on each translated entry; every other key on
	// a legacy entry rides through into the entry's adapter_state map.
	connectorsKeyPage                 = "page"
	connectorsKeyListName             = "list_name"
	connectorsKeyRemoteHandle         = "remote_handle"
	connectorsKeyRemoteListTitle      = "remote_list_title"
	connectorsKeyState                = "state"
	connectorsKeyPausedReason         = "paused_reason"
	connectorsKeyPausedAt             = "paused_at"
	connectorsKeyBoundAt              = "bound_at"
	connectorsKeyLastSyncedSeq        = "last_synced_seq"
	connectorsKeyLastSuccessfulSyncAt = "last_successful_sync_at"

	// Legacy-shape engine field aliases (read-only). These map onto
	// new-shape engine fields during translation and are NOT preserved
	// in adapter_state.
	connectorsLegacyKeyRemoteListID = "remote_list_id"
	connectorsLegacyKeySubscribedAt = "subscribed_at"
)

// engineOwnedLegacyKeys is the set of keys on a legacy subscription
// entry that the migration interprets as engine-owned. Keys in this
// set become top-level fields on the new-shape entry; everything else
// goes into adapter_state. Identical to the rule the pre-Phase-7
// FrontmatterBindingStore.decodeLegacyBinding used — keep them in sync.
var engineOwnedLegacyKeys = map[string]struct{}{
	connectorsKeyPage:                 {},
	connectorsKeyListName:             {},
	connectorsKeyRemoteHandle:         {},
	connectorsLegacyKeyRemoteListID:   {},
	connectorsKeyRemoteListTitle:      {},
	connectorsKeyState:                {},
	connectorsKeyPausedReason:         {},
	connectorsKeyPausedAt:             {},
	connectorsKeyBoundAt:              {},
	connectorsLegacyKeySubscribedAt:   {},
	connectorsKeyLastSyncedSeq:        {},
	connectorsKeyLastSuccessfulSyncAt: {},
}

// ConnectorsSubscriptionsToBindingsMigrationScanJob walks the data dir
// and enqueues a per-page job for every profile page that still has a
// legacy wiki.connectors.<kind>.subscriptions[] entry under any
// connector kind.
type ConnectorsSubscriptionsToBindingsMigrationScanJob struct {
	scanner       DataDirScanner
	coordinator   *jobs.JobQueueCoordinator
	readerMutator wikipage.PageReaderMutator
}

// NewConnectorsSubscriptionsToBindingsMigrationScanJob constructs the scan job.
func NewConnectorsSubscriptionsToBindingsMigrationScanJob(
	scanner DataDirScanner,
	coordinator *jobs.JobQueueCoordinator,
	readerMutator wikipage.PageReaderMutator,
) *ConnectorsSubscriptionsToBindingsMigrationScanJob {
	return &ConnectorsSubscriptionsToBindingsMigrationScanJob{
		scanner:       scanner,
		coordinator:   coordinator,
		readerMutator: readerMutator,
	}
}

// GetName returns the job name for queue logging.
func (*ConnectorsSubscriptionsToBindingsMigrationScanJob) GetName() string {
	return "ConnectorsSubscriptionsToBindingsMigrationScanJob"
}

// Execute scans .md files and enqueues per-page migration jobs.
func (j *ConnectorsSubscriptionsToBindingsMigrationScanJob) Execute() error {
	if !j.scanner.DataDirExists() {
		return nil
	}

	files, err := j.scanner.ListMDFiles()
	if err != nil {
		return fmt.Errorf("list .md files: %w", err)
	}

	seen := make(map[string]struct{})
	for _, filename := range files {
		identifier, fm, ok := extractConnectorsMigrationFrontmatter(j.scanner, filename)
		if !ok {
			continue
		}
		if _, dup := seen[identifier]; dup {
			continue
		}
		seen[identifier] = struct{}{}

		if !pageNeedsConnectorsMigration(fm) {
			continue
		}

		migrationJob := NewConnectorsSubscriptionsToBindingsMigrationJob(j.readerMutator, identifier)
		if err := j.coordinator.EnqueueJob(migrationJob); err != nil {
			return fmt.Errorf("enqueue connectors subscriptions->bindings migration for %s: %w", identifier, err)
		}
	}
	return nil
}

// ConnectorsSubscriptionsToBindingsMigrationJob rewrites a single
// page's legacy wiki.connectors.<kind>.subscriptions[] entries into
// the new wiki.connectors.<kind>.bindings[] shape, translating each
// entry to route engine-owned fields top-level and the rest into an
// adapter_state subtree.
type ConnectorsSubscriptionsToBindingsMigrationJob struct {
	readerMutator wikipage.PageReaderMutator
	identifier    string
}

// NewConnectorsSubscriptionsToBindingsMigrationJob constructs the per-page job.
func NewConnectorsSubscriptionsToBindingsMigrationJob(rw wikipage.PageReaderMutator, id string) *ConnectorsSubscriptionsToBindingsMigrationJob {
	return &ConnectorsSubscriptionsToBindingsMigrationJob{readerMutator: rw, identifier: id}
}

// GetName returns the job name (includes the identifier for queue tracing).
func (j *ConnectorsSubscriptionsToBindingsMigrationJob) GetName() string {
	return fmt.Sprintf("ConnectorsSubscriptionsToBindingsMigrationJob-%s", j.identifier)
}

// Execute reads the page's frontmatter, rewrites every connector
// kind's subscriptions[] into bindings[], and writes back via the
// standard write path. No-op (no write) when the page has nothing to
// migrate.
func (j *ConnectorsSubscriptionsToBindingsMigrationJob) Execute() error {
	id := wikipage.PageIdentifier(j.identifier)
	_, fm, err := j.readerMutator.ReadFrontMatter(id)
	if err != nil {
		return fmt.Errorf("read frontmatter for %s: %w", j.identifier, err)
	}
	if fm == nil {
		return nil
	}

	if !rewriteConnectorsSubscriptions(fm) {
		return nil
	}

	if err := j.readerMutator.WriteFrontMatter(id, fm); err != nil {
		return fmt.Errorf("write migrated frontmatter for %s: %w", j.identifier, err)
	}
	return nil
}

// pageNeedsConnectorsMigration reports whether the page has any
// connector kind whose subtree still carries a subscriptions[] key
// (regardless of whether bindings[] is also present). The migration
// rewrites the legacy key in either case — the new shape wins and
// the legacy key is deleted.
//
// A page with only the new shape (bindings[]) is skipped — the
// rewrite is a no-op for it.
func pageNeedsConnectorsMigration(fm map[string]any) bool {
	if fm == nil {
		return false
	}
	wiki, ok := fm[connectorsKeyWiki].(map[string]any)
	if !ok {
		return false
	}
	conns, ok := wiki[connectorsKeyConnectors].(map[string]any)
	if !ok {
		return false
	}
	for _, kindRaw := range conns {
		kindMap, ok := kindRaw.(map[string]any)
		if !ok {
			continue
		}
		if _, hasLegacy := kindMap[connectorsKeySubscriptions]; hasLegacy {
			return true
		}
	}
	return false
}

// extractConnectorsMigrationFrontmatter pulls just the frontmatter from
// a .md file. Mirrors the helpers in the sibling migrations.
func extractConnectorsMigrationFrontmatter(scanner DataDirScanner, filename string) (string, map[string]any, bool) {
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

// rewriteConnectorsSubscriptions walks every wiki.connectors.<kind>
// subtree on fm and, for each kind that has a subscriptions[] entry,
// produces the new bindings[] equivalent and deletes the legacy key.
// Returns true when any change was made (the caller writes the page
// only on change so the migration stays idempotent across reboots).
//
// When BOTH shapes are present on a kind, the existing bindings[] is
// kept verbatim and the legacy subscriptions[] is dropped. The
// migration does NOT merge entries across shapes — it assumes the new
// shape is authoritative when present (this matches the pre-Phase-7
// FrontmatterBindingStore's "new shape wins" rule).
func rewriteConnectorsSubscriptions(fm map[string]any) bool {
	if fm == nil {
		return false
	}
	wiki, ok := fm[connectorsKeyWiki].(map[string]any)
	if !ok {
		return false
	}
	conns, ok := wiki[connectorsKeyConnectors].(map[string]any)
	if !ok {
		return false
	}

	changed := false
	for _, kindRaw := range conns {
		kindMap, ok := kindRaw.(map[string]any)
		if !ok {
			continue
		}
		legacyRaw, hasLegacy := kindMap[connectorsKeySubscriptions]
		if !hasLegacy {
			continue
		}

		// New-shape data wins on conflict; legacy is dropped without
		// being translated.
		if _, hasNew := kindMap[connectorsKeyBindings]; !hasNew {
			translated := translateLegacyEntries(legacyRaw)
			// Empty legacy array → no bindings[] written at all (the
			// connector subtree is left without either key, which is
			// the same shape a never-bound profile carries).
			if len(translated) > 0 {
				kindMap[connectorsKeyBindings] = translated
			}
		}

		delete(kindMap, connectorsKeySubscriptions)
		changed = true
	}
	return changed
}

// translateLegacyEntries converts a legacy subscriptions[] slice into
// a new-shape bindings[] slice. Non-map entries are dropped silently
// (they were unreachable by the engine anyway — every code path that
// wrote subscriptions[] wrote map entries).
func translateLegacyEntries(legacy any) []any {
	rawList, ok := legacy.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(rawList))
	for _, entry := range rawList {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, translateLegacyEntry(m))
	}
	return out
}

// translateLegacyEntry takes one legacy subscription map and produces
// its new-shape equivalent. Engine-owned keys (the legacy
// remote_list_id and subscribed_at aliases included) are routed to
// their new-shape names at the top of the entry; everything else
// rides through into the entry's adapter_state subtree verbatim.
//
// This is the load-bearing rule: the dual-read in the pre-Phase-7
// FrontmatterBindingStore used exactly this allowlist. Both code
// paths must produce identical Bindings for the same legacy input.
func translateLegacyEntry(m map[string]any) map[string]any {
	out := map[string]any{}

	// Engine-owned fields whose new-shape names match the legacy
	// names round-trip verbatim.
	copyIfPresent(m, out, connectorsKeyPage)
	copyIfPresent(m, out, connectorsKeyListName)
	copyIfPresent(m, out, connectorsKeyRemoteListTitle)
	copyIfPresent(m, out, connectorsKeyState)
	copyIfPresent(m, out, connectorsKeyPausedReason)
	copyIfPresent(m, out, connectorsKeyPausedAt)
	copyIfPresent(m, out, connectorsKeyLastSyncedSeq)
	copyIfPresent(m, out, connectorsKeyLastSuccessfulSyncAt)

	// remote_handle: prefer the new key if a transitional row carried
	// it; otherwise the legacy remote_list_id. Either way the new-shape
	// entry stores it at remote_handle.
	if v, ok := m[connectorsKeyRemoteHandle]; ok {
		out[connectorsKeyRemoteHandle] = v
	} else if v, ok := m[connectorsLegacyKeyRemoteListID]; ok {
		out[connectorsKeyRemoteHandle] = v
	}

	// bound_at: prefer the new key if present; otherwise legacy subscribed_at.
	if v, ok := m[connectorsKeyBoundAt]; ok {
		out[connectorsKeyBoundAt] = v
	} else if v, ok := m[connectorsLegacyKeySubscribedAt]; ok {
		out[connectorsKeyBoundAt] = v
	}

	// Adapter state: every key NOT in the engine-owned allowlist rides
	// through verbatim under the adapter_state subtree.
	adapterState := map[string]any{}
	for k, v := range m {
		if _, owned := engineOwnedLegacyKeys[k]; owned {
			continue
		}
		adapterState[k] = v
	}
	if len(adapterState) > 0 {
		out[connectorsKeyAdapterState] = adapterState
	}

	return out
}

// copyIfPresent is a small helper that copies key from src to dst
// when present in src. Keeps translateLegacyEntry tidy.
func copyIfPresent(src, dst map[string]any, key string) {
	if v, ok := src[key]; ok {
		dst[key] = v
	}
}
