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

/** Runtime check for a string-keyed plain record. */
function isPlainRecord(v: unknown): v is Record<string, unknown> {
 return v != null && typeof v === 'object' && !Array.isArray(v);
}

/** Copy `raw[key]` into the step when the raw value satisfies `pred`. */
function pickOptional(step: PlanStep, raw: Record<string, unknown>, key: string, pred: (v: unknown) => boolean): void {
 const value = raw[key];
 if (value !== undefined && pred(value)) step[key] = value;
}

function pickTargets(step: PlanStep, raw: Record<string, unknown>): void {
 const targets = raw['targets'];
 if (Array.isArray(targets)) {
  step.targets = targets.filter((t): t is string => typeof t === 'string');
 }
}

function pickRefType(step: PlanStep, raw: Record<string, unknown>): void {
 const refType = raw['refType'];
 if (refType === 'query' || refType === 'mutation') {
  step.refType = refType;
 }
}

function pickParams(step: PlanStep, raw: Record<string, unknown>): void {
 const params = raw['params'];
 if (isPlainRecord(params)) {
  step.params = { ...params };
 }
}

/** Runtime check for a step-shaped raw value: plain object with a string `type`. */
function isStepRaw(v: unknown): v is Record<string, unknown> & { type: string } {
 return isPlainRecord(v) && typeof v['type'] === 'string';
}

/** Validate one raw step object into a PlanStep, or undefined when malformed. */
export function toPlanStep(raw: unknown): PlanStep | undefined {
 if (!isStepRaw(raw)) return undefined;
 const step: PlanStep = { type: raw['type'] };
 // Shared predicate: one string guard feeds five pickOptional calls, so
 // behavior stays in lockstep and each call site stays a flat pick.
 const isString = (v: unknown): boolean => typeof v === 'string';

 pickOptional(step, raw, 'target', isString);
 pickOptional(step, raw, 'message', isString);
 pickOptional(step, raw, 'url', isString);
 pickOptional(step, raw, 'statementId', isString);
 pickOptional(step, raw, 'name', isString);
 pickTargets(step, raw);
 pickRefType(step, raw);
 pickParams(step, raw);
 if ('valueAST' in raw) step.valueAST = raw['valueAST'];
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