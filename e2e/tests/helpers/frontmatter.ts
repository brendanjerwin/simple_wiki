/**
 * Builds a RegExp that matches a TOML frontmatter key=value assignment
 * regardless of whether the server serialised the value with single or
 * double quotes (e.g. `key = "value"` or `key = 'value'`).
 */
export function frontMatterStringMatcher(key: string, value: string): RegExp {
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const escapedValue = value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`${escapedKey}\\s*=\\s*['"]${escapedValue}['"]`);
}
