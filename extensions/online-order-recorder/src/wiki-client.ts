interface ConnectResponse {
  content_markdown?: string;
  version_hash?: string;
  [key: string]: unknown;
}

export async function callConnectRPC(
  wikiUrl: string,
  service: string,
  method: string,
  payload: Record<string, unknown>
): Promise<ConnectResponse> {
  const url = `${wikiUrl.replace(/\/+$/, '')}/${service}/${method}`;

  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Connect-Protocol-Version': '1',
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Connect RPC ${method} failed (${response.status}): ${body}`);
  }

  return response.json() as Promise<ConnectResponse>;
}

export async function readPage(
  wikiUrl: string,
  pageName: string
): Promise<{ contentMarkdown: string; versionHash: string }> {
  const resp = await callConnectRPC(
    wikiUrl,
    'api.v1.PageManagementService',
    'ReadPage',
    { page: pageName }
  );
  return {
    contentMarkdown: resp['content_markdown'] as string ?? '',
    versionHash: resp['version_hash'] as string ?? '',
  };
}

export async function updatePageContent(
  wikiUrl: string,
  pageName: string,
  newContentMarkdown: string,
  expectedVersionHash: string
): Promise<void> {
  await callConnectRPC(
    wikiUrl,
    'api.v1.PageManagementService',
    'UpdatePageContent',
    {
      page: pageName,
      new_content_markdown: newContentMarkdown,
      expected_version_hash: expectedVersionHash,
    }
  );
}

export async function createPage(
  wikiUrl: string,
  pageName: string,
  contentMarkdown: string
): Promise<void> {
  await callConnectRPC(
    wikiUrl,
    'api.v1.PageManagementService',
    'CreatePage',
    {
      page: pageName,
      content_markdown: contentMarkdown,
    }
  );
}
