/**
 * Result of parsing a wiki URL - discriminated union type
 */
export type WikiUrlParseResult =
  | { success: true; pageIdentifier: string }
  | { success: false; error: string };

/**
 * WikiUrlParser - Extracts page identifiers from wiki URLs
 *
 * Handles various URL formats:
 * - Full URLs: https://wiki.example.com/my_page/view
 * - Absolute paths: /my_page/view
 * - Path only: /my_page
 * - Identifier only: my_page
 */
export class WikiUrlParser {
  /**
   * Parses a URL or path and extracts the wiki page identifier
   * @param urlOrPath - The URL, path, or identifier to parse
   * @returns Parse result with page identifier or error
   */
  static parse(urlOrPath: string): WikiUrlParseResult {
    const trimmed = urlOrPath.trim();

    if (!trimmed) {
      return { success: false, error: 'URL or path cannot be empty' };
    }

    let pathPart: string;

    // Try to parse as full URL first
    if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
      try {
        const url = new URL(trimmed);
        pathPart = url.pathname;
      } catch {
        return { success: false, error: 'Invalid URL format' };
      }
    } else {
      // Treat as path or identifier
      pathPart = trimmed;
    }

    // Remove query parameters if present
    const queryIndex = pathPart.indexOf('?');
    if (queryIndex !== -1) {
      pathPart = pathPart.substring(0, queryIndex);
    }

    // Split path by slashes and filter empty segments
    const segments = pathPart.split('/').filter(s => s.length > 0);

    // First non-empty segment is the page identifier
    const pageIdentifier = segments[0];

    if (!pageIdentifier) {
      return { success: false, error: 'No page identifier found in path' };
    }

    if (!this.isValidPageIdentifier(pageIdentifier)) {
      return { success: false, error: `Invalid page identifier: ${pageIdentifier}` };
    }

    return { success: true, pageIdentifier };
  }

  /**
   * Validates that a string looks like a valid page identifier
   * Page identifiers can contain Unicode letters (any language), numbers, underscores, and hyphens
   * Must start with a Unicode letter or underscore
   * @param identifier - The identifier to validate
   * @returns true if valid page identifier format
   */
  static isValidPageIdentifier(identifier: string): boolean {
    // Uses Unicode property escapes (ES2018+)
    // \p{L} = Unicode Letter category (any language: Latin, Japanese, Chinese, Arabic, etc.)
    // \p{N} = Unicode Number category (any numeral system)
    // 'u' flag required for Unicode property escapes
    const validPattern = /^[\p{L}_][\p{L}\p{N}_-]*$/u;
    return validPattern.test(identifier);
  }
}
