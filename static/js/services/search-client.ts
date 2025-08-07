import { createPromiseClient, PromiseClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';
import { SearchService } from '../gen/api/v1/search_connect.js';
import type { SearchContentRequest, SearchResult } from '../gen/api/v1/search_pb.js';

/**
 * SearchClient provides methods for searching wiki content via gRPC-Web.
 */
export class SearchClient {
  private client: PromiseClient<typeof SearchService> | null = null;
  private baseUrl: string;

  constructor(baseUrl: string = '') {
    this.baseUrl = baseUrl;
    // Delay client creation until first use to allow for testing
  }

  private getClient(): PromiseClient<typeof SearchService> {
    if (!this.client) {
      const transport = createGrpcWebTransport({
        baseUrl: this.baseUrl || window.location.origin,
      });
      this.client = createPromiseClient(SearchService, transport);
    }
    return this.client;
  }

  /**
   * Search for content matching the query.
   * @param query The search query string
   * @returns Promise resolving to search results
   */
  async search(query: string): Promise<SearchResult[]> {
    const request: Partial<SearchContentRequest> = { query };
    const response = await this.getClient().searchContent(request);
    
    // Return results directly - no HTML generation needed
    return response.results;
  }

}

// Export a singleton instance for convenience
export const searchClient = new SearchClient();