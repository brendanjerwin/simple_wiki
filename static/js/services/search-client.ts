import { createClient, type Client } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';
import { create } from '@bufbuild/protobuf';
import { SearchService, SearchContentRequestSchema } from '../gen/api/v1/search_pb.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';

/**
 * SearchClient provides methods for searching wiki content via gRPC-Web.
 */
export class SearchClient {
  private client: Client<typeof SearchService> | null = null;
  private baseUrl: string;

  constructor(baseUrl: string = '') {
    this.baseUrl = baseUrl;
    // Delay client creation until first use to allow for testing
  }

  private getClient(): Client<typeof SearchService> {
    if (!this.client) {
      const transport = createGrpcWebTransport({
        baseUrl: this.baseUrl || window.location.origin,
      });
      this.client = createClient(SearchService, transport);
    }
    return this.client;
  }

  /**
   * Search for content matching the query.
   * @param query The search query string
   * @returns Promise resolving to search results
   */
  async search(query: string): Promise<SearchResult[]> {
    const request = create(SearchContentRequestSchema, { query });
    const response = await this.getClient().searchContent(request);

    // Return results directly - no HTML generation needed
    return response.results;
  }

}

// Export a singleton instance for convenience
export const searchClient = new SearchClient();