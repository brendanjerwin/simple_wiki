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

#### Worker Pool Implementation

- **Configurable Concurrency**: Default to `runtime.NumCPU()` workers, configurable via CLI
- **Work Queue**: Channel-based distribution of page identifiers to workers
- **Error Isolation**: Individual page failures don't stop other processing
- **Graceful Shutdown**: Workers can be stopped cleanly during application shutdown

#### Progress Tracking

Multi-index progress monitoring across all index types:

```go
type IndexingProgress struct {
    OverallProgress    float64
    IndexProgress      map[string]IndexProgress  // "frontmatter", "bleve", "vector"
    QueueDepth         int
    ProcessingRate     float64
    EstimatedCompletion *time.Duration
}
```

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
    
    // Create background coordinator instead of direct MultiMaintainer
    backgroundCoordinator := NewBackgroundIndexingCoordinator(
        index.NewMultiMaintainer(frontmatterIndex, bleveIndex),
        s.Logger,
    )
    
    s.IndexMaintainer = backgroundCoordinator
    
    // Start background indexing instead of synchronous loop
    pageIdentifiers := s.getPageIdentifiers()
    return backgroundCoordinator.StartBackground(pageIdentifiers)
}
```

#### System Monitoring

- **gRPC Service Extension**: Add indexing status to existing system info service
- **UI Progress Display**: Visual progress indicators for all index types
- **Health Checks**: Application health independent of indexing status

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
