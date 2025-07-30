# ADR 0005: Rolling Migrations System for Frontmatter Transformations

## Status

Proposed

## Context

We have encountered situations where frontmatter stored in the wiki needs to be transformed to handle:

1. **Conflicting TOML frontmatter syntax**: Legacy content with `inventory.container = "value"` combined with `[inventory]` tables causes parsing conflicts
2. **Frontmatter format evolution**: As the wiki evolves, we may need to transform frontmatter structure, field names, or syntax
3. **Backward compatibility**: We need to support existing deployments without requiring manual frontmatter migration
4. **Data integrity**: Transformations must be safe and reliable

### Current Problem

The immediate issue is TOML frontmatter with mixed dot notation and explicit table syntax:

```toml
+++
inventory.container = "fireplace_cabinet_right"
[inventory]
items = []
+++
```

This creates a conflict because both syntaxes attempt to define the `inventory` table, causing parsing failures.

## Decision

We will implement a **Rolling Migrations System** that automatically transforms frontmatter as it's accessed, with the following design principles:

### Architecture Overview

1. **Migration Registry**: Central registry of all available migrations
2. **Pattern-Based Application**: Migrations use robust pattern matching to determine applicability - no tracking needed
3. **Automatic Application**: Apply migrations when content is accessed (including during index initialization)
4. **Stateless Design**: No metadata storage required - migrations are self-determining

### Core Components

#### Migration Interface
```go
type FrontmatterMigration interface {
    AppliesTo(content []byte) bool        // Check if frontmatter migration is needed
    Apply(content []byte) ([]byte, error) // Transform frontmatter content
}
```

#### Migration Applicator
```go
type FrontmatterMigrationApplicator interface {
    RegisterMigration(migration FrontmatterMigration)
    ApplyMigrations(content []byte) ([]byte, error)
}
```

### Pattern-Based Detection

Migrations rely entirely on pattern matching to determine applicability. Each migration implements robust detection logic:

```go
func (m *TOMLDotNotationMigration) AppliesTo(content []byte) bool {
    // Only apply to TOML frontmatter with specific conflict pattern
    return bytes.HasPrefix(content, []byte("+++")) && 
           hasConflictingDotNotationAndTables(content)
}
```

No metadata storage or tracking is needed - migrations are self-determining based on content patterns.

### Integration Points

Migrations will be integrated at a single, well-defined point in the frontmatter processing pipeline:

```go
// In server/site.go - lenientParse function
func lenientParse(content []byte, matter any) (body []byte, err error) {
    // Apply frontmatter migrations BEFORE parsing
    migratedContent, err := frontmatterMigrationApplicator.ApplyMigrations(content)
    if err != nil {
        // Log migration failure - migratedContent already contains original content
        log.Printf("Migration failed, using original content: %v", err)
    }
    
    // Continue with existing parsing logic using migratedContent
    body, err = adrgfrontmatter.Parse(bytes.NewReader(migratedContent), matter)
    // ... rest of existing logic
}
```

This ensures migrations run:
- During `ReadFrontMatter()` calls (including index initialization)
- Before any frontmatter parsing attempts
- Only once per content access (no caching needed due to pattern-based design)

### Migration Execution Order

When multiple migrations match the same content:
1. Migrations execute in **registration order** (first registered, first executed)
2. Each migration receives the output of the previous migration
3. If any migration fails, the process stops and returns original content
4. Migrations must be designed to not conflict with each other

### Error Handling Strategy

```go
func (a *FrontmatterMigrationApplicator) ApplyMigrations(content []byte) ([]byte, error) {
    current := content
    for _, migration := range a.migrations {
        if migration.AppliesTo(current) {
            migrated, err := migration.Apply(current)
            if err != nil {
                // Log error and return original content - site continues to function
                return content, fmt.Errorf("migration failed: %w", err)
            }
            current = migrated
        }
    }
    return current, nil
}
```

**Error Recovery**:
- Migration failures return original content with error logging
- Site continues to function with unmigrated content
- No retry logic to prevent infinite loops
- Operators can fix content manually if needed

### Migration Categories (Frontmatter Only)

1. **Syntax Migrations**: Transform frontmatter syntax conflicts (TOML, YAML formatting issues)
2. **Structure Migrations**: Reorganize frontmatter field hierarchy or nesting
3. **Field Migrations**: Rename or transform specific frontmatter fields
4. **Validation Migrations**: Fix invalid frontmatter that causes parsing errors

**Note**: This system is specifically scoped to frontmatter transformations. Future content or file organization migrations would require separate systems.

### Safety Mechanisms

