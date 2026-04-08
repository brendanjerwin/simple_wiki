/**
 * Shared timeout constants for E2E tests.
 * Values chosen to give components enough time to load and respond to async
 * operations (gRPC round-trips, debounce delays, etc.) even on slow CI runners.
 */

/** Time allowed for a web component to finish loading/rendering. */
export const COMPONENT_LOAD_TIMEOUT_MS = 15000;

/** Time allowed for identifier auto-generation (debounce + gRPC round-trip). */
export const IDENTIFIER_GENERATE_TIMEOUT_MS = 10000;
