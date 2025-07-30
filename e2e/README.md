# Simple Wiki E2E Tests

This directory contains Playwright-powered end-to-end tests for the Simple Wiki application.

## Overview

The E2E test suite focuses on critical path functionality following the testing triangle principle:

- Lightweight tests that verify the most valuable user workflows
- Builds the application fresh for each test run
- Uses clean test data directory for isolation
- Covers core functionality: page creation, editing, navigation, and persistence

## Running E2E Tests

### Quick Start

```bash
# Run all E2E tests
devbox run e2e:test

# Run with visible browser for debugging
devbox run e2e:test:headed
```

This single command will:

1. Build the application (`devbox run build`)
2. Set up a clean test data directory
3. Start the wiki server on port 8051
4. Install/configure Playwright browsers
5. Run the test suite
6. Clean up automatically

### Creating New E2E Tests

#### Interactive Test Development with Playwright Codegen

```bash
# Start the wiki server 
devbox services up

# Run Playwright codegen to record interactions
devbox run e2e:codegen
```

This will open a browser with Playwright's recording interface. Navigate through the wiki and Playwright will generate test code automatically.

**First-time setup:** If you get browser errors, run `devbox run e2e:setup` once to install Playwright browsers.

### Test Structure

The test suite includes:

- **Page Load Test**: Verifies the home page editing interface loads correctly
- **Navigation Test**: Tests URL-based navigation between different page types  
- **Basic Functionality Tests**: Creates pages, edits content, and verifies persistence
- **Cleanup Test**: Ensures test data is cleaned up after runs

### Current Status

As of implementation:

- ✅ **Infrastructure**: Playwright setup, browser automation, server management
- ✅ **Page Loading**: Home page and edit interface verification
- ✅ **URL Navigation**: Direct navigation to edit/view/list pages
- ✅ **Cleanup**: Automatic test data cleanup
- ⚠️ **Content Persistence**: Some issues with auto-save timing (known limitation)
- ⚠️ **Complex Workflows**: Page linking tests need refinement

### Manual Testing

For debugging, you can start the test server manually:

```bash
# Start server on test port with clean data
./simple_wiki-linux-amd64 --port 8051 --data ./e2e/test-data --debug
```

Then run individual Playwright tests:

```bash
cd e2e
bunx playwright test --headed  # Run with visible browser
bunx playwright test --debug   # Run with debugger
```

## Test Philosophy

These E2E tests follow the testing triangle principle:

- **Minimal and focused**: Only tests critical user paths
- **Fast and reliable**: Designed to run quickly in CI/CD environments  
- **Isolated**: Each test run uses fresh data and clean state
- **Defensive**: Tests handle timing issues and browser quirks gracefully

The goal is to catch regressions in core functionality while maintaining a lightweight test suite that provides confidence without excessive maintenance overhead.

## Configuration

- **Test Data**: Uses `./e2e/test-data/` directory (automatically cleaned up)
- **Test Port**: Server runs on port 8051 during tests
- **Browser**: Uses system Chromium with headless mode
- **Timeouts**: Configured for typical local development speeds

## Troubleshooting

### Common Issues

1. **Port conflicts**: If port 8051 is in use, tests will fail. Stop other services.
2. **Browser issues**: Tests use system Chromium. Ensure it's installed via devbox.
3. **Timing issues**: Some tests may need adjustment for slower systems.

### Debug Tools

- Screenshots are taken on test failures (saved in `test-results/`)
- Use `--headed` flag to see browser actions
- Check server logs for backend issues
- Use `--debug` flag for step-by-step debugging

## Extending Tests

When adding new E2E tests:

### Using the Development Environment

1. **Start the development environment**: `devbox run e2e:codegen`
2. **Use Playwright codegen**: Follow the on-screen instructions to record interactions
3. **Focus on critical workflows**: Only test the most important user paths
4. **Test edge cases**: Include both happy path and error scenarios
5. **Keep tests isolated**: Each test should be independent

### Best Practices

1. Focus on critical user workflows only
2. Use descriptive test names that explain the behavior being tested
3. Ensure proper cleanup in test teardown
4. Test both happy path and edge cases
5. Keep tests isolated and independent
6. Use the existing patterns shown in the codegen environment

### Test File Organization

- **Main test file**: `e2e/tests/critical-paths.spec.ts`
- **New test files**: Create additional `.spec.ts` files in `e2e/tests/` as needed
- **Helper functions**: Extract common patterns into reusable functions
