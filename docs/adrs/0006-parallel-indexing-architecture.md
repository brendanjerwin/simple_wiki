# ADR 0006: Parallel Multi-Index Background Architecture

## Status

Accepted

## Context

### Performance Problem

The current indexing system causes significant startup delays due to:

1. **Sequential Processing**: The `InitializeIndexing()` method processes all pages synchronously in a single goroutine
2. **Blocking Startup**: The application is unavailable until all pages are indexed
3. **Heavy Per-Page Processing**: Each page requires migrations, template rendering, markdown-to-HTML conversion, and text extraction
4. **Multi-Core Underutilization**: Multi-core servers only use one CPU core during indexing

### Existing Architecture Strengths

The current system has excellent foundational architecture:

- **`IMaintainIndex` Interface**: Clean abstraction allowing multiple index implementations
- **`MultiMaintainer`**: Coordinates multiple index types (frontmatter + Bleve) seamlessly
- **Separation of Concerns**: Each index type handles its specific indexing logic independently
- **Error Handling**: Robust error handling with proper logging and recovery

The bottleneck is solely in the synchronous initialization loop in `server/site.go`:

```go
files := s.DirectoryList()
for _, file := range files {
    if err := s.IndexMaintainer.AddPageToIndex(file.Name()); err != nil {
        // Error handling...
    }
}
```

### Future Requirements

The architecture must support future integration of RAG vector stores:

- **Vector Embeddings**: Computationally expensive operations requiring GPU/CPU optimization
- **Hybrid Search**: Combining traditional search with vector similarity
- **Progressive Enhancement**: Different index types completing at different rates

## Decision

We will implement **Parallel Background Indexing** that extends the existing architecture without breaking compatibility.

### Core Principles

1. **Extend, Don't Replace**: Preserve existing `IMaintainIndex` interface and `MultiMaintainer` coordinator
2. **Background Processing**: Move indexing off the critical startup path
3. **Parallel Execution**: Utilize all available CPU cores for indexing operations
4. **Progressive Enhancement**: Application functionality becomes available as indexes complete
5. **Future-Ready**: Architecture supports RAG vector stores without redesign

### Architecture Design

#### Generic Job Queue Architecture

```go
type IndexingService struct {
    coordinator      *jobs.JobQueueCoordinator
    frontmatterIndex index.IndexOperator
    bleveIndex       index.IndexOperator
}

type JobQueueCoordinator struct {
    queues map[string]*artifex.Dispatcher  // Per-index job queues
    stats  map[string]*QueueStats         // Queue status tracking
    mu     sync.RWMutex
}
```

The architecture uses a **generic job queue system** that coordinates multiple independent job queues, one per index type. This provides better separation of concerns and reusability across different background processing needs.

#### Per-Index Job Queue Implementation

**IMPLEMENTATION**: The final architecture uses **separate job queues per index type** with a generic coordinator pattern. This provides excellent throughput isolation and extensibility:

- **Per-Index Queues**: Each index type gets its own dedicated job queue using Artifex dispatchers
- **Generic Job Pattern**: All indexing operations are modeled as jobs implementing a common `Job` interface
- **Throughput Isolation**: Fast indexes (frontmatter) are never blocked by slow indexes (Bleve/embeddings)
- **Error Isolation**: Job failures in one index don't affect other indexes
- **Extensible Design**: The pattern supports any type of background work, not just indexing

```go
type Job interface {
    Execute() error
    GetJobType() string
}

type QueueStats struct {
    QueueName     string
    JobsRemaining uint64
    HighWaterMark uint64
    IsActive      bool
}
```

#### Progress Tracking

Generic job queue status reporting with per-index visibility:

```go
type IndexingProgress struct {
    IsRunning           bool
    TotalPages          int
    CompletedPages      int
    QueueDepth          int  // Sum across all index queues
    ProcessingRatePerSecond float64
    EstimatedCompletion *time.Duration
    IndexProgress       map[string]SingleIndexProgress
}

type SingleIndexProgress struct {
    Name                string
    Completed           int
    Total               int
    ProcessingRatePerSecond float64
    LastError           *string
    QueueDepth          int
    WorkDistributionComplete bool
}
```

**Key Implementation Details**:

- Progress calculated from active job queue statistics
- Per-queue status reporting through `QueueStats`
- Thread-safe concurrent access through coordinator
- Real-time queue depth monitoring for each index type

### Integration Points

#### Startup Modification

```go
func (s *Site) InitializeIndexing() error {
    // Create indexes (unchanged)
    frontmatterIndex := frontmatter.NewIndex(s)
    bleveIndex, err := bleve.NewIndex(s, frontmatterIndex)
    if err != nil {
        return err
    }
    multiMaintainer := index.NewMultiMaintainer(frontmatterIndex, bleveIndex)
    
    s.FrontmatterIndexQueryer = frontmatterIndex
    s.BleveIndexQueryer = bleveIndex
    s.IndexMaintainer = multiMaintainer
    
    // Create job queue coordinator and indexing service
    s.JobQueueCoordinator = jobs.NewJobQueueCoordinator()
    s.IndexingService = NewIndexingService(s.JobQueueCoordinator, frontmatterIndex, bleveIndex)
    s.IndexingService.InitializeQueues()
    
    // Get all files that need to be indexed
    files := s.DirectoryList()
    if len(files) == 0 {
        s.Logger.Info("No pages found to index.")
        return nil
    }
    
    // Convert files to page identifiers and start background indexing
    pageIdentifiers := make([]string, len(files))
    for i, file := range files {
        pageIdentifiers[i] = file.Name()
    }
    
    s.IndexingService.BulkEnqueuePages(pageIdentifiers, index.Add)
    s.Logger.Info("Background indexing started for %d pages. Application is ready.", len(files))
    return nil
}
```

