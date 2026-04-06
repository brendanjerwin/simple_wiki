import { createClient, type Client } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';
import { create } from '@bufbuild/protobuf';
import {
  PageManagementService,
  ReadPageRequestSchema,
  CreatePageRequestSchema,
  UpdatePageContentRequestSchema,
} from '../../../static/js/gen/api/v1/page_management_pb.js';

let cachedClient: { url: string; client: Client<typeof PageManagementService> } | null = null;

function getWikiClient(wikiUrl: string): Client<typeof PageManagementService> {
  const url = wikiUrl.replace(/\/+$/, ''); // NOSONAR - false positive: literal token + quantifier + anchor is immune to ReDoS
  if (cachedClient?.url === url) {
    return cachedClient.client;
  }
  const transport = createGrpcWebTransport({ baseUrl: url });
  const client = createClient(PageManagementService, transport);
  cachedClient = { url, client };
  return client;
}

export async function readPage(
  wikiUrl: string,
  pageName: string
): Promise<{ contentMarkdown: string; versionHash: string }> {
  const client = getWikiClient(wikiUrl);
  const request = create(ReadPageRequestSchema, { pageName });
  const response = await client.readPage(request);
  return {
    contentMarkdown: response.contentMarkdown,
    versionHash: response.versionHash,
  };
}

export async function updatePageContent(
  wikiUrl: string,
  pageName: string,
  newContentMarkdown: string,
  expectedVersionHash: string
): Promise<void> {
  const client = getWikiClient(wikiUrl);
  const request = create(UpdatePageContentRequestSchema, {
    pageName,
    newContentMarkdown,
    expectedVersionHash,
  });
  await client.updatePageContent(request);
}

export async function createPage(
  wikiUrl: string,
  pageName: string,
  contentMarkdown: string
): Promise<void> {
  const client = getWikiClient(wikiUrl);
  const request = create(CreatePageRequestSchema, {
    pageName,
    contentMarkdown,
  });
  await client.createPage(request);
}
