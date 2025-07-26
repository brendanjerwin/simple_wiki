# ADR-0004: Error Handling Architecture

## Status

Accepted

## Context

The application needs a consistent and robust error handling strategy that provides a good user experience while maintaining system stability. Previously, error handling was scattered throughout the codebase with inconsistent presentation and no clear strategy for when to catch versus when to let errors bubble up.

Key challenges addressed:

- Inconsistent error presentation across components
- No clear strategy for recoverable vs. unrecoverable errors
- Error handling logic mixed into UI components
- Risk of users continuing to work in undefined system states after errors

## Decision

We will implement a **selective exception handling architecture** with the following core principles:

### 1. Only Catch Exceptions You Can Meaningfully Handle

**Catch exceptions when:**

- You can provide meaningful user feedback and allow retry (e.g., network requests)
- You can gracefully degrade functionality while keeping the app usable
- You can recover automatically or provide alternative behavior
- The error is expected and part of normal operation (e.g., validation errors)

**Don't catch exceptions when:**

- You're just logging and re-throwing without handling
- The error represents a programming bug that should be fixed
- You can't provide any meaningful recovery or user action
- You're hiding genuine problems that should be addressed

### 2. Global Unhandled Error Handling

All unhandled errors bubble up to a global error handler that:

- Catches all unhandled JavaScript errors and promise rejections
- Displays a "kernel panic" screen with error details
- Prevents users from continuing to work in an undefined system state
- Allows users to refresh and restart the application

### 3. Consistent Error Presentation

All errors use a unified presentation system:

- **AugmentErrorService**: Augments errors with classification metadata (ErrorKind, icon, failedGoalDescription)
- **ErrorDisplay Component**: Provides consistent visual presentation with expand/collapse
- **Structured Error Types**: AugmentedError extends Error with additional metadata

### 4. Contextual Error Information

Errors include contextual information about what operation was being attempted:

- **failedGoalDescription**: Optional parameter describing the goal that was being attempted when the error occurred (e.g., "saving frontmatter", "loading user profile")
- **User-Friendly Display**: Error messages show as "Error while {failedGoalDescription}: {originalErrorMessage}"
- **Progressive Disclosure**: Technical details (stack traces) are available via expand/collapse

### 5. Error Classification System

Errors are classified using an ErrorKind enum:

- NETWORK: Connectivity and communication errors
- PERMISSION: Authorization and authentication errors
- VALIDATION: Input validation and data format errors
- NOT_FOUND: Missing resources or files
- TIMEOUT: Performance and deadline exceeded errors
- SERVER: Internal server and system errors
- WARNING: General warnings and recoverable issues
- ERROR: Critical errors and unspecified failures

Each kind maps to appropriate visual icons for quick user recognition.

## Implementation

### Core Components

1. **AugmentErrorService**: Converts any error into AugmentedError with metadata
2. **AugmentedError**: Error subclass with errorKind, icon, and failedGoalDescription properties
3. **ErrorDisplay**: Reusable web component for consistent error presentation
4. **Global Error Handler**: Catches unhandled errors and shows kernel panic

### Error Flow

```
Error Occurs → Can it be handled locally?
├─ Yes → Catch, augment with AugmentErrorService, show in ErrorDisplay
└─ No → Let bubble → Global handler → Kernel panic screen
```

### Usage Examples

**Recoverable Error (Handle Locally):**

```typescript
try {
  await this.client.saveDocument();
} catch (err) {
  this.augmentedError = AugmentErrorService.augmentError(err, 'saving document');
  // User sees "Error while saving document: Connection failed" and can retry
}
```

**Unrecoverable Error (Let Bubble):**

```typescript
private processData(data: unknown): ProcessedData {
  if (!this.isValidData(data)) {
    // Programming error - let it bubble to global handler
    throw new Error('Data corruption detected');
  }
  return this.transform(data);
}
```

## Benefits

1. **User Experience**: Clear, actionable error messages with consistent presentation
2. **System Stability**: Prevents users from working in undefined states after errors
3. **Developer Experience**: Clear guidelines on when to catch vs. let bubble
4. **Maintainability**: Centralized error processing and presentation logic
5. **Debugging**: Unhandled errors surface as kernel panics, making bugs visible

## Consequences

### Positive

- Consistent error handling patterns across the application
- Clear separation between recoverable and unrecoverable errors
- Better user feedback for operational issues
- Immediate visibility of programming errors through kernel panics

### Negative

- More upfront complexity in error handling setup
- Developers must think carefully about whether to catch errors
- Kernel panic screens may seem "dramatic" for some errors

### Risks Mitigated

- Users continuing to work after system errors
- Inconsistent error presentation across components
- Hidden bugs due to overly broad exception catching
- Business logic mixed into UI components

## Compliance with Design Principles

This architecture aligns with our core design principles:

- **Defensive Programming**: Errors are handled gracefully or surfaced clearly
- **Fail Fast**: Unrecoverable errors immediately surface as kernel panics
- **Single Responsibility**: Error processing separated from UI components
- **Consistency**: Unified error presentation and handling patterns

## Related Decisions

- Use of Lit Element web components for UI consistency
- Global error handler implementation for unhandled exceptions
- AugmentedError as extension of standard Error class