#### System Monitoring

**IMPLEMENTATION**: Complete gRPC API and UI implementation delivered:

- **gRPC SystemInfoService**: Extended with `GetIndexingStatus()` endpoint using job queue status
- **Real-time Progress API**: Returns detailed per-queue progress through `IndexingService.GetProgress()`
- **SystemInfo UI Component**: Bottom-right overlay with version and indexing status
- **Auto-refresh Behavior**: 2-second intervals during active indexing, 10-second when idle
- **Responsive UI States**: Loading, idle, active, complete, and error states
- **Per-index Detail View**: Queue-based progress breakdown with job statistics

### Future RAG Integration

Vector stores integrate seamlessly using existing patterns:

```go
// Vector index implements existing interface
type VectorIndexWorker struct {
    store      VectorStore
    embedder   EmbeddingModel
}

func (v *VectorIndexWorker) AddPageToIndex(identifier wikipage.PageIdentifier) error {
    // Vector-specific indexing logic
}

// Integration requires no architecture changes
multiMaintainer := index.NewMultiMaintainer(
    frontmatterIndex, 
    bleveIndex, 
    vectorIndex,  // Seamlessly added
)
```

### Error Handling Strategy

- **Preserve Existing Patterns**: Maintain current error handling and logging
- **Index Independence**: Failures in one index type don't affect others
- **User Feedback**: Clear indication of indexing status and errors
- **Graceful Degradation**: Application functions with partially completed indexes

### Performance Characteristics

#### Startup Performance

- **Immediate Availability**: Application serves requests immediately
- **Background Processing**: Indexing utilizes multiple CPU cores without blocking
- **Progressive Enhancement**: Features become available as indexes complete

#### Resource Management

- **Configurable Workers**: Limit concurrent indexing operations
- **Memory Efficiency**: Process pages in batches to manage memory usage
- **CPU Utilization**: Distribute work across available cores

#### Future Scalability

- **Vector Processing**: Architecture supports GPU acceleration for embeddings
- **Large Wikis**: Efficient handling of thousands of pages
- **Hybrid Operations**: Multiple index types with different performance characteristics

## Consequences

### Positive

- **Immediate Startup**: Users can access the wiki instantly regardless of content size
- **Multi-Core Utilization**: Full utilization of available CPU resources
- **Better User Experience**: Clear progress indication instead of mysterious delays
- **Future-Ready Architecture**: Seamless path for RAG vector store integration
- **Preserved Compatibility**: All existing indexing logic and interfaces unchanged
- **Progressive Enhancement**: Application functionality scales with indexing completion

### Negative

- **Increased Complexity**: Additional coordination logic and state management
- **New Failure Modes**: Background processing introduces new error scenarios
- **Resource Usage**: Multiple worker goroutines consume additional memory
- **Testing Complexity**: Asynchronous operations require more sophisticated testing

### Neutral

- **Configuration Options**: Additional CLI flags for indexing behavior
- **Monitoring Requirements**: Need for indexing progress visibility
- **Documentation Updates**: New architectural patterns require explanation

## Implementation Strategy

### Phase 1: Core Infrastructure ✅ COMPLETED

- Implement `JobQueueCoordinator` with generic job queue system
- Create `IndexingService` with per-index job queues
- Modify `InitializeIndexing()` for background job processing

### Phase 2: System Integration ✅ COMPLETED

- Extend gRPC system info service with job queue status
- Create UI components for progress display using queue statistics
- Implement real-time status monitoring

### Phase 3: Testing and Optimization ✅ COMPLETED

- Comprehensive testing of parallel job processing scenarios
- Performance optimization with Artifex dispatcher integration
- Documentation and architectural decision recording

### Phase 4: Future Preparation

- Generic job interface supports any background processing workload
- Performance monitoring infrastructure in place
- Extensible foundation for RAG vector stores and other background tasks

## Risks and Mitigations

### Risk: Job Queue Complexity

**Mitigation**: Use proven Artifex dispatcher library, comprehensive testing of job processing scenarios

### Risk: Resource Usage

**Mitigation**: Configurable queue capacities, job queue monitoring, controlled worker counts per queue

### Risk: User Confusion

**Mitigation**: Clear progress indicators through job queue statistics, real-time status reporting

### Risk: Future Vector Store Integration

**Mitigation**: Generic job interface supports any processing type, monitoring infrastructure already in place

## Alternatives Considered

1. **Synchronous Optimization**: Improve existing sequential processing
   - Rejected: Doesn't utilize multi-core systems effectively

2. **Custom Worker Pool Implementation**: Build indexing-specific worker pools
   - Rejected: Generic job queue system provides better reusability and proven reliability

3. **Index-Specific Parallelization**: Parallelize within each index type
   - Rejected: Doesn't address the coordination bottleneck, more complex integration

4. **Lazy Indexing**: Index pages only when accessed
   - Rejected: Poor performance for search functionality, inconsistent user experience

## Outcome

This architecture transforms indexing from a startup blocker into a background enhancement using a **generic job queue system** that supports any type of background processing. It provides immediate user value through faster startup times and establishes a flexible, extensible foundation for future RAG vector stores and other background tasks.

The design maintains backward compatibility with existing indexing interfaces, provides comprehensive job queue monitoring, and delivers dramatic user experience improvements through parallel processing and real-time progress tracking.
