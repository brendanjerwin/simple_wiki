// Package index provides background indexing coordination.
package index

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
)

// IndexingProgress represents the progress of indexing operations.
type IndexingProgress struct {
	IsRunning           bool
	TotalPages          int
	CompletedPages      int
	QueueDepth          int
	ProcessingRatePerSecond float64
	EstimatedCompletion *time.Duration
	IndexProgress       map[string]SingleIndexProgress
}

// SingleIndexProgress represents progress for a single index type.
type SingleIndexProgress struct {
	Name                string
	Completed           int
	Total               int
	ProcessingRatePerSecond float64
	LastError           *string
}

// indexProgressTracker tracks progress for a single index internally.
type indexProgressTracker struct {
	name           string
	completed      int
	total          int
	startTime      time.Time
	lastError      *string
}

// BackgroundIndexingCoordinator coordinates background indexing operations
// across multiple index types using a worker pool pattern.
type BackgroundIndexingCoordinator struct {
	multiMaintainer *MultiMaintainer
	logger          lumber.Logger
	workerCount     int
	
	// Worker coordination
	ctx        context.Context
	cancel     context.CancelFunc
	workQueue  chan wikipage.PageIdentifier
	wg         sync.WaitGroup
	
	// Progress tracking
	mu              sync.RWMutex
	totalPages      int
	completedPages  int
	startTime       time.Time
	isRunning       bool
	
	// Per-index progress tracking
	indexProgress   map[string]*indexProgressTracker
}

// NewBackgroundIndexingCoordinator creates a new background indexing coordinator.
func NewBackgroundIndexingCoordinator(multiMaintainer *MultiMaintainer, logger lumber.Logger, workerCount int) *BackgroundIndexingCoordinator {
	if workerCount <= 0 {
		panic("workerCount must be greater than 0")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize progress tracking for each index
	indexProgress := make(map[string]*indexProgressTracker)
	for _, maintainer := range multiMaintainer.Maintainers {
		if indexNameProvider, ok := maintainer.(IProvideIndexName); ok {
			name := indexNameProvider.GetIndexName()
			indexProgress[name] = &indexProgressTracker{
				name: name,
			}
		}
	}
	
	return &BackgroundIndexingCoordinator{
		multiMaintainer: multiMaintainer,
		logger:          logger,
		workerCount:     workerCount,
		ctx:             ctx,
		cancel:          cancel,
		workQueue:       make(chan wikipage.PageIdentifier, workerCount*2), // Buffered queue
		indexProgress:   indexProgress,
	}
}

// NewBackgroundIndexingCoordinatorWithCPUWorkers creates a new background indexing coordinator
// using runtime.NumCPU() workers.
func NewBackgroundIndexingCoordinatorWithCPUWorkers(multiMaintainer *MultiMaintainer, logger lumber.Logger) *BackgroundIndexingCoordinator {
	return NewBackgroundIndexingCoordinator(multiMaintainer, logger, runtime.NumCPU())
}

// AddPageToIndex implements IMaintainIndex interface for compatibility.
// For synchronous operations, it directly delegates to the underlying MultiMaintainer.
func (b *BackgroundIndexingCoordinator) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	return b.multiMaintainer.AddPageToIndex(identifier)
}

// RemovePageFromIndex implements IMaintainIndex interface for compatibility.
func (b *BackgroundIndexingCoordinator) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	return b.multiMaintainer.RemovePageFromIndex(identifier)
}

// StartBackground starts background indexing of the provided page identifiers.
func (b *BackgroundIndexingCoordinator) StartBackground(pageIdentifiers []wikipage.PageIdentifier) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.isRunning {
		return nil // Already running
	}
	
	b.totalPages = len(pageIdentifiers)
	b.completedPages = 0
	b.startTime = time.Now()
	b.isRunning = true
	
	// Initialize per-index progress
	for _, tracker := range b.indexProgress {
		tracker.completed = 0
		tracker.total = len(pageIdentifiers)
		tracker.startTime = time.Now()
		tracker.lastError = nil
	}
	
	// Start worker goroutines
	for i := 0; i < b.workerCount; i++ {
		b.wg.Add(1)
		go b.worker()
	}
	
	// Start a goroutine to feed work to the queue
	go func() {
		defer close(b.workQueue)
		for _, identifier := range pageIdentifiers {
			select {
			case b.workQueue <- identifier:
			case <-b.ctx.Done():
				return
			}
		}
	}()
	
	b.logger.Info("Started background indexing with %d workers for %d pages", b.workerCount, b.totalPages)
	return nil
}

