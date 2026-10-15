/**
 * Hashtag extraction and normalization shared between the page-body renderer
 * (server-side via `internal/hashtags`) and frontend components such as the
 * checklist input parser.
 *
 * The grammar matches the Go implementation: a `#` counts as a tag start only
 * when preceded by start-of-string, whitespace, or punctuation other than `[`
 * and `(`. Tag chars are Unicode letters/digits, `-`, and `_`. Tags inside
 * fenced code blocks (```) or inline code spans (`) are skipped, `\#` is an
 * escape, and tags are capped at MAX_TAG_LEN runes.
 *
 * Normalization is NFKC + lowercase only — hyphens and underscores are NOT
 * collapsed (so `#home-lab` and `#home_lab` stay distinct).
 */

export const MAX_TAG_LEN = 64;

const FENCE_MARKER = '```';

const TAG_CHAR_RE = /[\p{L}\p{N}\-_]/u;
const LETTER_OR_DIGIT_RE = /[\p{L}\p{N}]/u;
const PUNCT_OR_SYMBOL_RE = /[\p{P}\p{S}]/u;

/**
 * Reports whether `prev` is a rune that may immediately precede a `#` for the
 * `#` to count as the start of a tag.
 */
function isTagBoundary(prev: string): boolean {
  if (prev === '[' || prev === '(') return false;
  if (/\s/u.test(prev)) return true;
  if (LETTER_OR_DIGIT_RE.test(prev)) return false;
  return PUNCT_OR_SYMBOL_RE.test(prev);
}

function isTagChar(ch: string): boolean {
  return TAG_CHAR_RE.test(ch);
}

// CodeStateAdvance captures the outcome of advanceCodeState for a single
// character position: whether the character was consumed by a code-span or
// fence transition, how many extra positions to skip (FENCE_MARKER is 3
// chars; we skip 2 extra), and the updated parser state.
interface CodeStateAdvance {
  consumed: boolean;
  extraI: number;
  newPrev: string;
  newInFence: boolean;
  newInInlineCode: boolean;
}

// advanceCodeState handles fence-marker and inline-code transitions at
// position i. Returns a CodeStateAdvance; when consumed is true, the caller
// should apply extraI, newPrev, newInFence, newInInlineCode and `continue`
// the outer loop without further processing of this character.
function advanceCodeState(
  chars: string[],
  i: number,
  prev: string,
  inFence: boolean,
  inInlineCode: boolean,
): CodeStateAdvance {
  const ch = chars[i] ?? '';

  if (!inInlineCode && atFenceMarker(chars, i, prev)) {
    return { consumed: true, extraI: FENCE_MARKER.length - 1, newPrev: '`', newInFence: !inFence, newInInlineCode: inInlineCode };
  }
  if (inFence) {
    return { consumed: true, extraI: 0, newPrev: ch, newInFence: inFence, newInInlineCode: inInlineCode };
  }
  if (ch === '`') {
    return { consumed: true, extraI: 0, newPrev: ch, newInFence: inFence, newInInlineCode: !inInlineCode };
  }
  if (inInlineCode) {
    return { consumed: true, extraI: 0, newPrev: ch, newInFence: inFence, newInInlineCode: inInlineCode };
  }
  return { consumed: false, extraI: 0, newPrev: ch, newInFence: inFence, newInInlineCode: inInlineCode };
}

// tryAddHashtag reads a tag starting at chars[start], normalizes it, and
// appends it to result if it hasn't been seen yet. Returns the number of
// characters consumed (0 when no tag was added so the caller can skip the
// advance).
function tryAddHashtag(chars: string[], start: number, seen: Set<string>, result: string[]): number {
  const { tag, consumed } = readTag(chars, start);
  if (tag === '') return 0;
  const normalized = normalizeHashtag(tag);
  if (normalized === '' || seen.has(normalized)) return consumed;
  seen.add(normalized);
  result.push(normalized);
  return consumed;
}

/**
 * Extract the unique set of normalized hashtags from `body`, preserving
 * first-occurrence order.
 */
export function extractHashtags(body: string): string[] {
  const result: string[] = [];
  const seen = new Set<string>();

  // Iterate by code points so multi-byte characters aren't split.
  const chars = Array.from(body);
  let prev = ' '; // start-of-string is treated as a boundary
  let inInlineCode = false;
  let inFence = false;

  for (let i = 0; i < chars.length; i++) {
    const ch = chars[i] ?? '';

    const cs = advanceCodeState(chars, i, prev, inFence, inInlineCode);
    if (cs.consumed) {
      i += cs.extraI;
      prev = cs.newPrev;
      inFence = cs.newInFence;
      inInlineCode = cs.newInInlineCode;
      continue;
    }

    if (ch === '\\' && chars[i + 1] === '#') {
      prev = chars[i + 1] ?? '';
      i++;
      continue;
    }

    if (ch === '#' && isTagBoundary(prev)) {
      const advance = tryAddHashtag(chars, i + 1, seen, result);
      if (advance > 0) {
        i += advance;
        prev = chars[i] ?? '';
        continue;
      }
    }

    prev = ch;
  }

  return result;
}

function atFenceMarker(chars: string[], i: number, prev: string): boolean {
  if (i + FENCE_MARKER.length > chars.length) return false;
  if (chars.slice(i, i + FENCE_MARKER.length).join('') !== FENCE_MARKER) return false;
  if (i === 0) return true;
  return prev === '\n';
}

function readTag(chars: string[], start: number): { tag: string; consumed: number } {
  let end = start;
  while (end < chars.length && isTagChar(chars[end] ?? '')) {
    end++;
  }
  if (end === start) return { tag: '', consumed: 0 };
  return { tag: chars.slice(start, end).join(''), consumed: end - start };
}

/**
 * Normalize a single raw tag (without the leading `#`) to its canonical form.
 */
export function normalizeHashtag(tag: string): string {
  if (tag === '') return '';
  const folded = tag.normalize('NFKC').toLocaleLowerCase();
  let cleaned = '';
  for (const ch of folded) {
    if (isTagChar(ch)) cleaned += ch;
  }
  const cleanedChars = Array.from(cleaned);
  if (cleanedChars.length > MAX_TAG_LEN) {
    return cleanedChars.slice(0, MAX_TAG_LEN).join('');
  }
  return cleaned;
}
