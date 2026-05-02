package sync

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Cursor-reset reasons used as the {reason} attribute on
// keep_bridge_cursor_reset_total. The set is closed (no free-form
// strings) so dashboards can pivot on the label without surprise.
const (
	// CursorResetReasonChronicTruncation fires when the truncation
	// escape hatch trips: TruncatedTickStreak >= threshold AND the
	// tick made no progress (cursor stuck + zero synced_fp updates).
	CursorResetReasonChronicTruncation = "chronic_truncation"

	// CursorResetReasonNonMonotonic fires when a future runtime
	// invariant detects Keep returning a cursor that lex-orders
	// behind the prior one. Reserved name; not yet emitted.
	CursorResetReasonNonMonotonic = "non_monotonic"

	// CursorResetReasonManual fires when an operator drops the
	// cursor via a (future) admin RPC. Reserved name; not yet
	// emitted.
	CursorResetReasonManual = "manual"
)

// Push attempt statuses used as the {status} attribute on
// keep_bridge_push_attempts_total.
const (
	// PushAttemptStatusSuccess: Keep echoed Status=SUCCESS for the
	// node in WriteResults. synced_fp advanced this tick.
	PushAttemptStatusSuccess = "success"

	// PushAttemptStatusFailure: Keep echoed a non-SUCCESS Status
	// (or omitted the node from WriteResults entirely). synced_fp
	// stays at its prior value; PushFailureCount increments.
	PushAttemptStatusFailure = "failure"

	// PushAttemptStatusDeadLetteredSkip: the diff loop's dead-letter
	// gate at the top of the per-item branch fired — the item was
	// not sent to Keep this tick because PushFailureCount >=
	// deadLetterThreshold. Cleared by ClearDeadLetter or a wiki-side
	// re-edit.
	PushAttemptStatusDeadLetteredSkip = "dead_lettered_skip"
)

// Metric attribute keys. Stable across releases — change with care
// (Grafana queries pivot on the exact key strings).
const (
	attrKeepBridgeProfile = "profile"
	attrKeepBridgePage    = "page"
	attrKeepBridgeList    = "list"
	attrKeepBridgeReason  = "reason"
	attrKeepBridgeStatus  = "status"
)

// metrics owns the OTEL instruments for the Keep bridge. A single
// instance is constructed lazily via metricsSingleton — every call
// to recordX/setX from the connector and migration paths goes through
// the same instruments so the OTEL SDK aggregates them cleanly.
//
// The instruments stay nil-tolerant on construction failure: if any
// instrument fails to register, every emit method becomes a no-op
// rather than crashing the sync engine. OTEL can be misconfigured
// in many ways at runtime; metric instrumentation must NEVER block
// real work.
type metrics struct {
	deadLetterCount       metric.Int64Gauge
	cursorResetTotal      metric.Int64Counter
	truncationStreak      metric.Int64Gauge
	pushAttemptsTotal     metric.Int64Counter
	silentRebaselineTotal metric.Int64Counter
	migrationPending      metric.Int64Gauge
}

var (
	metricsOnce     sync.Once
	metricsInstance *metrics
)

// getMetrics returns the package-level metrics singleton, constructing
// it on first call. Safe under concurrent calls — sync.Once guarantees
// single initialization.
func getMetrics() *metrics {
	metricsOnce.Do(func() {
		metricsInstance = newMetrics()
	})
	return metricsInstance
}

// newMetrics constructs every instrument. Failures during instrument
// registration are silently dropped (the field stays nil) so the
// connector keeps running; downstream emit methods nil-check before
// recording.
func newMetrics() *metrics {
	meter := observability.Meter("simple_wiki/keep_bridge")

	deadLetterCount, _ := meter.Int64Gauge(
		"keep_bridge_dead_letter_count",
		metric.WithDescription("Per-binding gauge of items currently dead-lettered (PushFailureCount >= 10)."),
		metric.WithUnit("{item}"),
	)

	cursorResetTotal, _ := meter.Int64Counter(
		"keep_bridge_cursor_reset_total",
		metric.WithDescription("Per-binding counter of KeepCursor resets, by reason."),
		metric.WithUnit("{reset}"),
	)

	truncationStreak, _ := meter.Int64Gauge(
		"keep_bridge_truncation_streak",
		metric.WithDescription("Per-binding current TruncatedTickStreak (consecutive truncated pulls)."),
		metric.WithUnit("{tick}"),
	)

	pushAttemptsTotal, _ := meter.Int64Counter(
		"keep_bridge_push_attempts_total",
		metric.WithDescription("Per-binding counter of per-item push outcomes, by status."),
		metric.WithUnit("{attempt}"),
	)

	silentRebaselineTotal, _ := meter.Int64Counter(
		"keep_bridge_silent_rebaseline_total",
		metric.WithDescription("Per-binding counter of migration-job silent rebaselines (wiki_fp == keep_fp)."),
		metric.WithUnit("{rebaseline}"),
	)

	migrationPending, _ := meter.Int64Gauge(
		"keep_bridge_migration_pending",
		metric.WithDescription("Per-binding gauge of fingerprint-migration pending state (1=un-migrated, 0=migrated)."),
		metric.WithUnit("{binding}"),
	)

	return &metrics{
		deadLetterCount:       deadLetterCount,
		cursorResetTotal:      cursorResetTotal,
		truncationStreak:      truncationStreak,
		pushAttemptsTotal:     pushAttemptsTotal,
		silentRebaselineTotal: silentRebaselineTotal,
		migrationPending:      migrationPending,
	}
}

