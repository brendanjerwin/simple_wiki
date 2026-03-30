export type ColumnDataType = 'integer' | 'decimal' | 'currency' | 'percentage' | 'date' | 'text';

export interface ColumnTypeInfo {
  detectedType: ColumnDataType;
  confidenceRatio: number;
}

const currencyStandardPattern = /^-?[$€£¥]\s?[\d,]+(?:\.\d+)?$/;
const currencySymbolNegativePattern = /^[$€£¥]\s?-[\d,]+(?:\.\d+)?$/;
const percentagePattern = /^-?\d+(?:\.\d+)?%$/;
const numberPattern = /^-?[\d,]+(?:\.\d+)?$/;
const integerPattern = /^-?[\d,]+$/;
const isoDatePattern = /^\d{4}-\d{2}-\d{2}$/;
const usDatePattern = /^\d{1,2}\/\d{1,2}\/\d{4}$/;
const humanDatePattern = /^[A-Z][a-z]{2}\s+\d{1,2},?\s+\d{4}$/;

const confidenceThreshold = 0.7;

function isCurrency(text: string): boolean {
  const trimmed = text.trim();
  return currencyStandardPattern.test(trimmed) || currencySymbolNegativePattern.test(trimmed);
}

function isPercentage(text: string): boolean {
  return percentagePattern.test(text.trim());
}

function isNumber(text: string): boolean {
  return numberPattern.test(text.trim());
}

function isInteger(text: string): boolean {
  return integerPattern.test(text.trim());
}

function isDate(text: string): boolean {
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

  const numberMatchCount = nonEmpty.filter(t => isNumber(t)).length;
  const numberRatio = numberMatchCount / nonEmpty.length;
  if (numberRatio >= confidenceThreshold) {
    const allIntegers = nonEmpty.filter(t => isNumber(t)).every(t => isInteger(t));
    return {
      detectedType: allIntegers ? 'integer' : 'decimal',
      confidenceRatio: numberRatio,
    };
  }

  return { detectedType: 'text', confidenceRatio: 1 };
}

export function parseForType(text: string, columnType: ColumnDataType): number {
  switch (columnType) {
    case 'integer':
    case 'decimal': return parseNumericValue(text);
    case 'currency': return parseCurrencyValue(text);
    case 'percentage': return parsePercentageValue(text);
    case 'date': return parseDateValue(text);
    default: return NaN;
  }
}

export function parseNumericValue(text: string): number {
  const cleaned = text.trim().replace(/,/g, '');
  if (cleaned === '') return NaN;
  return Number(cleaned);
}

export function parseDateValue(text: string): number {
  const trimmed = text.trim();

  if (isoDatePattern.test(trimmed)) {
    const epochMs = Date.parse(trimmed + 'T00:00:00Z');
    return Number.isNaN(epochMs) ? NaN : epochMs;
  }

  if (usDatePattern.test(trimmed)) {
    const [month, day, year] = trimmed.split('/');
    const isoStr = `${year}-${month!.padStart(2, '0')}-${day!.padStart(2, '0')}T00:00:00Z`;
    const epochMs = Date.parse(isoStr);
    return Number.isNaN(epochMs) ? NaN : epochMs;
  }

  if (humanDatePattern.test(trimmed)) {
    const epochMs = Date.parse(trimmed);
    return Number.isNaN(epochMs) ? NaN : epochMs;
  }

  return NaN;
}

export function parseCurrencyValue(text: string): number {
  const trimmed = text.trim();
  if (trimmed === '') return NaN;
  const negative = trimmed.startsWith('-') || /^[$€£¥]\s?-/.test(trimmed);
  const cleaned = trimmed.replace(/^-?\s*[$€£¥]\s?-?/, '').replace(/,/g, '');
  if (cleaned === '') return NaN;
  const value = Number(cleaned);
  const signedValue = negative ? -value : value;
  return Number.isNaN(value) ? NaN : signedValue;
}

export function parsePercentageValue(text: string): number {
  const trimmed = text.trim();
  if (!trimmed.endsWith('%')) {
    return NaN;
  }
  const cleaned = trimmed.slice(0, -1);
  return Number(cleaned);
}
