import type { JsonObject } from '@bufbuild/protobuf';

/**
 * Flattens frontmatter (protobuf Struct) into dot-notation key-value pairs
 * for display in the page import preview.
 *
 * Skips null, undefined, and array values (arrays are handled separately by ArrayOperation).
 *
 * @example
 * flattenFrontmatter({inventory: {container: "drawer"}})
 * // Returns: [["inventory.container", "drawer"]]
 */
export function flattenFrontmatter(
  obj: JsonObject,
  prefix = ''
): [string, string][] {
  const result: [string, string][] = [];

  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;

    // Skip null, undefined, and arrays
    if (value === null || value === undefined || Array.isArray(value)) {
      continue;
    }

    if (typeof value === 'object') {
      // Recurse into nested objects
      result.push(...flattenFrontmatter(value as JsonObject, fullKey));
    } else {
      result.push([fullKey, String(value)]);
    }
  }

  return result;
}
