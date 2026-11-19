/**
 * E2E tests for the MicButton page-app component (issue #1468).
 *
 * MicButton(label, template?, mode?, disabled?, onResult?, lang?, interim?)
 * renders a voice-input button backed by the browser's Web Speech API.
 *
 * DOM structure (post-#1471 redesign):
 *   <wiki-page-app-mic-button> (custom element, own shadow root)
 *     └── shadow root
 *           ├── <button class="mic-btn"> (or <span> wrapper in unsupported path)
 *           │     └── <span class="mic-circle"><svg>…</svg></span>
 *           ├── <span class="mic-label">label text</span>
 *           └── (unsupported path only)
 *                 <span class="mic-unsupported-label">Voice input not supported</span>
 *
 * The button is nested in two open shadow roots:
 *   wiki-page-app-host (shadow) → wiki-page-app-mic-button (shadow) → button
 * Playwright locators pierce open shadow DOMs by default, so locators on `host`
 * will find elements inside the nested custom element's shadow root.
 *
 * Recording is not driven in these tests (no microphone in headless CI).
 */
import { expect, test } from '@playwright/test';
import { callAPI, setupPage } from './page-app-helpers';

const TEST_PAGE = 'e2e_page_app_micbutton';
const PAGE_LOAD_TIMEOUT_MS = 15000;

const MIC_PROGRAM = 'root = Card("MicButton test", Stack([MicButton("Speak")]))';

function pageBody(program: string): string {
 return `\`\`\`wiki-page-app\n${program}\n\`\`\`\n`;
}

