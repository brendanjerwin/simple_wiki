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

export function asStringOrUndefined(v: unknown): string | undefined {
 return typeof v === 'string' ? v : undefined;
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

/** A validated action step used by the host's plan executor. */
export interface PlanStep {
 type: string;
 target?: string;
 targets?: string[];
 valueAST?: unknown;
 message?: string;
 url?: string;
 statementId?: string;
 refType?: 'query' | 'mutation';
 /** Library action steps (custom_action): the action name and its params. */
 name?: string;
 params?: Record<string, unknown>;
 [key: string]: unknown;
}

/** Validate one raw step object into a PlanStep, or undefined when malformed. */
export function toPlanStep(raw: unknown): PlanStep | undefined {
 if (raw === null || typeof raw !== 'object' || !('type' in raw) || typeof raw.type !== 'string') {
  return undefined;
 }
 const step: PlanStep = { type: raw.type };
 if ('target' in raw && typeof raw.target === 'string') step.target = raw.target;
 if ('targets' in raw && Array.isArray(raw.targets)) {
  step.targets = raw.targets.filter((t): t is string => typeof t === 'string');
 }
 if ('valueAST' in raw) step.valueAST = raw.valueAST;
 if ('message' in raw && typeof raw.message === 'string') step.message = raw.message;
 if ('url' in raw && typeof raw.url === 'string') step.url = raw.url;
 if ('statementId' in raw && typeof raw.statementId === 'string') step.statementId = raw.statementId;
 if ('refType' in raw && (raw.refType === 'query' || raw.refType === 'mutation')) step.refType = raw.refType;
 if ('name' in raw && typeof raw.name === 'string') step.name = raw.name;
 if ('params' in raw && raw.params != null && typeof raw.params === 'object' && !Array.isArray(raw.params)) {
  const params: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(raw.params)) params[key] = value;
  step.params = params;
 }
 return step;
}

/** Minimal structural shape the host needs from an action plan. */
export interface PlanLike {
 steps: PlanStep[];
}

export function asPlan(v: unknown): PlanLike | undefined {
 if (v == null || typeof v !== 'object' || !('steps' in v) || !Array.isArray(v.steps)) {
  return undefined;
 }
 const steps: PlanStep[] = [];
 for (const raw of v.steps) {
  const step = toPlanStep(raw);
  if (step !== undefined) {
   steps.push(step);
  }
 }
 return { steps };
}