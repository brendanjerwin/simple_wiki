// Package index provides background indexing coordination.
package index

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
)

const (
	// Buffer sizing constants
	defaultBufferPercentage = 0.2 // 20% buffer for dynamic workloads
	minBufferSize = 10 // Minimum buffer size
	progressLogInterval = 100 // Log progress every N pages
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
	QueueDepth          int
	WorkDistributionComplete bool
}

// indexProgressTracker tracks progress for a single index internally.
type indexProgressTracker struct {
	name           string
	completed      int
	total          int
	startTime      time.Time
	lastError      *string
	workDistributionComplete bool
}

// indexWorkerPool manages workers for a single index type.
type indexWorkerPool struct {
	indexName     string
	maintainer    IMaintainIndex
	workQueue     chan wikipage.PageIdentifier
	workerCount   int
	wg            sync.WaitGroup
	logger        lumber.Logger
}

// BackgroundIndexingCoordinator coordinates background indexing operations
// across multiple index types using separate worker pools per index type.
type BackgroundIndexingCoordinator struct {
	multiMaintainer *MultiMaintainer
	logger          lumber.Logger
	
	// Worker coordination
	ctx        context.Context
	cancel     context.CancelFunc
	
	// Per-index worker pools
	indexPools     map[string]*indexWorkerPool
	
	// Progress tracking
	mu              sync.RWMutex
	totalPages      int
	startTime       time.Time
	isRunning       bool
	
	// Per-index progress tracking
	indexProgress   map[string]*indexProgressTracker
}

// IndexWorkerConfig specifies the number of workers for a specific index type.
type IndexWorkerConfig struct {
	IndexName   string
	WorkerCount int
}

// NewBackgroundIndexingCoordinator creates a new background indexing coordinator with per-index worker configuration.
func NewBackgroundIndexingCoordinator(multiMaintainer *MultiMaintainer, logger lumber.Logger, workerConfigs []IndexWorkerConfig) *BackgroundIndexingCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create worker config map for easy lookup
	workerConfigMap := make(map[string]int)
	for _, config := range workerConfigs {
		if config.WorkerCount <= 0 {
			panic(fmt.Sprintf("worker count for index '%s' must be greater than 0", config.IndexName))
		}
		workerConfigMap[config.IndexName] = config.WorkerCount
	}
	
	// Initialize per-index worker pools and progress tracking
	indexPools := make(map[string]*indexWorkerPool)
	indexProgress := make(map[string]*indexProgressTracker)
	
	for _, maintainer := range multiMaintainer.Maintainers {
		var indexName string
		if indexNameProvider, ok := maintainer.(IProvideIndexName); ok {
			indexName = indexNameProvider.GetIndexName()
		} else {
			indexName = "unknown"
		}
		
		// Get worker count for this index, default to 1 if not specified
		workerCount := workerConfigMap[indexName]
		if workerCount == 0 {
			workerCount = 1
			logger.Warn("No worker count specified for index '%s', defaulting to 1", indexName)
		}
		
		// Create worker pool for this index
		indexPools[indexName] = &indexWorkerPool{
			indexName:   indexName,
			maintainer:  maintainer,
			workQueue:   make(chan wikipage.PageIdentifier, workerCount*2), // Buffered queue
			workerCount: workerCount,
			logger:      logger,
		}
		
		// Initialize progress tracking for this index
		indexProgress[indexName] = &indexProgressTracker{
			name: indexName,
		}
	}
	
	return &BackgroundIndexingCoordinator{
		multiMaintainer: multiMaintainer,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		indexPools:      indexPools,
		indexProgress:   indexProgress,
	}
}

