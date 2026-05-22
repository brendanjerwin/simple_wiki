package server

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// readPathWritesTotal counts every time the lazy-migration chain saves
// back to disk during a `ReadPage` (i.e., the bug class this refactor is
// eliminating). The counter is the post-deploy verification signal: after
// Phase 4 flips the canonicalizer from noop to real, this counter must
// drop to zero. After Phase 5 deletes the save-on-read chain entirely,
// the counter has no incrementer and stays at zero permanently.
//
// Lives in the server package because that's where the lazy-save side
// effect lives during the transition. After Phase 6 (lazy package
// deletion) this file is deleted with the rest of the dead code.

var (
	readPathWritesInitOnce sync.Once
	readPathWritesCounter  metric.Int64Counter
)

func initReadPathWritesMetric() {
	readPathWritesInitOnce.Do(func() {
		meter := otel.Meter("simple_wiki/migration_save_on_read")
		if c, err := meter.Int64Counter(
			"wiki_read_path_writes_total",
			metric.WithDescription("Saves triggered by the lazy-migration chain during a page read"),
			metric.WithUnit("{save}"),
		); err == nil {
			readPathWritesCounter = c
		}
	})
}

// recordReadPathWrite increments the counter. Called from inside
// applyMigrationsForPage when the migration chain decides to save back to
// disk. If the counter failed to initialize (only possible on a malformed
// instrument name — ours is a constant), the call is a no-op.
func recordReadPathWrite() {
	initReadPathWritesMetric()
	if readPathWritesCounter != nil {
		readPathWritesCounter.Add(context.Background(), 1)
	}
}
