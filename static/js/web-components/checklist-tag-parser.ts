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
 */
export function parseTaggedInput(input: string): { tags: string[]; text: string } {
  const tags = extractHashtags(input);

  // Strip every `#tag` token from the text, preserving everything else.
  // Match `#` only when preceded by start-of-string or whitespace, then
  // consume tag chars (letters, digits, hyphen, underscore — no Unicode
  // letter classes here because we just need to remove the spelled token,
  // not normalize it).
  const text = input
    .replace(/(^|\s)#[\p{L}\p{N}\-_]+/gu, '$1')
    .replace(/\s+/g, ' ')
    .trim();

  return { tags, text };
}

/**
 * Compose structured item data into the editable `#tag` text format.
 * e.g. { text: "milk", tags: ["dairy", "fridge"] } -> "milk #dairy #fridge"
 */
export function composeTaggedText(item: ChecklistItem): string {
  if (item.tags.length === 0) return item.text;
  return item.text + item.tags.map(t => ` #${t}`).join('');
}