// NewBackgroundIndexingCoordinatorWithCPUWorkers creates a new background indexing coordinator
// using runtime.NumCPU() workers per index type.
func NewBackgroundIndexingCoordinatorWithCPUWorkers(multiMaintainer *MultiMaintainer, logger lumber.Logger) *BackgroundIndexingCoordinator {
	cpuCount := runtime.NumCPU()
	
	// Create default worker configs for all index types
	var workerConfigs []IndexWorkerConfig
	for _, maintainer := range multiMaintainer.Maintainers {
		if indexNameProvider, ok := maintainer.(IProvideIndexName); ok {
			indexName := indexNameProvider.GetIndexName()
			workerConfigs = append(workerConfigs, IndexWorkerConfig{
				IndexName:   indexName,
				WorkerCount: cpuCount,
			})
		}
	}
	
	return NewBackgroundIndexingCoordinator(multiMaintainer, logger, workerConfigs)
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
	
	b.initializeIndexing(pageIdentifiers)
	b.resizeQueues(pageIdentifiers)
	b.initializeProgress(pageIdentifiers)
	b.startWorkersAndDistributors(pageIdentifiers)
	
	totalWorkers := b.calculateTotalWorkers()
	b.logger.Info("Started background indexing with %d total workers across %d index types for %d pages", 
		totalWorkers, len(b.indexPools), b.totalPages)
	return nil
}

// initializeIndexing sets up the basic indexing state.
func (b *BackgroundIndexingCoordinator) initializeIndexing(pageIdentifiers []wikipage.PageIdentifier) {
	b.totalPages = len(pageIdentifiers)
	b.startTime = time.Now()
	b.isRunning = true
}

// resizeQueues resizes all index queues to accommodate the workload.
func (b *BackgroundIndexingCoordinator) resizeQueues(pageIdentifiers []wikipage.PageIdentifier) {
	baseSize := len(pageIdentifiers)
	bufferSize := int(float64(baseSize) * defaultBufferPercentage)
	if bufferSize < minBufferSize {
		bufferSize = minBufferSize
	}
	queueSize := baseSize + bufferSize
	
	for indexName, pool := range b.indexPools {
		oldQueue := pool.workQueue
		pool.workQueue = make(chan wikipage.PageIdentifier, queueSize)
		b.logger.Debug("Resized %s queue from buffer=%d to buffer=%d for %d pages", 
			indexName, cap(oldQueue), cap(pool.workQueue), len(pageIdentifiers))
	}
}

// initializeProgress resets progress tracking for all indexes.
func (b *BackgroundIndexingCoordinator) initializeProgress(pageIdentifiers []wikipage.PageIdentifier) {
	for _, tracker := range b.indexProgress {
		tracker.completed = 0
		tracker.total = len(pageIdentifiers)
		tracker.startTime = time.Now()
		tracker.lastError = nil
		tracker.workDistributionComplete = false
	}
}

// startWorkersAndDistributors starts worker goroutines and work distributors for all indexes.
func (b *BackgroundIndexingCoordinator) startWorkersAndDistributors(pageIdentifiers []wikipage.PageIdentifier) {
	for indexName, pool := range b.indexPools {
		b.logger.Info("Starting %d workers for %s index", pool.workerCount, indexName)
		
		b.startIndexWorkers(pool, indexName)
		b.startWorkDistributor(pool, indexName, pageIdentifiers)
	}
}

// startIndexWorkers starts worker goroutines for a specific index.
func (b *BackgroundIndexingCoordinator) startIndexWorkers(pool *indexWorkerPool, indexName string) {
	for i := 0; i < pool.workerCount; i++ {
		pool.wg.Add(1)
		go b.indexWorker(pool, indexName)
	}
}

// startWorkDistributor starts a work distributor goroutine for a specific index.
func (b *BackgroundIndexingCoordinator) startWorkDistributor(pool *indexWorkerPool, indexName string, pageIdentifiers []wikipage.PageIdentifier) {
	go func(indexPool *indexWorkerPool, idxName string) {
		defer b.completeWorkDistribution(indexPool, idxName, len(pageIdentifiers))
		
		b.logger.Info("Work distributor for %s starting - will feed %d pages to queue (capacity: %d)", 
			idxName, len(pageIdentifiers), cap(indexPool.workQueue))
		
		b.distributeWork(indexPool, idxName, pageIdentifiers)
	}(pool, indexName)
}

// completeWorkDistribution handles cleanup when work distribution is complete.
func (b *BackgroundIndexingCoordinator) completeWorkDistribution(indexPool *indexWorkerPool, idxName string, pageCount int) {
	close(indexPool.workQueue)
	
	b.mu.Lock()
	if tracker, exists := b.indexProgress[idxName]; exists {
		tracker.workDistributionComplete = true
	}
	b.mu.Unlock()
	
	b.logger.Info("Work distributor for %s completed - fed %d pages, queue depth now %d", 
		idxName, pageCount, len(indexPool.workQueue))
}