1. **Robust Pattern Matching**: Migrations only run when precise patterns are detected
2. **Transformation Validation**: Migrations transform content to patterns that won't match again
3. **Comprehensive Testing**: Each `AppliesTo()` method extensively unit tested with edge cases
4. **Conservative Approach**: Pattern matching errs on the side of caution - better to miss than corrupt
5. **Backup Integration**: Leverage existing versioned storage for recovery
6. **Error Recovery**: Graceful handling of migration failures with fallback to original content
7. **Progressive Enhancement**: Site continues to function even if migrations fail

### Performance Considerations

**Startup Impact**: 
- During index initialization, all files are read and migrations will run
- Pattern matching must be efficient (simple byte/string operations preferred)
- Complex TOML parsing in `AppliesTo()` should be minimized

**Runtime Impact**:
- Migrations only run when `AppliesTo()` returns true
- Transformed content won't trigger migrations again
- No caching needed due to pattern-based design

**Optimization Strategies**:
- Fast path checks (file extension, content prefixes) before expensive pattern matching
- Lazy compilation of regex patterns
- Early return from `AppliesTo()` when possible

## Implementation Plan

### Phase 1: Core Infrastructure
- Create `rollingmigrations` package with frontmatter-focused interfaces
- Implement `FrontmatterMigrationApplicator` with migration registry and execution order
- Create comprehensive unit tests for pattern matching and error handling
- Add performance benchmarks for migration execution

### Phase 2: TOML Conflict Resolution Migration
- Implement `TOMLDotNotationMigration` to resolve the immediate issue
- Add extensive tests for various TOML conflict scenarios
- Integrate with existing frontmatter parsing pipeline

### Phase 3: Integration and Monitoring
- Integrate migrations into `lenientParse()` function in `server/site.go`
- Add structured logging for migration application and failures
- Monitor migration performance during startup indexing
- Add metrics for migration success/failure rates

### Phase 4: Additional Migrations
- Framework is ready for future frontmatter transformations
- Documentation and examples for creating new migrations
- Migration development guidelines and testing standards

### Migration Development Guidelines

**Creating New Migrations**:
1. Implement `FrontmatterMigration` interface
2. Write extensive unit tests for `AppliesTo()` with edge cases:
   - Empty content, malformed frontmatter, binary content
   - Content that looks like it should match but shouldn't
   - Content at boundaries of pattern matching logic
3. Ensure `Apply()` transforms content so `AppliesTo()` returns false
4. Add performance benchmarks if pattern matching is complex
5. Document the specific problem being solved and transformation applied

**Code Review Requirements**:
- Migration pattern matching logic reviewed by 2+ developers
- Test coverage must include edge cases and performance tests
- Migration must be registered in correct order relative to existing migrations

## Example: TOML Dot Notation Migration

```go
type TOMLDotNotationMigration struct{}

func (m *TOMLDotNotationMigration) AppliesTo(content []byte) bool {
    // Check for TOML with both dot notation and explicit tables
    return bytes.HasPrefix(content, []byte("+++")) && 
           containsDotNotationConflict(content)
}

func (m *TOMLDotNotationMigration) Apply(content []byte) ([]byte, error) {
    // Transform conflicting TOML to use consistent table syntax
    // Result will not match AppliesTo() pattern, preventing re-application
    return resolveTomlDotNotationConflicts(content)
}
```

## Benefits

1. **Backward Compatibility**: Existing content continues to work without manual intervention
2. **Simplicity**: No metadata storage or tracking - migrations are self-contained
3. **Automatic Application**: Migrations apply transparently when content is accessed
4. **Extensibility**: Easy to add new migrations as requirements evolve
5. **Safety**: Robust pattern matching prevents incorrect application
6. **Performance**: Only applicable migrations run, transformed content won't match again

## Risks and Mitigations

### Risk: Migration Failures
**Mitigation**: Comprehensive error handling, fallback to original content, logging

### Risk: Performance Impact
**Mitigation**: Efficient pattern matching, migrations only run when needed, transformed content won't trigger re-application

### Risk: Data Corruption
**Mitigation**: Extensive testing, precise pattern matching, backup integration, migrations transform to non-matching patterns

### Risk: Pattern Matching Errors
**Mitigation**: Extensive unit testing of `AppliesTo()` with edge cases, comprehensive test coverage, conservative pattern matching, thorough validation of transformation results

## Alternatives Considered

1. **One-time batch migration**: Requires downtime and manual intervention
2. **Version-specific parsers**: Leads to code bloat and maintenance burden
3. **Manual content fixes**: Not scalable for deployed instances
4. **Content validation only**: Doesn't solve the compatibility issue

## Consequences

- **Positive**: Seamless backward compatibility, extensible architecture, automated content evolution
- **Negative**: Additional complexity in content reading pipeline, new failure modes to handle
- **Neutral**: New package to maintain, migration registry to manage

This system will resolve the immediate TOML parsing issue while providing a robust foundation for future content transformations.