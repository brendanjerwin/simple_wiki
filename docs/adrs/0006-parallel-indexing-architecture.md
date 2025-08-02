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

#### Background Indexing Coordinator

```go
type BackgroundIndexingCoordinator struct {
    multiMaintainer  *index.MultiMaintainer  // Existing coordinator unchanged
    workerPool       *IndexingWorkerPool     // New parallel processing
    progressTracker  *ProgressTracker        // New progress monitoring
    logger           lumber.Logger
}
```

The coordinator wraps the existing `MultiMaintainer` and implements the same `IMaintainIndex` interface for backward compatibility.

#### Per-Index Worker Pool Implementation

**IMPLEMENTATION NOTE**: The final architecture uses **separate worker pools per index type** rather than a single shared pool. This critical enhancement addresses throughput isolation between fast indexes (frontmatter) and slow indexes (AI embeddings).

- **Per-Index Queues**: Each index type gets its own worker pool and queue
- **Configurable Workers Per Index**: Independent worker counts per index type (e.g., frontmatter: 4 workers, embeddings: 1 worker)
- **Throughput Isolation**: Fast indexes are never blocked by slow ones
- **Error Isolation**: Individual page failures don't stop other processing
- **Graceful Shutdown**: All worker pools can be stopped cleanly during application shutdown

```go
type indexWorkerPool struct {
    indexName     string
    maintainer    IMaintainIndex
    workQueue     chan wikipage.PageIdentifier
    workerCount   int
    wg            sync.WaitGroup
    logger        lumber.Logger
}

type IndexWorkerConfig struct {
    IndexName   string
    WorkerCount int
}
```

#### Progress Tracking

Multi-index progress monitoring across all index types with per-index detail:

```go
type IndexingProgress struct {
    IsRunning           bool
    TotalPages          int
    CompletedPages      int  // Minimum across all indexes
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
}
```

**Key Implementation Details**:

- Overall progress calculated as minimum completion across all indexes
- Individual error tracking per index type
- Thread-safe concurrent access with RWMutex
- Completion estimation based on slowest index

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
    
    // Create background coordinator with per-index worker configuration
    workerConfigs := []index.IndexWorkerConfig{
        {IndexName: "frontmatter", WorkerCount: runtime.NumCPU()},
        {IndexName: "bleve", WorkerCount: runtime.NumCPU()},
    }
    backgroundCoordinator := index.NewBackgroundIndexingCoordinator(
        index.NewMultiMaintainer(frontmatterIndex, bleveIndex),
        s.Logger,
        workerConfigs,
    )
    
    s.IndexMaintainer = backgroundCoordinator
    
    // Start background indexing instead of synchronous loop
    pageIdentifiers := s.getPageIdentifiers()
    return backgroundCoordinator.StartBackground(pageIdentifiers)
}
```

#### System Monitoring

**IMPLEMENTATION**: Complete gRPC API and UI implementation delivered:

- **gRPC SystemInfoService**: Extended with `GetIndexingStatus()` endpoint
- **Real-time Progress API**: Returns detailed per-index progress, rates, and errors
- **SystemInfo UI Component**: Bottom-right overlay with version and indexing status
- **Auto-refresh Behavior**: 2-second intervals during active indexing, 10-second when idle
- **Responsive UI States**: Loading, idle, active, complete, and error states
- **Per-index Detail View**: Expandable progress breakdown with error messages

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

### Phase 1: Core Infrastructure

- Implement `BackgroundIndexingCoordinator` with worker pool
- Add progress tracking across existing index types
- Modify `InitializeIndexing()` for background processing

### Phase 2: System Integration

- Extend gRPC system info service with indexing status
- Create UI components for progress display
- Add CLI configuration options

### Phase 3: Testing and Optimization

- Comprehensive testing of parallel indexing scenarios
- Performance optimization and tuning
- Documentation and deployment guides

### Phase 4: Future Preparation

- Interface extensions for vector store integration
- Performance monitoring for computationally expensive operations
- Hybrid search capability foundation

## Risks and Mitigations

### Risk: Background Indexing Complexity

**Mitigation**: Extend existing patterns rather than replacing them, comprehensive testing

### Risk: Resource Usage

**Mitigation**: Configurable worker limits, memory usage monitoring, backpressure mechanisms

### Risk: User Confusion

**Mitigation**: Clear progress indicators, status reporting, and documentation

### Risk: Future Vector Store Integration

**Mitigation**: Design interfaces with vector operations in mind, performance monitoring infrastructure

## Alternatives Considered

1. **Synchronous Optimization**: Improve existing sequential processing
   - Rejected: Doesn't utilize multi-core systems effectively

2. **Complete Architecture Replacement**: Design new indexing system from scratch
   - Rejected: Existing architecture is solid, replacement introduces unnecessary risk

3. **Index-Specific Parallelization**: Parallelize within each index type
   - Rejected: Doesn't address the coordination bottleneck, more complex integration

4. **Lazy Indexing**: Index pages only when accessed
   - Rejected: Poor performance for search functionality, inconsistent user experience

## Outcome

This architecture transforms indexing from a startup blocker into a background enhancement while preserving the excellent existing multi-index foundation. It provides immediate user value through faster startup times and establishes a clear, efficient path for future RAG vector store integration without requiring architectural rework.

The design maintains backward compatibility, preserves existing testing, and provides a foundation for advanced search capabilities while dramatically improving the user experience through parallel processing and progressive enhancement.
