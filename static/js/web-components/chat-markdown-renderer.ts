import { createClient, type Client } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { create } from '@bufbuild/protobuf';
import {
  PageManagementService,
  RenderMarkdownRequestSchema,
} from '../gen/api/v1/page_management_pb.js';

/**
 * Service that renders markdown content to HTML via the RenderMarkdown gRPC RPC.
 * Caches results by content string to avoid redundant server calls.
 */
export class ChatMarkdownRenderer {
  private cache = new Map<string, string>();
  private client: Client<typeof PageManagementService>;

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
    const html = response.renderedHtml;
    this.cache.set(cacheKey, html);
    return html;
  }
}