// distributeWork feeds work items to the index queue.
func (b *BackgroundIndexingCoordinator) distributeWork(indexPool *indexWorkerPool, idxName string, pageIdentifiers []wikipage.PageIdentifier) {
	for i, identifier := range pageIdentifiers {
		if b.shouldLogProgress(i, len(pageIdentifiers)) {
			b.logger.Debug("Feeding page %d/%d (%s) to %s queue", i+1, len(pageIdentifiers), identifier, idxName)
		}
		
		select {
		case indexPool.workQueue <- identifier:
			if b.shouldLogProgress(i, len(pageIdentifiers)) {
				b.logger.Debug("Successfully queued page %s to %s (queue depth now: %d)", identifier, idxName, len(indexPool.workQueue))
			}
		case <-b.ctx.Done():
			b.logger.Info("Work distributor for %s cancelled at page %d/%d", idxName, i+1, len(pageIdentifiers))
			return
		}
	}
}

// shouldLogProgress determines if progress should be logged for the current item.
func (*BackgroundIndexingCoordinator) shouldLogProgress(currentIndex, totalCount int) bool {
	return (currentIndex+1)%progressLogInterval == 0 || currentIndex == 0 || currentIndex == totalCount-1
}

// calculateTotalWorkers calculates the total number of workers across all indexes.
func (b *BackgroundIndexingCoordinator) calculateTotalWorkers() int {
	totalWorkers := 0
	for _, pool := range b.indexPools {
		totalWorkers += pool.workerCount
	}
	return totalWorkers
}

// Stop gracefully stops the background indexing process.
func (b *BackgroundIndexingCoordinator) Stop() error {
	b.cancel()
	
	// Wait for all index pools to complete
	for indexName, pool := range b.indexPools {
		b.logger.Info("Waiting for %s index workers to stop", indexName)
		pool.wg.Wait()
	}
	
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
	
	totalQueueDepth := b.calculateTotalQueueDepth()
	overallCompleted := b.calculateOverallCompleted()
	
	// Log queue state for debugging
	if b.isRunning {
		queueStates := make([]string, 0, len(b.indexPools))
		for indexName, pool := range b.indexPools {
			queueDepth := len(pool.workQueue)
			queueCapacity := cap(pool.workQueue)
			completed := 0
			workDistComplete := false
			if tracker, exists := b.indexProgress[indexName]; exists {
				completed = tracker.completed
				workDistComplete = tracker.workDistributionComplete
			}
			queueStates = append(queueStates, fmt.Sprintf("%s: completed=%d, queue=%d/%d, dist_complete=%t", 
				indexName, completed, queueDepth, queueCapacity, workDistComplete))
		}
		b.logger.Debug("Queue states - Total: completed=%d/%d, total_queue=%d | Per-index: %v", 
			overallCompleted, b.totalPages, totalQueueDepth, queueStates)
	}
	
	// Check if indexing should automatically transition to completed
	// Check completion conditions and determine if we should transition to completed state
	shouldComplete := b.isRunning && b.totalPages > 0 && totalQueueDepth == 0 && overallCompleted >= b.totalPages
	isRunning := b.isRunning && !shouldComplete
	
	// If we should complete, update the state immediately to avoid race conditions
	if shouldComplete {
		b.isRunning = false
		b.logger.Info("Background indexing completed automatically - all queues empty and %d/%d pages processed", overallCompleted, b.totalPages)
	}
	
	progress := IndexingProgress{
		IsRunning:      isRunning,
		TotalPages:     b.totalPages,
		CompletedPages: overallCompleted,
		QueueDepth:     totalQueueDepth,
	}
	
	b.calculateProcessingRateAndEstimation(&progress, overallCompleted)
	b.populateIndexProgress(&progress)
	
	return progress
}

// calculateTotalQueueDepth calculates the total queue depth across all index pools.
func (b *BackgroundIndexingCoordinator) calculateTotalQueueDepth() int {
	totalQueueDepth := 0
	for _, pool := range b.indexPools {
		totalQueueDepth += len(pool.workQueue)
	}
	return totalQueueDepth
}

