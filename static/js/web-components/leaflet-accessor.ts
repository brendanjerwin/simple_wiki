// Leaflet accessor module. ES module namespaces are frozen, so sinon.stub
// fails with "ES Modules cannot be stubbed". We wrap the Leaflet namespace
// in a Proxy that intercepts property access and allows test stubs to
// override individual functions via a mutable override map.
import * as LNamespace from 'leaflet';

const overrides = new Map<string, unknown>();

export const Leaflet: typeof LNamespace = new Proxy(LNamespace, {
  get(_target, prop: string) {
    if (overrides.has(prop)) return overrides.get(prop);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Proxy access to frozen ES module namespace requires casting to a record type
    return (LNamespace as Record<string, unknown>)[prop];
  },
  getOwnPropertyDescriptor(_target, prop: string) {
    // sinon checks for property existence before stubbing; report it as
    // configurable and writable so sinon proceeds.
    if (overrides.has(prop) || prop in LNamespace) {
      const value = overrides.get(prop) ?? (LNamespace as Record<string, unknown>)[prop];
      return { configurable: true, enumerable: true, writable: true, value };
    }
    return undefined;
  },
}) as typeof LNamespace;

// Test helper: replace a Leaflet export with a stub. Call in beforeEach,
// restore via sinon.restore() is NOT sufficient — use resetLeafletOverrides()
// in afterEach.
export function setLeafletOverride(key: string, value: unknown): void {
  overrides.set(key, value);
}

export function resetLeafletOverrides(): void {
  overrides.clear();
}