test.describe.serial('MicButton component', () => {
 test.setTimeout(90000);

 test.beforeEach(async ({ request }) => {
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
  await setupPage(request, TEST_PAGE, pageBody(MIC_PROGRAM));
 });

 test.afterAll(async ({ request }) => {
  await callAPI(request, 'PageAppService', 'ClearPageApp', { page: TEST_PAGE });
  await callAPI(request, 'PageManagementService', 'DeletePage', { page: TEST_PAGE });
 });

 test.describe('when speech recognition is supported (idle state)', () => {
  test('renders an accessible enabled button with correct idle attributes', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   // Drill through both shadow roots via evaluate to assert button attributes.
   const btnAttrs = await page.evaluate(() => {
    const hostEl = document.querySelector('wiki-page-app-host');
    const micEl = hostEl?.shadowRoot?.querySelector('wiki-page-app-mic-button');
    const btn = micEl?.shadowRoot?.querySelector('button.mic-btn') as HTMLButtonElement | null;
    if (!btn) return null;
    return {
     disabled: btn.disabled,
     ariaLabel: btn.getAttribute('aria-label'),
     ariaPressed: btn.getAttribute('aria-pressed'),
     dataListening: btn.getAttribute('data-listening'),
    };
   });

   expect(btnAttrs, 'button.mic-btn must be found in nested shadow DOM').not.toBeNull();
   expect(btnAttrs!.disabled).toBe(false);
   expect(btnAttrs!.ariaLabel).toBeTruthy();
   expect(btnAttrs!.ariaPressed).toBe('false');
   expect(btnAttrs!.dataListening).toBe('false');
  });

  test('renders the .mic-circle element inside the button', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const micCircleExists = await page.evaluate(() => {
    const hostEl = document.querySelector('wiki-page-app-host');
    const micEl = hostEl?.shadowRoot?.querySelector('wiki-page-app-mic-button');
    return micEl?.shadowRoot?.querySelector('.mic-circle') != null;
   });

   expect(micCircleExists).toBe(true);
  });

  test('renders the .mic-label caption alongside the button', async ({ page }) => {
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const labelText = await page.evaluate(() => {
    const hostEl = document.querySelector('wiki-page-app-host');
    const micEl = hostEl?.shadowRoot?.querySelector('wiki-page-app-mic-button');
    return micEl?.shadowRoot?.querySelector('span.mic-label')?.textContent?.trim() ?? null;
   });

   expect(labelText).toBe('Speak');
  });

  test('does not emit console errors or unhandled page errors on render', async ({ page }) => {
   const consoleErrors: string[] = [];
   const pageErrors: string[] = [];

   page.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
   });
   page.on('pageerror', (err) => {
    pageErrors.push(err.message);
   });

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   expect(consoleErrors).toHaveLength(0);
   expect(pageErrors).toHaveLength(0);
  });
 });

 test.describe('when speech recognition APIs are absent (graceful degradation)', () => {
  test('renders without throwing or emitting console errors', async ({ page }) => {
   const consoleErrors: string[] = [];
   const pageErrors: string[] = [];

   page.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
   });
   page.on('pageerror', (err) => {
    pageErrors.push(err.message);
   });

   await page.addInitScript(() => {
    // Simulate a browser without Web Speech API (e.g. Firefox).
    delete (window as unknown as Record<string, unknown>)['SpeechRecognition'];
    delete (window as unknown as Record<string, unknown>)['webkitSpeechRecognition'];
   });

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   expect(consoleErrors).toHaveLength(0);
   expect(pageErrors).toHaveLength(0);
  });

  test('renders the button as disabled in the unsupported state', async ({ page }) => {
   await page.addInitScript(() => {
    delete (window as unknown as Record<string, unknown>)['SpeechRecognition'];
    delete (window as unknown as Record<string, unknown>)['webkitSpeechRecognition'];
   });

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const btnAttrs = await page.evaluate(() => {
    const hostEl = document.querySelector('wiki-page-app-host');
    const micEl = hostEl?.shadowRoot?.querySelector('wiki-page-app-mic-button');
    const btn = micEl?.shadowRoot?.querySelector('button.mic-btn') as HTMLButtonElement | null;
    if (!btn) return null;
    return {
     disabled: btn.disabled,
     ariaDisabled: btn.getAttribute('aria-disabled'),
     ariaLabel: btn.getAttribute('aria-label'),
    };
   });

   expect(btnAttrs, 'button.mic-btn must be found in nested shadow DOM').not.toBeNull();
   expect(btnAttrs!.disabled).toBe(true);
   expect(btnAttrs!.ariaDisabled).toBe('true');
   // aria-label must still be non-empty for screen-reader users
   expect(btnAttrs!.ariaLabel).toBeTruthy();
  });

  test('renders the .mic-unsupported-label indicating voice input is unavailable', async ({ page }) => {
   await page.addInitScript(() => {
    delete (window as unknown as Record<string, unknown>)['SpeechRecognition'];
    delete (window as unknown as Record<string, unknown>)['webkitSpeechRecognition'];
   });

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const unsupportedText = await page.evaluate(() => {
    const hostEl = document.querySelector('wiki-page-app-host');
    const micEl = hostEl?.shadowRoot?.querySelector('wiki-page-app-mic-button');
    return (
     micEl?.shadowRoot?.querySelector('span.mic-unsupported-label')?.textContent?.trim() ?? null
    );
   });

   expect(unsupportedText).toBe('Voice input not supported');
  });

  test('still renders the .mic-label caption alongside the disabled button', async ({ page }) => {
   await page.addInitScript(() => {
    delete (window as unknown as Record<string, unknown>)['SpeechRecognition'];
    delete (window as unknown as Record<string, unknown>)['webkitSpeechRecognition'];
   });

   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const labelText = await page.evaluate(() => {
    const hostEl = document.querySelector('wiki-page-app-host');
    const micEl = hostEl?.shadowRoot?.querySelector('wiki-page-app-mic-button');
    return micEl?.shadowRoot?.querySelector('span.mic-label')?.textContent?.trim() ?? null;
   });

   expect(labelText).toBe('Speak');
  });
 });

 test.describe('when prefers-reduced-motion is set to reduce', () => {
  // Playwright supports emulating prefers-reduced-motion via page.emulateMedia().
  // The CSS replaces the pulse animation with a static outline — a pure-CSS change
  // not asserted here. We verify the button still renders and is operable so the
  // reduced-motion path does not accidentally break rendering.
  test('still renders an enabled button that is operable', async ({ page }) => {
   await page.emulateMedia({ reducedMotion: 'reduce' });
   await page.goto(`/${TEST_PAGE}/view`);

   const host = page.locator('wiki-page-app-host');
   await expect(host).toHaveCount(1, { timeout: PAGE_LOAD_TIMEOUT_MS });
   await expect(
    host.locator('.wiki-page-app-card-title', { hasText: 'MicButton test' }),
   ).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });

   const btnAttrs = await page.evaluate(() => {
    const hostEl = document.querySelector('wiki-page-app-host');
    const micEl = hostEl?.shadowRoot?.querySelector('wiki-page-app-mic-button');
    const btn = micEl?.shadowRoot?.querySelector('button.mic-btn') as HTMLButtonElement | null;
    if (!btn) return null;
    return { disabled: btn.disabled };
   });

   expect(btnAttrs, 'button.mic-btn must be found in nested shadow DOM').not.toBeNull();
   expect(btnAttrs!.disabled).toBe(false);
  });
 });
});
