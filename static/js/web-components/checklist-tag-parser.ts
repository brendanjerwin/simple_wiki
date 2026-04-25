import { extractHashtags } from '../utils/tag-parser.js';

export interface ChecklistItem {
  text: string;
  checked: boolean;
  tags: string[];
}

export interface ChecklistData {
  items: ChecklistItem[];
}

/**
 * Parse all #tag tokens from the input string. Tags are normalized
 * (NFKC + lowercase, hyphens/underscores preserved) and #tag tokens are
 * removed from the returned text.
 *
 * Examples:
 *   "milk #dairy #fridge"  -> { tags: ["dairy", "fridge"], text: "milk" }
 *   "#dairy milk #fridge"  -> { tags: ["dairy", "fridge"], text: "milk" }
 *   "buy #dairy milk"      -> { tags: ["dairy"], text: "buy milk" }
 *   "just milk"            -> { tags: [], text: "just milk" }
 *
 * The text-stripping logic mirrors the extractor's grammar so that:
 *   - `\#tag` is unescaped (the leading backslash is consumed) but the
 *     `#tag` is preserved in the text.
 *   - `#tag` inside an inline code span (`...`) is preserved verbatim.
 */
export function parseTaggedInput(input: string): { tags: string[]; text: string } {
  const tags = extractHashtags(input);
  const text = stripExtractedTags(input)
    .replace(/\s+/g, ' ')
    .trim();

  return { tags, text };
}

const TAG_TEXT_CHAR_RE = /[\p{L}\p{N}\-_]/u;

/**
 * Strip the `#tag` tokens that the extractor would have extracted,
 * leaving everything else (including escaped `#tag` and `#tag` inside
 * inline code spans) intact.
 */
function stripExtractedTags(input: string): string {
  const chars = Array.from(input);
  let out = '';
  let prev = ' '; // start-of-string is treated as a boundary
  let inInlineCode = false;

  for (let i = 0; i < chars.length; i++) {
    const ch = chars[i] ?? '';

    // `\#` escape: drop the backslash, keep the `#` literally and continue.
    if (!inInlineCode && ch === '\\' && chars[i + 1] === '#') {
      out += '#';
      prev = '#';
      i++;
      continue;
    }

    if (ch === '`') {
      inInlineCode = !inInlineCode;
      out += ch;
      prev = ch;
      continue;
    }

    if (inInlineCode) {
      out += ch;
      prev = ch;
      continue;
    }

    if (ch === '#' && isStripBoundary(prev, out)) {
      const consumed = consumeTagChars(chars, i + 1);
      if (consumed > 0) {
        // Drop the trailing space we may have just emitted so we don't
        // leave a stray double space behind.
        i += consumed;
        prev = chars[i] ?? '';
        continue;
      }
    }

    out += ch;
    prev = ch;
  }

  return out;
}

function isStripBoundary(prev: string, accumulated: string): boolean {
  if (accumulated === '') return true;
  return /\s/u.test(prev);
}

function consumeTagChars(chars: string[], start: number): number {
  let end = start;
  while (end < chars.length && TAG_TEXT_CHAR_RE.test(chars[end] ?? '')) {
    end++;
  }
  return end - start;
}

/**
 * Compose structured item data into the editable `#tag` text format.
 * e.g. { text: "milk", tags: ["dairy", "fridge"] } -> "milk #dairy #fridge"
 */
export function composeTaggedText(item: ChecklistItem): string {
  if (item.tags.length === 0) return item.text;
  return item.text + item.tags.map(t => ` #${t}`).join('');
}
