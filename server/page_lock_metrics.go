package server

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Page-lock observability. Two instruments:
//
//   - wiki_page_lock_wait_seconds — histogram of acquire latency. A 16-deep
//     queue (the production incident this refactor fixes) shows up as a
//     bimodal distribution with a fat tail.
//   - wiki_page_lock_holders — UpDownCounter (gauge) of locks currently
//     held. Surfaces contention depth that wait-histograms might miss when
//     individual holds are short.
//
// Both are scoped to the "simple_wiki/pagelocks" meter so dashboards can
// pivot on the meter name without confusing them with HTTP or gRPC metrics.

var (
	pageLockMetricsInitOnce sync.Once
	pageLockWaitSeconds     metric.Float64Histogram
	pageLockHolders         metric.Int64UpDownCounter
)

// initPageLockMetrics constructs the instruments on first call.
// Subsequent calls are no-ops. If construction fails (only possible on
// malformed instrument names — our names are constants, so practically
// never), the instruments stay nil and the lockPage path quietly skips
// recording rather than crashing. Observability is non-load-bearing.
func initPageLockMetrics() {
	pageLockMetricsInitOnce.Do(func() {
		meter := otel.Meter("simple_wiki/pagelocks")

		hist, err := meter.Float64Histogram(
			"wiki_page_lock_wait_seconds",
			metric.WithDescription("How long a caller waited to acquire a page lock"),
			metric.WithUnit("s"),
		)
		if err != nil {
			return
		}
		pageLockWaitSeconds = hist

		gauge, err := meter.Int64UpDownCounter(
			"wiki_page_lock_holders",
			metric.WithDescription("Number of page locks currently held"),
			metric.WithUnit("{lock}"),
		)
		if err != nil {
			return
		}
		pageLockHolders = gauge
	})
}

// recordPageLockAcquired updates the metrics on a successful acquire.
// waitDuration is the time spent blocked on the mutex.
func recordPageLockAcquired(waitDuration time.Duration) {
	initPageLockMetrics()
	ctx := context.Background()
	if pageLockWaitSeconds != nil {
		pageLockWaitSeconds.Record(ctx, waitDuration.Seconds())
	}
	if pageLockHolders != nil {
		pageLockHolders.Add(ctx, 1)
	}
}

// recordPageLockReleased decrements the holders gauge on release.
func recordPageLockReleased() {
	if pageLockHolders != nil {
		pageLockHolders.Add(context.Background(), -1)
	}
}
