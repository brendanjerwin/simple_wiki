export type ColumnDataType = 'integer' | 'decimal' | 'currency' | 'percentage' | 'date' | 'text';

export interface ColumnTypeInfo {
  detectedType: ColumnDataType;
  confidenceRatio: number;
}

const currencyLeadNegativePattern = /^-?[$€£¥]\s{0,10}[\d,]+(?:\.\d+)?$/;
const currencyMidNegativePattern = /^[$€£¥]\s{0,10}-[\d,]+(?:\.\d+)?$/;
const percentagePattern = /^-?\d+(?:\.\d+)?%$/;
const numberPattern = /^-?[\d,]+(?:\.\d+)?$/;
const integerPattern = /^-?[\d,]+$/;
const isoDatePattern = /^\d{4}-\d{2}-\d{2}$/;
const usDatePattern = /^\d{1,2}\/\d{1,2}\/\d{4}$/;
const humanDatePattern = /^[A-Z][a-z]{2}\s{1,10}\d{1,2},?\s{1,10}\d{4}$/;

const confidenceThreshold = 0.7;

function isCurrency(text: string) {
  const trimmed = text.trim();
  return currencyLeadNegativePattern.test(trimmed) || currencyMidNegativePattern.test(trimmed);
}

function isPercentage(text: string) {
  return percentagePattern.test(text.trim());
}

function isNumber(text: string) {
  return numberPattern.test(text.trim());
}

function isInteger(text: string) {
  return integerPattern.test(text.trim());
}

function isDate(text: string) {
  const trimmed = text.trim();
  return isoDatePattern.test(trimmed) || usDatePattern.test(trimmed) || humanDatePattern.test(trimmed);
}

type TypeChecker = { type: ColumnDataType; test: (text: string) => boolean };

const typeCheckers: TypeChecker[] = [
  { type: 'currency', test: isCurrency },
  { type: 'percentage', test: isPercentage },
  { type: 'date', test: isDate },
];

export function detectColumnType(cellTexts: string[]): ColumnTypeInfo {
  const nonEmpty = cellTexts.filter(t => t.trim() !== '');

  if (nonEmpty.length === 0) {
    return { detectedType: 'text', confidenceRatio: 0 };
  }

  for (const checker of typeCheckers) {
    const matchCount = nonEmpty.filter(t => checker.test(t)).length;
    const ratio = matchCount / nonEmpty.length;
    if (ratio >= confidenceThreshold) {
      return { detectedType: checker.type, confidenceRatio: ratio };
    }
  }

  const numberMatches = nonEmpty.filter(isNumber);
  const numberMatchCount = numberMatches.length;
  const numberRatio = numberMatchCount / nonEmpty.length;
  if (numberRatio >= confidenceThreshold) {
    const allIntegers = numberMatches.every(isInteger);
    return {
      detectedType: allIntegers ? 'integer' : 'decimal',
      confidenceRatio: numberRatio,
    };
  }

  return { detectedType: 'text', confidenceRatio: 1 };
}

export function parseForType(text: string, columnType: ColumnDataType) {
  switch (columnType) {
    case 'integer':
    case 'decimal': return parseNumericValue(text);
    case 'currency': return parseCurrencyValue(text);
    case 'percentage': return parsePercentageValue(text);
    case 'date': return parseDateValue(text);
    default: return Number.NaN;
  }
}

export function parseNumericValue(text: string) {
  const cleaned = text.trim().replaceAll(',', '');
  if (cleaned === '') return Number.NaN;
  return Number(cleaned);
}

export function parseDateValue(text: string) {
  const trimmed = text.trim();

  if (isoDatePattern.test(trimmed)) {
    const epochMs = Date.parse(trimmed + 'T00:00:00Z');
    return Number.isNaN(epochMs) ? Number.NaN : epochMs;
  }

  if (usDatePattern.test(trimmed)) {
    const [month, day, year] = trimmed.split('/');
    if (!month || !day || !year) {
      return Number.NaN;
    }
    const isoStr = `${year}-${month.padStart(2, '0')}-${day.padStart(2, '0')}T00:00:00Z`;
    const epochMs = Date.parse(isoStr);
    return Number.isNaN(epochMs) ? Number.NaN : epochMs;
  }

  if (humanDatePattern.test(trimmed)) {
    const epochMs = Date.parse(trimmed);
    return Number.isNaN(epochMs) ? Number.NaN : epochMs;
  }

  return Number.NaN;
}

export function parseCurrencyValue(text: string) {
  const trimmed = text.trim();
  if (trimmed === '') return Number.NaN;
  const negative = trimmed.startsWith('-') || /^[$€£¥]\s{0,10}-/.test(trimmed);
  const cleaned = trimmed.replace(/^-?\s{0,10}[$€£¥]\s{0,10}-?/, '').replaceAll(',', '');
  if (cleaned === '') return Number.NaN;
  const value = Number(cleaned);
  if (Number.isNaN(value)) {
    return Number.NaN;
  }
  return negative ? -value : value;
}

export function parsePercentageValue(text: string) {
  const trimmed = text.trim();
  if (!trimmed.endsWith('%')) {
    return Number.NaN;
  }
  const cleaned = trimmed.slice(0, -1);
  return Number(cleaned);
}
