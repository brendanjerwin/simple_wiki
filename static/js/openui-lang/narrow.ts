/**
 * Runtime narrowing helpers for component renderers.
 *
 * Component props arrive as `Record<string, unknown>` (parsed AST values,
 * possibly reactive markers). The wiki renderer contract keeps them opaque,
 * and the eslint `no-unsafe-type-assertion` rule forbids narrowing casts —
 * so renderers narrow with these validators instead.
 */

export function asString(v: unknown, fallback = ''): string {
 return typeof v === 'string' ? v : fallback;
}

export function asStringArray(v: unknown): string[] {
 return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
}

export function asNumberArray(v: unknown): number[] {
 return Array.isArray(v) ? v.filter((x): x is number => typeof x === 'number') : [];
}

export function asArray(v: unknown): unknown[] {
 return Array.isArray(v) ? v : [];
}

export function asNumber(v: unknown, fallback = 0): number {
 return typeof v === 'number' ? v : fallback;
}

export function asPlan(v: unknown): PlanLike | undefined {
 if (v != null && typeof v === 'object' && 'steps' in (v as Record<string, unknown>)) {
  return v as PlanLike;
 }
 return undefined;
}

/** Minimal structural shape the host needs from an action plan. */
export interface PlanLike {
 steps: unknown[];
}