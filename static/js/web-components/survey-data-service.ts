import type { JsonObject } from '@bufbuild/protobuf';

/**
 * Narrow `value` to a non-null, non-array object, or return null.
 */
export function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- narrowed above: non-null, non-array object
  return value as Record<string, unknown>;
}

export interface SurveyField {
  name: string;
  type: 'number' | 'text' | 'choice' | 'boolean';
  min?: number;
  max?: number;
  options?: string[];
}

export interface SurveyResponse {
  user: string;
  anonymous: boolean;
  submitted_at: string;
  values: Record<string, unknown>;
}

export interface SurveyData {
  question: string;
  fields: SurveyField[];
  closed: boolean;
  responses: SurveyResponse[];
}

function parseSurveyField(raw: unknown): SurveyField | null {
  const r = asRecord(raw);
  if (!r) return null;
  const name = typeof r['name'] === 'string' ? r['name'] : '';
  if (!name) return null;
  const rawType = typeof r['type'] === 'string' ? r['type'] : 'text';
  const type: SurveyField['type'] =
    rawType === 'number' || rawType === 'text' || rawType === 'choice' || rawType === 'boolean'
      ? rawType
      : 'text';
  const field: SurveyField = { name, type };
  if (typeof r['min'] === 'number') field.min = r['min'];
  if (typeof r['max'] === 'number') field.max = r['max'];
  if (Array.isArray(r['options'])) {
    field.options = r['options'].filter((o): o is string => typeof o === 'string');
  }
  return field;
}

function parseSurveyResponse(raw: unknown): SurveyResponse | null {
  const r = asRecord(raw);
  if (!r) return null;
  const user = typeof r['user'] === 'string' ? r['user'] : '';
  if (!user) return null;
  const valuesRaw = asRecord(r['values']) ?? {};
  return {
    user,
    anonymous: Boolean(r['anonymous']),
    submitted_at: typeof r['submitted_at'] === 'string' ? r['submitted_at'] : '',
    values: valuesRaw as Record<string, unknown>,
  };
}

/**
 * Extract SurveyData from the raw frontmatter object.
 */
export function extractSurveyData(frontmatter: JsonObject, surveyName: string): SurveyData {
  const defaultData: SurveyData = {
    question: '',
    fields: [],
    closed: false,
    responses: [],
  };

  const surveysObj = asRecord(frontmatter['surveys']);
  if (!surveysObj) return defaultData;

  const surveyObj = asRecord(surveysObj[surveyName]);
  if (!surveyObj) return defaultData;

  const question = typeof surveyObj['question'] === 'string' ? surveyObj['question'] : '';
  const closed = Boolean(surveyObj['closed']);

  const fields: SurveyField[] = [];
  if (Array.isArray(surveyObj['fields'])) {
    for (const raw of surveyObj['fields']) {
      const field = parseSurveyField(raw);
      if (field) fields.push(field);
    }
  }

  const responses: SurveyResponse[] = [];
  if (Array.isArray(surveyObj['responses'])) {
    for (const raw of surveyObj['responses']) {
      const resp = parseSurveyResponse(raw);
      if (resp) responses.push(resp);
    }
  }

  return { question, fields, closed, responses };
}

/**
 * Find the existing response for the given username, or null if not found.
 */
export function findUserResponse(
  responses: SurveyResponse[],
  username: string
): SurveyResponse | null {
  if (!username) return null;
  return responses.find(r => r.user === username) ?? null;
}

/**
 * Build an updated responses array by replacing or appending the user's response.
 */
export function upsertResponse(
  responses: SurveyResponse[],
  username: string,
  values: Record<string, unknown>
): SurveyResponse[] {
  const now = new Date().toISOString();
  const newResponse: SurveyResponse = {
    user: username,
    anonymous: false,
    submitted_at: now,
    values,
  };
  const existingIndex = responses.findIndex(r => r.user === username);
  if (existingIndex >= 0) {
    return responses.map((r, i) => (i === existingIndex ? newResponse : r));
  }
  return [...responses, newResponse];
}
