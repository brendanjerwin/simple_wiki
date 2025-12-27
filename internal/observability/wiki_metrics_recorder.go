package observability

import (
	"errors"
	"strings"
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

	// requiredDependencyCount is the number of persistence dependencies required.
	requiredDependencyCount = 3
)

// WikiMetricsRecorder provides lightweight metrics persistence to a wiki page.
// This allows tracking basic statistics even when OTEL is unavailable, and provides
// an audit trail directly within the wiki itself.
type WikiMetricsRecorder struct {
	pageWriter wikipage.PageWriter
	pageReader PageReader
	logger     Logger
	jobQueue   *jobs.JobQueueCoordinator

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

// PageReader is a narrow interface for reading page content.
// This allows WikiMetricsRecorder to accept any type that can read pages,
// without requiring the full PageReaderMutator interface.
type PageReader interface {
	ReadFrontMatter(requestedIdentifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error)
	ReadMarkdown(requestedIdentifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error)
}

// NewWikiMetricsRecorder creates a new WikiMetricsRecorder.
// All dependencies must be provided together: pageWriter, pageReader, and jobQueue.
// If any are nil, all must be nil (metrics-only mode without persistence).
// If logger is nil, logging will be disabled.
func NewWikiMetricsRecorder(pageWriter wikipage.PageWriter, pageReader PageReader, jobQueue *jobs.JobQueueCoordinator, logger Logger) (*WikiMetricsRecorder, error) {
	// Validate that all persistence dependencies are provided together or not at all
	hasWriter := pageWriter != nil
	hasReader := pageReader != nil
	hasQueue := jobQueue != nil

	// Count how many are provided
	providedCount := 0
	if hasWriter {
		providedCount++
	}
	if hasReader {
		providedCount++
	}
	if hasQueue {
		providedCount++
	}

	// Either all three must be provided, or none
	if providedCount != 0 && providedCount != requiredDependencyCount {
		return nil, errors.New("pageWriter, pageReader, and jobQueue must all be provided together or all be nil")
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
	default:
		// Unknown result type, just count the lookup
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
// If the page doesn't exist, it creates it with a markdown template that displays frontmatter data.
// If the page exists, it only updates the frontmatter without touching the markdown.
func (r *WikiMetricsRecorder) Persist() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.hasPageAccess() {
		return nil // No-op if page access is not configured
	}

	// Read existing frontmatter (may be nil for new page)
	_, existingFM, _ := r.pageReader.ReadFrontMatter(ObservabilityMetricsPage)
	if existingFM == nil {
		existingFM = make(map[string]any)
	}

	// Check if markdown needs to be written (empty or whitespace-only)
	_, existingMD, _ := r.pageReader.ReadMarkdown(ObservabilityMetricsPage)
	needsMarkdownTemplate := len(strings.TrimSpace(string(existingMD))) == 0

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
			"lookups_total":            stats.TailscaleLookups,
			"successes_total":          stats.TailscaleSuccesses,
			"failures_total":           stats.TailscaleFailures,
			"not_tailnet_total":        stats.TailscaleNotTailnet,
			"header_extractions_total": stats.HeaderExtractions,
		},
		"last_updated": time.Now().Format(time.RFC3339),
	}

	// Set the identifier and title
	existingFM["identifier"] = ObservabilityMetricsPage
	existingFM["title"] = "Observability Metrics"
	existingFM[observabilityPrefix] = observabilityData

	// If markdown is empty/whitespace, write the template
	if needsMarkdownTemplate {
		if err := r.pageWriter.WriteMarkdown(ObservabilityMetricsPage, r.buildMarkdownTemplate()); err != nil {
			if r.logger != nil {
				r.logger.Error("Failed to write metrics page template: %v", err)
			}
			return err
		}
		if r.logger != nil {
			r.logger.Info("Created observability metrics page with template")
		}
	}

	// Write frontmatter (creates page if markdown didn't, or updates existing)
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

// PersistAsync enqueues a job to persist metrics asynchronously via the job queue.
// Requires jobQueue to be configured in the constructor.
func (r *WikiMetricsRecorder) PersistAsync() {
	if r.jobQueue == nil {
		return // No persistence configured
	}

	r.jobQueue.EnqueueJob(&metricsPersistJob{recorder: r})
}

// metricsPersistJob is a job that persists wiki metrics.
type metricsPersistJob struct {
	recorder *WikiMetricsRecorder
}

// GetName returns the job name for queue routing.
func (*metricsPersistJob) GetName() string {
	return "observability_metrics_persist"
}

// Execute performs the metrics persistence.
func (j *metricsPersistJob) Execute() error {
	return j.recorder.Persist()
}

// buildMarkdownTemplate returns a markdown template that displays frontmatter data.
// This template is only written when creating a new page; existing pages keep their markdown.
// Frontmatter values are accessed via .Map in the TemplateContext.
func (*WikiMetricsRecorder) buildMarkdownTemplate() string {
	return `# Observability Metrics

*This page displays server observability statistics from frontmatter data.*

**Last Updated:** {{ .Map.observability.last_updated }}

## HTTP Metrics

| Metric | Value |
|--------|-------|
| Total Requests | {{ .Map.observability.http.requests_total }} |
| Total Errors | {{ .Map.observability.http.errors_total }} |

## gRPC Metrics

| Metric | Value |
|--------|-------|
| Total Requests | {{ .Map.observability.grpc.requests_total }} |
| Total Errors | {{ .Map.observability.grpc.errors_total }} |

## Tailscale Identity Metrics

| Metric | Value |
|--------|-------|
| Total Lookups | {{ .Map.observability.tailscale.lookups_total }} |
| Successful | {{ .Map.observability.tailscale.successes_total }} |
| Failed | {{ .Map.observability.tailscale.failures_total }} |
| Not Tailnet | {{ .Map.observability.tailscale.not_tailnet_total }} |
| Header Extractions | {{ .Map.observability.tailscale.header_extractions_total }} |
`
}
