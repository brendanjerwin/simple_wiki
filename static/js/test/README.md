# Testing Infrastructure

This directory contains the testing infrastructure for the JavaScript web components.

## Setup

The testing infrastructure uses:
- **Vitest** - Fast test runner with great ES module support
- **Happy DOM** - Lightweight DOM implementation for testing
- **Vitest's built-in mocking** - For mocking dependencies

## Running Tests

```bash
# Run all tests once
bun test

# Run tests in watch mode
bun run test:watch

# Run tests with coverage
bun run test:coverage
```

## Test Files

- `test/wiki-search.test.js` - Tests for the WikiSearch web component
- `test/setup.js` - Test setup and cleanup utilities
- `vitest.config.js` - Vitest configuration

## What's Tested

The tests verify:
1. Component instantiation
2. Event listener management (memory leak prevention)
3. Keyboard shortcut handling (Ctrl+K / Cmd+K)
4. Proper cleanup on component disconnection
5. Multiple connect/disconnect cycles to ensure no memory leaks

## Memory Leak Prevention

The tests specifically verify that:
- Event listeners are properly added when the component connects to the DOM
- Event listeners are properly removed when the component disconnects from the DOM
- The same bound function reference is used for both adding and removing listeners
- Multiple connect/disconnect cycles don't accumulate event listeners