// Stop gracefully stops the background indexing process.
func (b *BackgroundIndexingCoordinator) Stop() error {
	b.cancel()
	b.wg.Wait()
	
	b.mu.Lock()
	defer b.mu.Unlock()
	b.isRunning = false
	
	b.logger.Info("Background indexing stopped")
	return nil
}

// GetProgress returns the current indexing progress.
func (b *BackgroundIndexingCoordinator) GetProgress() IndexingProgress {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	progress := IndexingProgress{
		IsRunning:      b.isRunning,
		TotalPages:     b.totalPages,
		CompletedPages: b.completedPages,
		QueueDepth:     len(b.workQueue),
	}
	
	// Calculate processing rate
	if b.isRunning && b.completedPages > 0 {
		elapsed := time.Since(b.startTime)
		progress.ProcessingRatePerSecond = float64(b.completedPages) / elapsed.Seconds()
		
		// Estimate completion time
		if progress.ProcessingRatePerSecond > 0 {
			remaining := b.totalPages - b.completedPages
			estimatedSeconds := float64(remaining) / progress.ProcessingRatePerSecond
			estimated := time.Duration(estimatedSeconds) * time.Second
			progress.EstimatedCompletion = &estimated
		}
	}
	
	// Populate per-index progress tracking
	progress.IndexProgress = make(map[string]SingleIndexProgress)
	for name, tracker := range b.indexProgress {
		singleProgress := SingleIndexProgress{
			Name:      name,
			Completed: tracker.completed,
			Total:     tracker.total,
			LastError: tracker.lastError,
		}
		
		// Calculate per-index processing rate
		if b.isRunning && tracker.completed > 0 {
			elapsed := time.Since(tracker.startTime)
			singleProgress.ProcessingRatePerSecond = float64(tracker.completed) / elapsed.Seconds()
		}
		
		progress.IndexProgress[name] = singleProgress
	}
	
	return progress
}

// WaitForCompletion blocks until all background indexing is complete.
func (b *BackgroundIndexingCoordinator) WaitForCompletion() error {
	// Wait for all workers to finish
	b.wg.Wait()
	return nil
}

// worker processes pages from the work queue.
func (b *BackgroundIndexingCoordinator) worker() {
	defer b.wg.Done()
	
	for {
		select {
		case identifier, ok := <-b.workQueue:
			if !ok {
				return // Queue closed
			}
			
			// Process the page with per-index tracking
			b.processPageWithTracking(identifier)
			
		case <-b.ctx.Done():
			return // Context cancelled
		}
	}
}

// processPageWithTracking processes a page and tracks progress for each individual index.
func (b *BackgroundIndexingCoordinator) processPageWithTracking(identifier wikipage.PageIdentifier) {
	// Process each index individually to track per-index progress
	// We don't hold the mutex during index operations to avoid blocking progress reporting
	type indexResult struct {
		name string
		err  error
	}
	
	results := make([]indexResult, 0, len(b.multiMaintainer.Maintainers))
	
	// Process all indexes without holding the mutex
	for _, maintainer := range b.multiMaintainer.Maintainers {
		var indexName string
		if indexNameProvider, ok := maintainer.(IProvideIndexName); ok {
			indexName = indexNameProvider.GetIndexName()
		} else {
			indexName = "unknown"
		}
		
		// Try to index the page in this specific index
		err := maintainer.AddPageToIndex(identifier)
		if err != nil {
			b.logger.Error("Failed to index page '%s' in %s index: %v", identifier, indexName, err)
		}
		
		results = append(results, indexResult{name: indexName, err: err})
	}
	
	// Now update all progress atomically with minimal lock time
	b.mu.Lock()
	defer b.mu.Unlock()
	
	allSuccessful := true
	for _, result := range results {
		if result.err != nil {
			allSuccessful = false
			// Record the error for this index
			if tracker, exists := b.indexProgress[result.name]; exists {
				errorMsg := result.err.Error()
				tracker.lastError = &errorMsg
			}
		} else {
			// Success - increment progress for this index
			if tracker, exists := b.indexProgress[result.name]; exists {
				tracker.completed++
				// Clear any previous error
				tracker.lastError = nil
			}
		}
	}
	
	// Only increment overall progress if at least one index succeeded
	if allSuccessful {
		b.completedPages++
	}
}