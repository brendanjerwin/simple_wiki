// Package eager: Keep-bridge fingerprint migration.
//
// Backfills per-item synced_fp baselines onto legacy Keep-bridge
// bindings written before the fingerprint-rewrite landed. Without
// this migration those bindings would silently rebaseline on the
// first cron tick (the "lazy first-tick" approach the project plan
// explicitly rejected). The eager job pulls Keep once per legacy
// binding, applies the agreement-or-Keep-wins rule, and stamps
// `MigratedFingerprints = true` so subsequent ticks see a real
// merge-base. See plan §"Migration" for the full rationale.
package eager

import (
	"context"
	"fmt"
	"strings"

	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
)

// KeepBridgeFingerprintMigrator is the subset of keepsync.Connector
// the per-binding migration job calls. Stated as an interface so
// tests can substitute a fake without spinning up a real Keep
// client. Production wires *keepsync.Connector here at startup.
type KeepBridgeFingerprintMigrator interface {
	MigrateBindingFingerprints(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error
}

// KeepBridgeBindingStateLoader is the subset of keepsync.BindingStore
// the scan job calls to identify legacy bindings on a profile page.
// Production wires *keepsync.BindingStore; tests substitute a fake.
type KeepBridgeBindingStateLoader interface {
	LoadState(profileID wikipage.PageIdentifier) (keepsync.ConnectorState, error)
}

// KeepBridgeFingerprintMigrationScanJob walks the data dir, parses
// each page's TOML frontmatter, and enqueues a per-binding
// migration job for every binding whose MigratedFingerprints flag
// is unset. Mirrors ChecklistTagSyntaxMigrationScanJob's structure;
// the per-page guard reads `wiki.connectors.google_keep.email` to
// gate the keepsync.BindingStore lookup.
type KeepBridgeFingerprintMigrationScanJob struct {
	scanner     DataDirScanner
	coordinator *jobs.JobQueueCoordinator
	migrator    KeepBridgeFingerprintMigrator
	loader      KeepBridgeBindingStateLoader
}

// NewKeepBridgeFingerprintMigrationScanJob constructs the scan job.
// The migrator and loader are typically *keepsync.Connector and
// *keepsync.BindingStore respectively; the interfaces let tests
// inject fakes.
func NewKeepBridgeFingerprintMigrationScanJob(
	scanner DataDirScanner,
	coordinator *jobs.JobQueueCoordinator,
	migrator KeepBridgeFingerprintMigrator,
	loader KeepBridgeBindingStateLoader,
) *KeepBridgeFingerprintMigrationScanJob {
	return &KeepBridgeFingerprintMigrationScanJob{
		scanner:     scanner,
		coordinator: coordinator,
		migrator:    migrator,
		loader:      loader,
	}
}

// GetName returns the queue name. The coordinator routes jobs by
// queue name; this scan job runs on its own queue (not the per-
// binding migration queue) so the scan completes before any per-
// binding job dequeues.
func (*KeepBridgeFingerprintMigrationScanJob) GetName() string {
	return "KeepBridgeFingerprintMigrationScanJob"
}

// Execute scans .md files, picks the ones with a Keep connector
// configured, decodes their bindings, and enqueues a migration job
// for each binding whose MigratedFingerprints flag is unset.
func (j *KeepBridgeFingerprintMigrationScanJob) Execute() error {
	if !j.scanner.DataDirExists() {
		return nil
	}

	files, err := j.scanner.ListMDFiles()
	if err != nil {
		return fmt.Errorf("list .md files: %w", err)
	}

	// Track profile IDs we've already enqueued for so duplicate
	// .md files (e.g., munged + original encodings of the same
	// identifier) don't produce duplicate jobs.
	seen := make(map[wikipage.PageIdentifier]struct{})
	for _, filename := range files {
		profileID, hasConnector := j.profileIDIfHasKeepConnector(filename)
		if !hasConnector {
			continue
		}
		if _, dup := seen[profileID]; dup {
			continue
		}
		seen[profileID] = struct{}{}

		state, lerr := j.loader.LoadState(profileID)
		if lerr != nil {
			// Skip pages whose state can't be decoded — the per-
			// binding job is the wrong place to surface that;
			// the next regular sync tick will. Continuing keeps
			// the rest of the data dir's migrations moving.
			continue
		}
		for _, b := range state.Bindings {
			if b.MigratedFingerprints {
				continue
			}
			migrationJob := NewKeepBridgeFingerprintMigrationJob(j.migrator, profileID, b.Page, b.ListName)
			if eerr := j.coordinator.EnqueueJob(migrationJob); eerr != nil {
				return fmt.Errorf("enqueue keep-bridge fingerprint migration for %s/%s/%s: %w",
					profileID, b.Page, b.ListName, eerr)
			}
			// Mark the binding as migration-pending so operators can
			// see the un-drained queue via the metric. The migration
			// job itself clears this gauge on successful completion.
			keepsync.SetMigrationPendingMetric(context.Background(), profileID, b.Page, b.ListName, true)
		}
	}
	return nil
}

// profileIDIfHasKeepConnector parses just the TOML frontmatter from
// a .md file and returns (profileID, true) iff the page has a
// configured Keep connector (signaled by wiki.connectors.google_
// keep.email being a non-empty string). The profileID is taken
// from the frontmatter `identifier` field — same convention the
// other eager scan jobs use.
func (j *KeepBridgeFingerprintMigrationScanJob) profileIDIfHasKeepConnector(filename string) (wikipage.PageIdentifier, bool) {
	mdData, err := j.scanner.ReadMDFile(filename)
	if err != nil {
		return "", false
	}

	content := string(mdData)
	if !strings.HasPrefix(content, "+++") {
		return "", false
	}

	parts := strings.SplitN(content, "+++", tomlFrontmatterParts)
	if len(parts) < tomlFrontmatterParts {
		return "", false
	}

	fm := map[string]any{}
	if uerr := toml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); uerr != nil {
		return "", false
	}

	wiki, ok := fm["wiki"].(map[string]any)
	if !ok {
		return "", false
	}
	connectors, ok := wiki["connectors"].(map[string]any)
	if !ok {
		return "", false
	}
	gk, ok := connectors["google_keep"].(map[string]any)
	if !ok {
		return "", false
	}
	email, ok := gk["email"].(string)
	if !ok || email == "" {
		return "", false
	}
	id, ok := fm["identifier"].(string)
	if !ok || id == "" {
		return "", false
	}
	return wikipage.PageIdentifier(id), true
}