// calculateOverallCompleted calculates the minimum completed pages across all indexes.
func (b *BackgroundIndexingCoordinator) calculateOverallCompleted() int {
	overallCompleted := 0
	if len(b.indexProgress) > 0 {
		// Initialize to the first index's progress
		first := true
		for _, tracker := range b.indexProgress {
			if first {
				overallCompleted = tracker.completed
				first = false
			} else if tracker.completed < overallCompleted {
				overallCompleted = tracker.completed
			}
		}
	}
	return overallCompleted
}

// calculateProcessingRateAndEstimation calculates processing rates and estimated completion time.
func (b *BackgroundIndexingCoordinator) calculateProcessingRateAndEstimation(progress *IndexingProgress, overallCompleted int) {
	if overallCompleted <= 0 {
		return
	}
	
	elapsed := time.Since(b.startTime)
	progress.ProcessingRatePerSecond = float64(overallCompleted) / elapsed.Seconds()
	
	// Estimate completion time based on slowest index
	slowestRate := progress.ProcessingRatePerSecond
	for _, tracker := range b.indexProgress {
		if tracker.completed > 0 {
			trackerElapsed := time.Since(tracker.startTime)
			trackerRate := float64(tracker.completed) / trackerElapsed.Seconds()
			if trackerRate < slowestRate {
				slowestRate = trackerRate
			}
		}
	}
	
	if slowestRate > 0 {
		remaining := b.totalPages - overallCompleted
		estimatedSeconds := float64(remaining) / slowestRate
		estimated := time.Duration(estimatedSeconds) * time.Second
		progress.EstimatedCompletion = &estimated
	}
}

// populateIndexProgress populates per-index progress tracking.
func (b *BackgroundIndexingCoordinator) populateIndexProgress(progress *IndexingProgress) {
	progress.IndexProgress = make(map[string]SingleIndexProgress)
	for name, tracker := range b.indexProgress {
		singleProgress := SingleIndexProgress{
			Name:      name,
			Completed: tracker.completed,
			Total:     tracker.total,
			LastError: tracker.lastError,
			WorkDistributionComplete: tracker.workDistributionComplete,
		}
		
		// Calculate per-index processing rate
		if tracker.completed > 0 {
			elapsed := time.Since(tracker.startTime)
			singleProgress.ProcessingRatePerSecond = float64(tracker.completed) / elapsed.Seconds()
		}
		
		// Set per-index queue depth
		if pool, exists := b.indexPools[name]; exists {
			singleProgress.QueueDepth = len(pool.workQueue)
		}
		
		progress.IndexProgress[name] = singleProgress
	}
}

// WaitForCompletion blocks until all background indexing is complete.
func (b *BackgroundIndexingCoordinator) WaitForCompletion() error {
	// Wait for all index pools to finish
	for _, pool := range b.indexPools {
		pool.wg.Wait()
	}
	return nil
}

// indexWorker processes pages from a specific index's work queue.
func (b *BackgroundIndexingCoordinator) indexWorker(pool *indexWorkerPool, indexName string) {
	defer pool.wg.Done()
	
	for {
		select {
		case identifier, ok := <-pool.workQueue:
			if !ok {
				return // Queue closed
			}
			
			// Process the page for this specific index
			b.processPageForIndex(pool.maintainer, indexName, identifier)
			
		case <-b.ctx.Done():
			return // Context cancelled
		}
	}
}

// processPageForIndex processes a page for a specific index and tracks progress.
func (b *BackgroundIndexingCoordinator) processPageForIndex(maintainer IMaintainIndex, indexName string, identifier wikipage.PageIdentifier) {
	b.logger.Debug("Starting to process page '%s' in %s index", identifier, indexName)
	
	// Process the page without holding the mutex
	err := maintainer.AddPageToIndex(identifier)
	if err != nil {
		b.logger.Error("Failed to process page '%s' in %s index: %v", identifier, indexName, err)
	} else {
		b.logger.Debug("Successfully processed page '%s' in %s index", identifier, indexName)
	}
	
	// Update progress atomically with minimal lock time
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if tracker, exists := b.indexProgress[indexName]; exists {
		if err != nil {
			// Record the error for this index
			errorMsg := err.Error()
			tracker.lastError = &errorMsg
		} else {
			// Success - increment progress for this index
			tracker.completed++
			// Don't clear previous errors, just record success
			// The lastError field will persist to show the most recent error
		}
	}
}