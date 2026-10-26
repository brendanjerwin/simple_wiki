/**
 * E2E tests for Dialog and Toast page-app components (issue #1320, Slice 3/4).
 *
 * Dialog(title?, open, children, onClose?) renders a modal overlay controlled
 * by a boolean $variable. Toast(message, visible, kind?, onDismiss?) renders
 * a transient notification fixed at the bottom-right of the viewport.
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_dialog_toast';

const PAGE_LOAD_TIMEOUT_MS = 15000;

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('Dialog component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Dialog', () => {
  test('does not render overlay when open=false', async ({ page, request }) => {
   const program = [
    '$open = false',
    'content = Text("Dialog body")',
    'dlg = Dialog("My Dialog", $open, [content], @Set($open, false))',
    'root = Card("App", Stack([Button("Open", @Set($open, true)), dlg]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Dialog backdrop should NOT be in the DOM when closed
   await expect(host.locator('.wiki-page-app-dialog-backdrop')).toHaveCount(0);
  });

  test('renders the modal when the open button is clicked', async ({ page, request }) => {
   const program = [
    '$open = false',
    'content = Text("Are you sure you want to reset?")',
    'dlg = Dialog("Confirm Reset", $open, [content], @Set($open, false))',
    'root = Card("App", Stack([Button("Reset", @Set($open, true), "danger"), dlg]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Click the button to open the dialog
   await host.locator('.wiki-page-app-button', { hasText: 'Reset' }).click();

   // The dialog overlay and title render
   await expect(host.locator('.wiki-page-app-dialog-backdrop')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
   await expect(
    host.locator('.wiki-page-app-dialog-title', { hasText: 'Confirm Reset' }),
   ).toBeVisible();
   await expect(host.locator('.wiki-page-app-dialog-body')).toContainText('Are you sure you want to reset?');
  });

  test('closes the dialog when the × close button is clicked', async ({ page, request }) => {
   const program = [
    '$open = false',
    'content = Text("Dialog content")',
    'dlg = Dialog("Closeable", $open, [content], @Set($open, false))',
    'root = Card("App", Stack([Button("Open", @Set($open, true)), dlg]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Open the dialog
   await host.locator('.wiki-page-app-button', { hasText: 'Open' }).click();
   await expect(host.locator('.wiki-page-app-dialog-backdrop')).toBeVisible({
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });

   // Click the × button to close
   await host.locator('.wiki-page-app-dialog-close').click();

   // Dialog should be gone
   await expect(host.locator('.wiki-page-app-dialog-backdrop')).toHaveCount(0, {
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });
});

test.describe.serial('Toast component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when a page app contains Toast', () => {
  test('does not render the toast when visible=false', async ({ page, request }) => {
   const program = [
    '$visible = false',
    'toast = Toast("Saved!", $visible, "success", @Set($visible, false))',
    'root = Card("App", Stack([Button("Save", @Set($visible, true)), toast]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Toast should not be visible
   await expect(host.locator('.wiki-page-app-toast')).toHaveCount(0);
  });

  test('shows the toast message when triggered', async ({ page, request }) => {
   const program = [
    '$visible = false',
    'toast = Toast("Session saved!", $visible, "success", @Set($visible, false))',
    'root = Card("App", Stack([Button("Save", @Set($visible, true)), toast]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Click the button to show the toast
   await host.locator('.wiki-page-app-button', { hasText: 'Save' }).click();

   // Toast renders with the message
   await expect(host.locator('.wiki-page-app-toast')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-toast-message', { hasText: 'Session saved!' }),
   ).toBeVisible();
   // The success kind should be applied
   await expect(host.locator('.wiki-page-app-toast[data-kind="success"]')).toBeVisible();
  });

  test('dismisses the toast when the × button is clicked', async ({ page, request }) => {
   const program = [
    '$visible = false',
    'toast = Toast("Notification text", $visible, "info", @Set($visible, false))',
    'root = Card("App", Stack([Button("Notify", @Set($visible, true)), toast]))',
   ].join('\n');
   await setupPage(request, TEST_PAGE, pageBody(program));

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'App' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Show the toast
   await host.locator('.wiki-page-app-button', { hasText: 'Notify' }).click();
   await expect(host.locator('.wiki-page-app-toast')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Dismiss it
   await host.locator('.wiki-page-app-toast-dismiss').click();

   // Toast should be gone
   await expect(host.locator('.wiki-page-app-toast')).toHaveCount(0, {
    timeout: PAGE_LOAD_TIMEOUT_MS,
   });
  });
 });
});