// KeepBridgeFingerprintMigrationJob runs the per-binding rebaseline:
// pull Keep once, apply agreement-or-Keep-wins to populate synced_fp,
// drop unpaired entries, stamp KeepCursor + MigratedFingerprints,
// persist. The heavy lifting lives in
// keepsync.Connector.MigrateBindingFingerprints (which handles the
// per-profile mutex, the network round-trip, and the rule); this
// job is the queue-shaped wrapper around that single call so a
// failure surfaces as a queue retry rather than a swallowed error.
type KeepBridgeFingerprintMigrationJob struct {
	migrator  KeepBridgeFingerprintMigrator
	profileID wikipage.PageIdentifier
	page      string
	listName  string
}

// NewKeepBridgeFingerprintMigrationJob constructs a per-binding
// migration job.
func NewKeepBridgeFingerprintMigrationJob(
	migrator KeepBridgeFingerprintMigrator,
	profileID wikipage.PageIdentifier,
	page, listName string,
) *KeepBridgeFingerprintMigrationJob {
	return &KeepBridgeFingerprintMigrationJob{
		migrator:  migrator,
		profileID: profileID,
		page:      page,
		listName:  listName,
	}
}

// GetName returns a per-binding queue name. Each binding gets its
// own queue identity (hashing-style) only conceptually; the
// coordinator's queue routing is by name, so all per-binding jobs
// share the same queue. The trailing identifier is for log
// readability.
func (j *KeepBridgeFingerprintMigrationJob) GetName() string {
	return fmt.Sprintf("KeepBridgeFingerprintMigrationJob-%s-%s-%s",
		j.profileID, j.page, j.listName)
}

// Execute delegates to the keepsync.Connector. A returned error
// triggers the coordinator's retry-with-backoff (per
// pkg/jobs.JobQueueCoordinator semantics); the binding's
// MigratedFingerprints stays false until the migration succeeds,
// keeping the SyncToKeep gate engaged.
func (j *KeepBridgeFingerprintMigrationJob) Execute() error {
	if err := j.migrator.MigrateBindingFingerprints(context.Background(), j.profileID, j.page, j.listName); err != nil {
		return fmt.Errorf("migrate keep-bridge fingerprints for %s/%s/%s: %w",
			j.profileID, j.page, j.listName, err)
	}
	return nil
}
