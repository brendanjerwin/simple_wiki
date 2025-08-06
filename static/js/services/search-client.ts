import { createPromiseClient, PromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { SearchService } from '../gen/api/v1/search_connect.js';
import type { SearchContentRequest, SearchContentResponse, SearchResult, HighlightSpan } from '../gen/api/v1/search_pb.js';

export interface SearchResultWithHTML extends SearchResult {
  fragmentHTML?: string;
}

/**
 * SearchClient provides methods for searching wiki content via gRPC-Web.
 */
export class SearchClient {
  private client: PromiseClient<typeof SearchService>;

  constructor(baseUrl: string = '') {
    const transport = createConnectTransport({
      baseUrl: baseUrl || window.location.origin,
    });
    this.client = createPromiseClient(SearchService, transport);
  }

  /**
   * Search for content matching the query.
   * @param query The search query string
   * @returns Promise resolving to search results
   */
  async search(query: string): Promise<SearchResultWithHTML[]> {
    const request: Partial<SearchContentRequest> = { query };
    const response = await this.client.searchContent(request);
    
    // Convert results to include HTML fragments with highlights
    return response.results.map(result => this.convertToHTMLResult(result));
  }

  /**
   * Convert a gRPC SearchResult to include an HTML fragment with highlights.
   * @param result The gRPC SearchResult
   * @returns SearchResult with added fragmentHTML field
   */
  private convertToHTMLResult(result: SearchResult): SearchResultWithHTML {
    const htmlResult = result as SearchResultWithHTML;
    
    if (result.fragment && result.highlights.length > 0) {
      htmlResult.fragmentHTML = this.buildHTMLFragment(result.fragment, result.highlights);
    } else if (result.fragment) {
      // No highlights, just escape the fragment
      htmlResult.fragmentHTML = this.escapeHTML(result.fragment);
    }
    
    return htmlResult;
  }

  /**
   * Build an HTML fragment with <mark> tags for highlights.
   * @param text The plain text fragment
   * @param highlights Array of highlight spans
   * @returns HTML string with marked highlights
   */
  private buildHTMLFragment(text: string, highlights: HighlightSpan[]): string {
    // Sort highlights by start position
    const sortedHighlights = [...highlights].sort((a, b) => a.start - b.start);
    
    let html = '';
    let lastEnd = 0;
    
    for (const highlight of sortedHighlights) {
      // Add text before the highlight
      if (highlight.start > lastEnd) {
        html += this.escapeHTML(text.substring(lastEnd, highlight.start));
      }
      
      // Add the highlighted text
      html += '<mark>' + this.escapeHTML(text.substring(highlight.start, highlight.end)) + '</mark>';
      lastEnd = highlight.end;
    }
    
    // Add any remaining text after the last highlight
    if (lastEnd < text.length) {
      html += this.escapeHTML(text.substring(lastEnd));
    }
    
    // Replace newlines with <br> tags for display
    return html.replace(/\n/g, '<br>');
  }

  /**
   * Escape HTML special characters to prevent XSS.
   * @param text The text to escape
   * @returns Escaped HTML string
   */
  private escapeHTML(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
}

// Export a singleton instance for convenience
export const searchClient = new SearchClient();