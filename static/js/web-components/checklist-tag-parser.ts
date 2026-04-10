export interface ChecklistItem {
  text: string;
  checked: boolean;
  tags: string[];
}

export interface ChecklistData {
  items: ChecklistItem[];
}

/**
 * Parse all :tag tokens from the input string.
 * Tags are lowercased. All :tag tokens removed from text.
 * Examples:
 *   "milk :dairy :fridge"  -> { tags: ["dairy", "fridge"], text: "milk" }
 *   ":dairy milk :fridge"  -> { tags: ["dairy", "fridge"], text: "milk" }
 *   "buy :dairy milk"      -> { tags: ["dairy"], text: "buy milk" }
 *   "just milk"            -> { tags: [], text: "just milk" }
 */
export function parseTaggedInput(input: string): { tags: string[]; text: string } {
  const tags: string[] = [];
  const tagPattern = /:(\S+)/g;
  let match: RegExpExecArray | null;

  while ((match = tagPattern.exec(input)) !== null) {
    const tag = match[1]?.trim().toLowerCase();
    if (tag) {
      tags.push(tag);
    }
  }

  // Remove all :tag tokens from the text
  const text = input.replaceAll(/:(\S+)/g, '').replaceAll(/\s+/g, ' ').trim();

  return { tags, text };
}

/**
 * Compose structured item data into the editable `:tag` text format.
 * e.g. { text: "milk", tags: ["dairy", "fridge"] } -> "milk :dairy :fridge"
 */
export function composeTaggedText(item: ChecklistItem): string {
  if (item.tags.length === 0) return item.text;
  return item.text + item.tags.map(t => ` :${t}`).join('');
}
