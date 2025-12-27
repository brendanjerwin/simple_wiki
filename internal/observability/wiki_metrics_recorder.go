package observability

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	// ObservabilityMetricsPage is the identifier for the wiki page storing observability metrics.
	ObservabilityMetricsPage = "observability_metrics"

	// observabilityPrefix is the frontmatter key prefix for observability data.
	observabilityPrefix = "observability"
)

// WikiMetricsRecorder provides lightweight metrics persistence to a wiki page.
// This allows tracking basic statistics even when OTEL is unavailable, and provides
// an audit trail directly within the wiki itself.
type WikiMetricsRecorder struct {
	pageWriter   wikipage.PageWriter
	pageReader   wikipage.PageReader
	logger       Logger
	jobQueue     *jobs.JobQueueCoordinator

	// In-memory counters (atomically updated)
	httpRequestsTotal  atomic.Int64
	httpErrorsTotal    atomic.Int64
	grpcRequestsTotal  atomic.Int64
	grpcErrorsTotal    atomic.Int64
	tailscaleLookups   atomic.Int64
	tailscaleSuccesses atomic.Int64
	tailscaleFailures  atomic.Int64
	tailscaleNotTailnet atomic.Int64
	headerExtractions  atomic.Int64

	// Synchronization for persistence
	mu            sync.Mutex
	lastPersisted time.Time
}

// Logger is a minimal logging interface for the wiki metrics recorder.
type Logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
}

// NewWikiMetricsRecorder creates a new WikiMetricsRecorder.
// Both pageWriter and pageReader must be provided together, or neither should be provided.
// jobQueue is required for async persistence; if nil, synchronous persistence will be used.
// If logger is nil, logging will be disabled.
func NewWikiMetricsRecorder(pageWriter wikipage.PageWriter, pageReader wikipage.PageReader, jobQueue *jobs.JobQueueCoordinator, logger Logger) (*WikiMetricsRecorder, error) {
	// Validate that both page access interfaces are provided together or not at all
	hasWriter := pageWriter != nil
	hasReader := pageReader != nil
	if hasWriter != hasReader {
		return nil, fmt.Errorf("pageWriter and pageReader must both be provided or both be nil")
	}

	return &WikiMetricsRecorder{
		pageWriter:    pageWriter,
		pageReader:    pageReader,
		jobQueue:      jobQueue,
		logger:        logger,
		lastPersisted: time.Now(),
	}, nil
}

// RecordHTTPRequest increments the HTTP request counter.
func (r *WikiMetricsRecorder) RecordHTTPRequest() {
	r.httpRequestsTotal.Add(1)
}

// RecordHTTPError increments the HTTP error counter.
func (r *WikiMetricsRecorder) RecordHTTPError() {
	r.httpErrorsTotal.Add(1)
}

// RecordGRPCRequest increments the gRPC request counter.
func (r *WikiMetricsRecorder) RecordGRPCRequest() {
	r.grpcRequestsTotal.Add(1)
}

// RecordGRPCError increments the gRPC error counter.
func (r *WikiMetricsRecorder) RecordGRPCError() {
	r.grpcErrorsTotal.Add(1)
}

// RecordTailscaleLookup increments the Tailscale lookup counter.
func (r *WikiMetricsRecorder) RecordTailscaleLookup(result IdentityLookupResult) {
	r.tailscaleLookups.Add(1)
	switch result {
	case ResultSuccess:
		r.tailscaleSuccesses.Add(1)
	case ResultFailure:
		r.tailscaleFailures.Add(1)
	case ResultNotTailnet:
		r.tailscaleNotTailnet.Add(1)
	}
}

// RecordHeaderExtraction increments the Tailscale header extraction counter.
func (r *WikiMetricsRecorder) RecordHeaderExtraction() {
	r.headerExtractions.Add(1)
}

// GetStats returns a snapshot of the current statistics.
func (r *WikiMetricsRecorder) GetStats() WikiMetricsStats {
	return WikiMetricsStats{
		HTTPRequestsTotal:      r.httpRequestsTotal.Load(),
		HTTPErrorsTotal:        r.httpErrorsTotal.Load(),
		GRPCRequestsTotal:      r.grpcRequestsTotal.Load(),
		GRPCErrorsTotal:        r.grpcErrorsTotal.Load(),
		TailscaleLookups:       r.tailscaleLookups.Load(),
		TailscaleSuccesses:     r.tailscaleSuccesses.Load(),
		TailscaleFailures:      r.tailscaleFailures.Load(),
		TailscaleNotTailnet:    r.tailscaleNotTailnet.Load(),
		HeaderExtractions:      r.headerExtractions.Load(),
	}
}

// WikiMetricsStats holds a snapshot of the metrics statistics.
type WikiMetricsStats struct {
	HTTPRequestsTotal   int64
	HTTPErrorsTotal     int64
	GRPCRequestsTotal   int64
	GRPCErrorsTotal     int64
	TailscaleLookups    int64
	TailscaleSuccesses  int64
	TailscaleFailures   int64
	TailscaleNotTailnet int64
	HeaderExtractions   int64
}

// hasPageAccess returns true if page access is configured.
func (r *WikiMetricsRecorder) hasPageAccess() bool {
	return r.pageWriter != nil && r.pageReader != nil
}

