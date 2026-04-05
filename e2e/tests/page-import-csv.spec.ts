import { test, expect, APIRequestContext } from '@playwright/test';

// Timeouts
const IMPORT_COMPLETION_TIMEOUT_MS = 15000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const MENU_APPEAR_TIMEOUT_MS = 5000;

// Helper: Call PageImportService via the Connect protocol (JSON over HTTP)
async function callPageImportAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.PageImportService/${method}`, {
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    data: body,
  });
}

// Helper: Call PageManagementService via the Connect protocol (JSON over HTTP)
async function callPageManagementAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.PageManagementService/${method}`, {
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    data: body,
  });
}

test.describe('CSV Page Import', () => {
  test.setTimeout(60000);

  test.describe('ParseCSVPreview API', () => {
    test('returns preview data for valid CSV with new pages', async ({ request }) => {
      const csvContent = [
        'identifier,title',
        'e2e_preview_page_a,Preview Page A',
        'e2e_preview_page_b,Preview Page B',
      ].join('\n');

      const resp = await callPageImportAPI(request, 'ParseCSVPreview', { csvContent });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        totalRecords: number;
        errorCount: number;
        createCount: number;
        updateCount: number;
        records: Array<{ identifier: string }>;
        parsingErrors: string[];
      };
      expect(body.totalRecords).toBe(2);
      expect(body.errorCount).toBe(0);
      expect(body.createCount).toBe(2);
      expect(body.updateCount).toBe(0);
      expect(body.records).toHaveLength(2);
      expect(body.parsingErrors ?? []).toHaveLength(0);
    });

    test('returns invalid_argument error for empty CSV content', async ({ request }) => {
      const resp = await callPageImportAPI(request, 'ParseCSVPreview', { csvContent: '' });

      expect(resp.status()).toBe(400);
      const body = await resp.json() as { code: string; message: string };
      expect(body.code).toBe('invalid_argument');
      expect(body.message).toContain('csv_content cannot be empty');
    });

    test('returns parsing errors when identifier column is missing', async ({ request }) => {
      const csvContent = ['title,description', 'My Page,A description'].join('\n');

      const resp = await callPageImportAPI(request, 'ParseCSVPreview', { csvContent });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        parsingErrors: string[];
        totalRecords: number;
      };
      expect(body.parsingErrors.length).toBeGreaterThan(0);
      expect(body.totalRecords).toBe(0);
    });

    test('returns zero records for header-only CSV', async ({ request }) => {
      const csvContent = 'identifier,title';

      const resp = await callPageImportAPI(request, 'ParseCSVPreview', { csvContent });

      expect(resp.status()).toBe(200);
      const body = await resp.json() as {
        totalRecords: number;
        records: Array<unknown>;
        parsingErrors: string[];
      };
      expect(body.totalRecords).toBe(0);
      expect(body.records ?? []).toHaveLength(0);
    });
  });

  test.describe('StartPageImportJob API', () => {
    test('starts import job and creates pages from valid CSV', async ({ request }) => {
      const pageName = 'e2e_import_job_created_page';
      const csvContent = ['identifier,title', `${pageName},Import Job Test Page`].join('\n');

      const importResp = await callPageImportAPI(request, 'StartPageImportJob', { csvContent });

      expect(importResp.status()).toBe(200);
      const importBody = await importResp.json() as {
        success: boolean;
        jobId: string;
        recordCount: number;
        error: string;
      };
      expect(importBody.success).toBe(true);
      expect(importBody.recordCount).toBe(1);
      expect(importBody.jobId).toBeTruthy();

      // Poll until the imported page exists (jobs run asynchronously in the background)
      await expect
        .poll(
          async () => {
            const resp = await callPageManagementAPI(request, 'ReadPage', { pageName });
            return resp.ok();
          },
          { timeout: IMPORT_COMPLETION_TIMEOUT_MS },
        )
        .toBe(true);
    });

    test('creates multiple pages from a multi-row CSV', async ({ request }) => {
      const pages = [
        { id: 'e2e_import_multi_page_one', title: 'Multi Import Page One' },
        { id: 'e2e_import_multi_page_two', title: 'Multi Import Page Two' },
        { id: 'e2e_import_multi_page_three', title: 'Multi Import Page Three' },
      ];
      const csvContent = [
        'identifier,title',
        ...pages.map((p) => `${p.id},${p.title}`),
      ].join('\n');

      const importResp = await callPageImportAPI(request, 'StartPageImportJob', { csvContent });

      expect(importResp.status()).toBe(200);
      const importBody = await importResp.json() as { success: boolean; recordCount: number };
      expect(importBody.success).toBe(true);
      expect(importBody.recordCount).toBe(3);

      // Poll until all pages exist
      for (const page of pages) {
        await expect
          .poll(
            async () => {
              const resp = await callPageManagementAPI(request, 'ReadPage', {
                pageName: page.id,
              });
              return resp.ok();
            },
            { timeout: IMPORT_COMPLETION_TIMEOUT_MS },
          )
          .toBe(true);
      }
    });

    test('returns invalid_argument error for empty CSV content', async ({ request }) => {
      const resp = await callPageImportAPI(request, 'StartPageImportJob', { csvContent: '' });

      expect(resp.status()).toBe(400);
      const body = await resp.json() as { code: string };
      expect(body.code).toBe('invalid_argument');
    });

    test('returns invalid_argument error for CSV missing identifier column', async ({ request }) => {
      const csvContent = ['title,description', 'No Identifier,Column here'].join('\n');

      const resp = await callPageImportAPI(request, 'StartPageImportJob', { csvContent });

      expect(resp.status()).toBe(400);
      const body = await resp.json() as { code: string };
      expect(body.code).toBe('invalid_argument');
    });
  });

  test.describe('Import dialog UI workflow', () => {
    test('shows CSV preview after uploading a valid CSV file', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Open the dialog via the tools menu dropdown
      await expect(page.locator('#page-import-trigger')).toBeAttached({
        timeout: MENU_APPEAR_TIMEOUT_MS,
      });
      await page.locator('.tools-menu').hover();
      await page.locator('#page-import-trigger').click();

      const dialog = page.locator('page-import-dialog');

      // Upload a valid CSV file via the hidden file input
      const csvContent = 'identifier,title\ne2e_ui_preview_page,UI Preview Test';
      await dialog.locator('.file-input').setInputFiles({
        name: 'test-import.csv',
        mimeType: 'text/csv',
        buffer: Buffer.from(csvContent),
      });

      // Verify the preview state: summary bar shows record counts
      await expect(dialog.locator('.summary-bar')).toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });
      await expect(dialog.locator('.summary-bar')).toContainText('1 total');
      await expect(dialog.locator('.summary-bar')).toContainText('1 new');

      // Verify a record panel is shown
      await expect(dialog.locator('.record-panel')).toBeVisible();
    });

    test('transitions to importing state after clicking import button', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await expect(page.locator('#page-import-trigger')).toBeAttached({
        timeout: MENU_APPEAR_TIMEOUT_MS,
      });
      await page.locator('.tools-menu').hover();
      await page.locator('#page-import-trigger').click();

      const dialog = page.locator('page-import-dialog');

      // Upload a valid CSV file
      const csvContent = 'identifier,title\ne2e_ui_import_page,UI Import Test';
      await dialog.locator('.file-input').setInputFiles({
        name: 'test-import.csv',
        mimeType: 'text/csv',
        buffer: Buffer.from(csvContent),
      });

      // Wait for the preview state to appear
      await expect(dialog.locator('.summary-bar')).toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });

      // Click the Import button (text matches "Import N page(s)")
      const importButton = dialog.locator('button').filter({ hasText: /^Import/ });
      await expect(importButton).toBeEnabled();
      await importButton.click();

      // Verify the importing state is shown with the report link
      await expect(dialog.locator('.importing-container')).toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });
      await expect(dialog.locator('a.report-link')).toBeVisible();
    });

    test('shows error display when uploading a non-CSV file', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await expect(page.locator('#page-import-trigger')).toBeAttached({
        timeout: MENU_APPEAR_TIMEOUT_MS,
      });
      await page.locator('.tools-menu').hover();
      await page.locator('#page-import-trigger').click();

      const dialog = page.locator('page-import-dialog');

      // Upload a plain text file (not a CSV)
      await dialog.locator('.file-input').setInputFiles({
        name: 'not-a-csv.txt',
        mimeType: 'text/plain',
        buffer: Buffer.from('this is not a csv file'),
      });

      // Verify the error display component appears
      await expect(dialog.locator('error-display')).toBeVisible({
        timeout: COMPONENT_LOAD_TIMEOUT_MS,
      });
    });

    test('closes dialog on Escape key press', async ({ page }) => {
      await page.goto('/home/view');
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await expect(page.locator('#page-import-trigger')).toBeAttached({
        timeout: MENU_APPEAR_TIMEOUT_MS,
      });
      await page.locator('.tools-menu').hover();
      await page.locator('#page-import-trigger').click();

      const dialog = page.locator('page-import-dialog');

      // Dialog drop-zone should be visible while open
      await expect(dialog.locator('.drop-zone')).toBeVisible({ timeout: MENU_APPEAR_TIMEOUT_MS });

      // Press Escape to close the dialog
      await page.keyboard.press('Escape');

      // The drop-zone should no longer be visible after closing
      await expect(dialog.locator('.drop-zone')).not.toBeVisible();
    });
  });
});