// bindingAttrs constructs the (profile, page, list) attribute slice
// every per-binding metric carries.
func bindingAttrs(profileID wikipage.PageIdentifier, page, list string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(attrKeepBridgeProfile, string(profileID)),
		attribute.String(attrKeepBridgePage, page),
		attribute.String(attrKeepBridgeList, list),
	}
}

// recordDeadLetterCount snapshots the current dead-letter cardinality
// for one binding. Called at the end of SyncToKeep so a freshly-
// cleared dead-letter (via ClearDeadLetter or a wiki re-edit) shows
// up immediately on the next tick rather than waiting for a stale
// count to age out.
func (m *metrics) recordDeadLetterCount(ctx context.Context, profileID wikipage.PageIdentifier, page, list string, count int64) {
	if m == nil || m.deadLetterCount == nil {
		return
	}
	m.deadLetterCount.Record(ctx, count, metric.WithAttributes(bindingAttrs(profileID, page, list)...))
}

// recordCursorReset bumps the cursor-reset counter with a reason
// label. Reasons are drawn from the CursorResetReason* constants;
// adding a new reason is a deliberate dashboard-visible event and
// requires the constant to be added above first.
func (m *metrics) recordCursorReset(ctx context.Context, profileID wikipage.PageIdentifier, page, list, reason string) {
	if m == nil || m.cursorResetTotal == nil {
		return
	}
	attrs := append(bindingAttrs(profileID, page, list), attribute.String(attrKeepBridgeReason, reason))
	m.cursorResetTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// recordTruncationStreak snapshots the current streak. Called after
// the streak counter is updated in SyncToKeep so a clean tick (which
// resets the streak to 0) is visible on the same tick.
func (m *metrics) recordTruncationStreak(ctx context.Context, profileID wikipage.PageIdentifier, page, list string, streak int64) {
	if m == nil || m.truncationStreak == nil {
		return
	}
	m.truncationStreak.Record(ctx, streak, metric.WithAttributes(bindingAttrs(profileID, page, list)...))
}

// recordPushAttempt bumps the push-attempts counter with a status
// label. n is the number of items in this status this tick — pass
// 1 per item (the loop emits per-item) or batched if the call site
// has multiple items in the same status.
func (m *metrics) recordPushAttempt(ctx context.Context, profileID wikipage.PageIdentifier, page, list, status string, n int64) {
	if m == nil || m.pushAttemptsTotal == nil || n == 0 {
		return
	}
	attrs := append(bindingAttrs(profileID, page, list), attribute.String(attrKeepBridgeStatus, status))
	m.pushAttemptsTotal.Add(ctx, n, metric.WithAttributes(attrs...))
}

// recordSilentRebaseline bumps the silent-rebaseline counter for
// the migration job. One increment per item rebaselined (not per
// binding) so the counter reflects per-item progress.
func (m *metrics) recordSilentRebaseline(ctx context.Context, profileID wikipage.PageIdentifier, page, list string) {
	if m == nil || m.silentRebaselineTotal == nil {
		return
	}
	m.silentRebaselineTotal.Add(ctx, 1, metric.WithAttributes(bindingAttrs(profileID, page, list)...))
}

// setMigrationPending sets the per-binding migration-pending gauge.
// pending=true → 1; pending=false → 0. The migration scan job sets
// 1 when it enqueues a job, the migration job clears to 0 on
// successful rebaseline. A binding stuck at 1 for hours is the
// "is the migration drained yet?" alert surface.
//
// `pending` is the gauge value, not a control flag — the caller
// already decided whether this binding has a queued migration job
// and we just emit the boolean as 0/1.
//
//revive:disable-next-line:flag-parameter
func (m *metrics) setMigrationPending(ctx context.Context, profileID wikipage.PageIdentifier, page, list string, pending bool) {
	if m == nil || m.migrationPending == nil {
		return
	}
	val := int64(0)
	if pending {
		val = 1
	}
	m.migrationPending.Record(ctx, val, metric.WithAttributes(bindingAttrs(profileID, page, list)...))
}

// SetMigrationPendingMetric is the exported entry point used by the
// eager-migration scan job (in migrations/eager) to mark a binding
// as un-migrated when it enqueues a migration job. The bridge's
// MigrateBindingFingerprints clears the gauge to 0 on successful
// completion. Lives here (not on Connector) because the scan job
// runs at startup before any Connector is wired into hot paths,
// and pure-functional access keeps the migration package free of
// connector-instance plumbing.
func SetMigrationPendingMetric(ctx context.Context, profileID wikipage.PageIdentifier, page, list string, pending bool) {
	getMetrics().setMigrationPending(ctx, profileID, page, list, pending)
}