// Persist writes the current statistics to the wiki page frontmatter.
// This method uses direct frontmatter manipulation to avoid amplifying stats through APIs.
func (r *WikiMetricsRecorder) Persist() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.hasPageAccess() {
		return nil // No-op if page access is not configured
	}

	// Read existing frontmatter (silently handle missing pages)
	existingFM := r.readOrCreateFrontmatter()

	// Build observability section
	stats := r.GetStats()
	observabilityData := map[string]any{
		"http": map[string]any{
			"requests_total": stats.HTTPRequestsTotal,
			"errors_total":   stats.HTTPErrorsTotal,
		},
		"grpc": map[string]any{
			"requests_total": stats.GRPCRequestsTotal,
			"errors_total":   stats.GRPCErrorsTotal,
		},
		"tailscale": map[string]any{
			"lookups_total":     stats.TailscaleLookups,
			"successes_total":   stats.TailscaleSuccesses,
			"failures_total":    stats.TailscaleFailures,
			"not_tailnet_total": stats.TailscaleNotTailnet,
			"header_extractions_total": stats.HeaderExtractions,
		},
		"last_updated": time.Now().Format(time.RFC3339),
	}

	// Set the identifier and title
	existingFM["identifier"] = ObservabilityMetricsPage
	existingFM["title"] = "Observability Metrics"
	existingFM[observabilityPrefix] = observabilityData

	// Write frontmatter directly (not through API)
	if err := r.pageWriter.WriteFrontMatter(ObservabilityMetricsPage, existingFM); err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to persist wiki metrics: %v", err)
		}
		return err
	}

	r.lastPersisted = time.Now()

	if r.logger != nil {
		r.logger.Info("Persisted observability metrics to wiki page")
	}

	return nil
}

// readOrCreateFrontmatter reads existing frontmatter or creates a fresh map.
// Read errors are logged but do not fail the operation.
func (r *WikiMetricsRecorder) readOrCreateFrontmatter() map[string]any {
	_, existingFM, err := r.pageReader.ReadFrontMatter(ObservabilityMetricsPage)
	if err != nil {
		// Log read errors (but not "not found" which is expected)
		if r.logger != nil {
			r.logger.Warn("Could not read existing metrics page, will create fresh: %v", err)
		}
		return make(map[string]any)
	}
	if existingFM == nil {
		return make(map[string]any)
	}
	return existingFM
}

// PersistWithMarkdown writes the current statistics to the wiki page with a markdown report.
func (r *WikiMetricsRecorder) PersistWithMarkdown() error {
	if err := r.Persist(); err != nil {
		return err
	}

	if !r.hasPageAccess() {
		return nil
	}

	stats := r.GetStats()
	markdown := r.buildMarkdownReport(stats)

	return r.pageWriter.WriteMarkdown(ObservabilityMetricsPage, markdown)
}

// PersistAsync enqueues a job to persist metrics asynchronously via the job queue.
// If no job queue is configured, falls back to synchronous persistence.
func (r *WikiMetricsRecorder) PersistAsync() {
	if r.jobQueue == nil {
		// Fall back to sync if no job queue
		_ = r.Persist()
		return
	}

	r.jobQueue.EnqueueJob(&metricsPersistJob{recorder: r, withMarkdown: false})
}

// PersistWithMarkdownAsync enqueues a job to persist metrics with markdown asynchronously.
// If no job queue is configured, falls back to synchronous persistence.
func (r *WikiMetricsRecorder) PersistWithMarkdownAsync() {
	if r.jobQueue == nil {
		// Fall back to sync if no job queue
		_ = r.PersistWithMarkdown()
		return
	}

	r.jobQueue.EnqueueJob(&metricsPersistJob{recorder: r, withMarkdown: true})
}

// metricsPersistJob is a job that persists wiki metrics.
type metricsPersistJob struct {
	recorder     *WikiMetricsRecorder
	withMarkdown bool
}

// GetName returns the job name for queue routing.
func (j *metricsPersistJob) GetName() string {
	return "observability_metrics_persist"
}

// Execute performs the metrics persistence.
func (j *metricsPersistJob) Execute() error {
	if j.withMarkdown {
		return j.recorder.PersistWithMarkdown()
	}
	return j.recorder.Persist()
}

// buildMarkdownReport builds a markdown report of the current statistics.
func (r *WikiMetricsRecorder) buildMarkdownReport(stats WikiMetricsStats) string {
	report := "# Observability Metrics\n\n"
	report += "*This page is automatically updated with observability statistics.*\n\n"
	report += "## HTTP Metrics\n\n"
	report += "| Metric | Value |\n"
	report += "|--------|-------|\n"
	report += "| Total Requests | " + formatInt64(stats.HTTPRequestsTotal) + " |\n"
	report += "| Total Errors | " + formatInt64(stats.HTTPErrorsTotal) + " |\n"
	report += "\n"

	report += "## gRPC Metrics\n\n"
	report += "| Metric | Value |\n"
	report += "|--------|-------|\n"
	report += "| Total Requests | " + formatInt64(stats.GRPCRequestsTotal) + " |\n"
	report += "| Total Errors | " + formatInt64(stats.GRPCErrorsTotal) + " |\n"
	report += "\n"

	report += "## Tailscale Identity Metrics\n\n"
	report += "| Metric | Value |\n"
	report += "|--------|-------|\n"
	report += "| Total Lookups | " + formatInt64(stats.TailscaleLookups) + " |\n"
	report += "| Successful | " + formatInt64(stats.TailscaleSuccesses) + " |\n"
	report += "| Failed | " + formatInt64(stats.TailscaleFailures) + " |\n"
	report += "| Not Tailnet | " + formatInt64(stats.TailscaleNotTailnet) + " |\n"
	report += "| Header Extractions | " + formatInt64(stats.HeaderExtractions) + " |\n"

	return report
}

// formatInt64 formats an int64 as a string.
func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}
