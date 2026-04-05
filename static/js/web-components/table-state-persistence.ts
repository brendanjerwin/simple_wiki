import type { SortDirection, ColumnFilterState } from './table-sorter-filterer.js';

const STORAGE_PREFIX = 'wiki-table-state:';
const TTL_MS = 90 * 24 * 60 * 60 * 1000; // 90 days

export interface PersistedTableState {
  sortColumnIndex: number | null;
  sortDirection: SortDirection;
  filters: Array<[number, SerializedFilterState]>;
}

interface SerializedFilterState {
  kind: 'checkbox' | 'range' | 'text-search';
  excludedValues?: string[];
  min?: number | null;
  max?: number | null;
  searchText?: string;
}

interface StorageEnvelope {
  state: PersistedTableState;
  expiresAtMs: number;
}

export function computeTableHash(headerTexts: string[], cellValues: string[][]): string {
  const content = headerTexts.join('\0') + '\0\0' +
    cellValues.map(row => row.join('\0')).join('\n');
  const buf = new Int32Array(1);
  for (let i = 0; i < content.length; ) {
    const char = content.codePointAt(i) ?? 0;
    const prev = buf[0] ?? 0;
    buf[0] = (prev << 5) - prev + char;
    i += char > 0xFFFF ? 2 : 1;
  }
  return ((buf[0] ?? 0) >>> 0).toString(36);
}

export function serializeFilter(filter: ColumnFilterState): SerializedFilterState {
  switch (filter.kind) {
    case 'checkbox':
      return {
        kind: 'checkbox',
        excludedValues: Array.from(filter.excludedValues).sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }) || a.localeCompare(b)),
      };
    case 'range':
      return {
        kind: 'range',
        min: filter.min,
        max: filter.max,
      };
    case 'text-search':
      return {
        kind: 'text-search',
        searchText: filter.searchText,
      };
  }
}

export function deserializeFilter(serialized: SerializedFilterState): ColumnFilterState {
  switch (serialized.kind) {
    case 'checkbox':
      return {
        kind: 'checkbox',
        excludedValues: new Set(serialized.excludedValues ?? []),
      };
    case 'range':
      return {
        kind: 'range',
        min: serialized.min ?? null,
        max: serialized.max ?? null,
      };
    case 'text-search':
      return {
        kind: 'text-search',
        searchText: serialized.searchText ?? '',
      };
  }
}

export function saveTableState(
  hash: string,
  sortColumnIndex: number | null,
  sortDirection: SortDirection,
  filters: Map<number, ColumnFilterState>,
): void {
  const serializedFilters: Array<[number, SerializedFilterState]> = [];
  for (const [colIndex, filter] of filters) {
    serializedFilters.push([colIndex, serializeFilter(filter)]);
  }

  const envelope: StorageEnvelope = {
    state: {
      sortColumnIndex,
      sortDirection,
      filters: serializedFilters,
    },
    expiresAtMs: Date.now() + TTL_MS,
  };

  try {
    localStorage.setItem(STORAGE_PREFIX + hash, JSON.stringify(envelope));
  } catch {
    // localStorage full or unavailable — silently ignore
  }
}

function isStorageEnvelope(value: unknown): value is StorageEnvelope {
  if (typeof value !== 'object' || value === null) return false;
  return 'expiresAtMs' in value && typeof value.expiresAtMs === 'number' &&
    'state' in value && typeof value.state === 'object' && value.state !== null;
}

export function loadTableState(hash: string): PersistedTableState | null {
  try {
    const key = STORAGE_PREFIX + hash;
    const raw = localStorage.getItem(key);
    if (raw === null) return null;

    const parsed: unknown = JSON.parse(raw);
    if (!isStorageEnvelope(parsed)) {
      localStorage.removeItem(key);
      return null;
    }
    if (Date.now() > parsed.expiresAtMs) {
      localStorage.removeItem(key);
      return null;
    }
    return parsed.state;
  } catch {
    // localStorage unavailable (privacy mode) or corrupt data — silently ignore
    return null;
  }
}
