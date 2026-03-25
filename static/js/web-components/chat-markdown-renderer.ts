import { createClient, type Client } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { create } from '@bufbuild/protobuf';
import {
  PageManagementService,
  RenderMarkdownRequestSchema,
} from '../gen/api/v1/page_management_pb.js';

const MAX_CACHE_SIZE = 200;

/**
 * Service that renders markdown content to HTML via the RenderMarkdown gRPC RPC.
 * Caches results by content string to avoid redundant server calls.
 * Cache is bounded to MAX_CACHE_SIZE entries; oldest entry is evicted when full.
 */
export class ChatMarkdownRenderer {
  private readonly cache = new Map<string, string>();
  private readonly client: Client<typeof PageManagementService>;

  constructor(client?: Client<typeof PageManagementService>) {
    this.client = client ?? createClient(PageManagementService, getGrpcWebTransport());
  }

  async renderMarkdown(content: string, page?: string): Promise<string> {
    if (!content) {
      return '';
    }

    const cacheKey = `${page ?? ''}:${content}`;
    const cached = this.cache.get(cacheKey);
    if (cached !== undefined) {
      return cached;
    }

    const request = create(RenderMarkdownRequestSchema, {
      content,
      page: page ?? '',
    });

    const response = await this.client.renderMarkdown(request);
    const renderedHtml = response.renderedHtml;

    if (this.cache.size >= MAX_CACHE_SIZE) {
      // Delete the oldest entry (first key in insertion order)
      const oldestKey = this.cache.keys().next().value;
      if (oldestKey !== undefined) {
        this.cache.delete(oldestKey);
      }
    }

    this.cache.set(cacheKey, renderedHtml);
    return renderedHtml;
  }
}
