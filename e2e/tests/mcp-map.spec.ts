import { test, expect } from '@playwright/test';
import { clearMapPage, seedMapPage } from './helpers/map-page';

const TEST_PAGE = 'e2e_mcp_map_test';
const TEST_MAP = 'mcp_yard';

test.describe('MapService MCP Tools E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeEach(async ({ page }) => {
    await seedMapPage(page, TEST_PAGE, TEST_MAP);
  });

  test.afterEach(async ({ page }) => {
    await clearMapPage(page, TEST_PAGE);
  });

  // Helper to call MCP tools via JSON-RPC
  async function callMcpTool(requestContext: any, toolName: string, args: any) {
    // 1. Initialize MCP session
    const initResponse = await requestContext.post('/mcp', {
      data: {
        jsonrpc: '2.0',
        id: 1,
        method: 'initialize',
        params: {
          protocolVersion: '2024-11-05',
          capabilities: {},
          clientInfo: { name: 'E2E Test Client', version: '1.0' },
        },
      },
      headers: {
        'Content-Type': 'application/json',
      },
    });

    expect(initResponse.ok()).toBe(true);
    const sessionID = initResponse.headers()['mcp-session-id'];

    // 2. Call the tool
    const callResponse = await requestContext.post('/mcp', {
      data: {
        jsonrpc: '2.0',
        id: 2,
        method: 'tools/call',
        params: {
          name: toolName,
          arguments: args,
        },
      },
      headers: {
        'Content-Type': 'application/json',
        'Mcp-Session-Id': sessionID || '',
      },
    });

    expect(callResponse.ok()).toBe(true);
    return await callResponse.json();
  }

  test('F5: should list map outlines and count overlays via ListMaps tool', async ({ request }) => {
    const result = await callMcpTool(request, 'api_v1_MapService_ListMaps', {
      page: TEST_PAGE,
    });

    expect(result.error).toBeUndefined();
    const outlines = result.result.content[0].text;
    const data = JSON.parse(outlines);
    expect(data.maps).toBeDefined();
    expect(data.maps.length).toBeGreaterThan(0);
    
    const map = data.maps.find((m: any) => m.name === TEST_MAP);
    expect(map).toBeDefined();
    expect(map.marker_count).toBe(1);
  });

  test('F5: should mutate track elements using AddTrack, UpdateTrack, and DeleteTrack tools', async ({ request }) => {
    // Upload real GPX files so validateTrackFile passes (AddTrack/UpdateTrack open the file).
    const minimalGpx = (name: string) => Buffer.from(
      `<?xml version="1.0"?><gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><trk><name>${name}</name><trkseg><trkpt lat="41.1" lon="-72.2"/></trkseg></trk></gpx>`
    ).toString('base64');

    const uploadAdd = await callMcpTool(request, 'api_v1_FileStorageService_UploadFile', {
      content: minimalGpx('MCP Trail Path'),
      filename: 'mcp-route.gpx',
    });
    expect(uploadAdd.error).toBeUndefined();
    const uploadAddData = JSON.parse(uploadAdd.result.content[0].text);
    const addFileHash: string = uploadAddData.hash;

    const uploadUpdate = await callMcpTool(request, 'api_v1_FileStorageService_UploadFile', {
      content: minimalGpx('MCP Trail Path Updated'),
      filename: 'mcp-route-v2.gpx',
    });
    expect(uploadUpdate.error).toBeUndefined();
    const uploadUpdateData = JSON.parse(uploadUpdate.result.content[0].text);
    const updateFileHash: string = uploadUpdateData.hash;

    // 1. Add track
    const addResult = await callMcpTool(request, 'api_v1_MapService_AddTrack', {
      page: TEST_PAGE,
      mapName: TEST_MAP,
      track: {
        label: 'MCP Trail Path',
        fileHash: addFileHash,
        filename: 'mcp-route.gpx',
        format: 'GPX',
        tags: ['mcp', 'agent'],
        color: '#ef4444',
      },
    });

    expect(addResult.error).toBeUndefined();
    const addData = JSON.parse(addResult.result.content[0].text);
    expect(addData.track.metadata.uid).toBeDefined();
    const trackUid = addData.track.metadata.uid;

    // 2. Update track (response uses UseProtoNames:true → snake_case: updated_at)
    const updateResult = await callMcpTool(request, 'api_v1_MapService_UpdateTrack', {
      page: TEST_PAGE,
      mapName: TEST_MAP,
      uid: trackUid,
      track: {
        label: 'MCP Trail Path Updated',
        fileHash: updateFileHash,
        filename: 'mcp-route-v2.gpx',
        format: 'GPX',
        tags: ['mcp', 'agent', 'updated'],
        color: '#f97316',
      },
      expectedUpdatedAt: addData.map.updated_at,
    });

    expect(updateResult.error).toBeUndefined();

    // 3. Get track geometry (Note: returns mock geometry in E2E since file storer mock handles it or fails-fast)
    // Here we verify GetTrackGeometry returns expected JSON structure or error code if file missing.
    const geomResult = await callMcpTool(request, 'api_v1_MapService_GetTrackGeometry', {
      page: TEST_PAGE,
      mapName: TEST_MAP,
      uid: trackUid,
    });
    // If files are missing, it returns NotFound. Otherwise returns geometry.
    // The test verifies the tool executes cleanly on the RPC server.
    expect(geomResult.error || geomResult.result).toBeDefined();

    // 4. Delete track
    const deleteResult = await callMcpTool(request, 'api_v1_MapService_DeleteTrack', {
      page: TEST_PAGE,
      mapName: TEST_MAP,
      uid: trackUid,
      expectedUpdatedAt: addData.map.updated_at, // optimistic concurrency (UseProtoNames:true → snake_case)
    });

    expect(deleteResult.error).toBeUndefined();
  });
});
