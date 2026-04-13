import { test, expect, type APIRequestContext } from '@playwright/test';

// E2E tests for the view toggle accessible radio buttons.
// These tests verify that the accessibility improvements introduced in PR #843
// are working correctly and will catch regressions.
//
// The view toggle renders as a radio group (<div role="radiogroup">) with two
// radio buttons (<button role="radio">): "table" and "cards".

const TEST_PAGE = 'e2e_view_toggle_a11y_test';

const PAGE_LOAD_TIMEOUT_MS = 15000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;

async function callPageAPI(
  request: APIRequestContext,
  method: string,
  body: Record<string, unknown>,
) {
  return request.post(`/api.v1.PageManagementService/${method}`, {
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    data: body,
  });
}

async function setupTestPage(request: APIRequestContext): Promise<void> {
  const markdown = `+++
identifier = "${TEST_PAGE}"
title = "E2E View Toggle A11y Test"
+++

# View Toggle A11y Test

| Name | Value |
|------|-------|
| Alpha | 1 |
| Beta | 2 |
| Gamma | 3 |`;

  const createResp = await callPageAPI(request, 'CreatePage', {
    pageName: TEST_PAGE,
    contentMarkdown: markdown,
  });
  if (createResp.ok()) {
    const body = await createResp.json() as { success: boolean };
    if (body.success) return;
  }

  const resetResp = await callPageAPI(request, 'UpdatePageContent', {
    pageName: TEST_PAGE,
    newContentMarkdown: markdown,
  });
  expect(resetResp.ok()).toBeTruthy();
}

test.describe('View Toggle Accessible Radio Buttons', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(60000);

  test.beforeAll(async ({ request }) => {
    await setupTestPage(request);
  });

  test.afterAll(async ({ request }) => {
    await callPageAPI(request, 'DeletePage', { pageName: TEST_PAGE });
  });

  test.describe('radio group structure', () => {
    test('should render the view toggle as a radiogroup', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const radioGroup = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const container = wikiTable?.shadowRoot?.querySelector('[role="radiogroup"]');
        return {
          role: container?.getAttribute('role') ?? null,
          ariaLabel: container?.getAttribute('aria-label') ?? null,
        };
      });

      expect(radioGroup.role).toBe('radiogroup');
      expect(radioGroup.ariaLabel).toBe('View mode');
    });

    test('should render two radio buttons inside the radiogroup', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const radioCount = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        return wikiTable?.shadowRoot?.querySelectorAll('[role="radiogroup"] [role="radio"]').length ?? 0;
      });

      expect(radioCount).toBe(2);
    });

    test('should have the table radio button with data-view="table"', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const tableButton = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const btn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
        return {
          exists: btn !== null,
          role: btn?.getAttribute('role') ?? null,
          dataView: btn?.getAttribute('data-view') ?? null,
        };
      });

      expect(tableButton.exists).toBe(true);
      expect(tableButton.role).toBe('radio');
      expect(tableButton.dataView).toBe('table');
    });

    test('should have the cards radio button with data-view="cards"', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const cardsButton = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const btn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="cards"]');
        return {
          exists: btn !== null,
          role: btn?.getAttribute('role') ?? null,
          dataView: btn?.getAttribute('data-view') ?? null,
        };
      });

      expect(cardsButton.exists).toBe(true);
      expect(cardsButton.role).toBe('radio');
      expect(cardsButton.dataView).toBe('cards');
    });
  });

  test.describe('initial aria-checked state', () => {
    test('should have table radio checked and cards radio unchecked by default', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const ariaChecked = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const tableBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
        const cardsBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="cards"]');
        return {
          tableAriaChecked: tableBtn?.getAttribute('aria-checked') ?? null,
          cardsAriaChecked: cardsBtn?.getAttribute('aria-checked') ?? null,
        };
      });

      expect(ariaChecked.tableAriaChecked).toBe('true');
      expect(ariaChecked.cardsAriaChecked).toBe('false');
    });

    test('should have tabindex=0 on the active (table) radio and tabindex=-1 on inactive (cards)', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const tabindices = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const tableBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
        const cardsBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="cards"]');
        return {
          tableTabindex: tableBtn?.getAttribute('tabindex') ?? null,
          cardsTabindex: cardsBtn?.getAttribute('tabindex') ?? null,
        };
      });

      expect(tabindices.tableTabindex).toBe('0');
      expect(tabindices.cardsTabindex).toBe('-1');
    });
  });

  test.describe('when clicking the cards radio button', () => {
    test('should update aria-checked: cards becomes true, table becomes false', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Click the cards button (auto-pierces shadow DOM)
      await page.locator('wiki-table').locator('[data-view="cards"]').click();

      const ariaChecked = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const tableBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
        const cardsBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="cards"]');
        return {
          tableAriaChecked: tableBtn?.getAttribute('aria-checked') ?? null,
          cardsAriaChecked: cardsBtn?.getAttribute('aria-checked') ?? null,
        };
      });

      expect(ariaChecked.cardsAriaChecked).toBe('true');
      expect(ariaChecked.tableAriaChecked).toBe('false');
    });

    test('should show the card view', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await page.locator('wiki-table').locator('[data-view="cards"]').click();

      await expect(page.locator('wiki-table').locator('.card-view')).toBeVisible();
    });
  });

  test.describe('when clicking the table radio button after switching to cards', () => {
    test('should restore aria-checked: table becomes true, cards becomes false', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await page.locator('wiki-table').locator('[data-view="cards"]').click();
      await page.locator('wiki-table').locator('[data-view="table"]').click();

      const ariaChecked = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const tableBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
        const cardsBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="cards"]');
        return {
          tableAriaChecked: tableBtn?.getAttribute('aria-checked') ?? null,
          cardsAriaChecked: cardsBtn?.getAttribute('aria-checked') ?? null,
        };
      });

      expect(ariaChecked.tableAriaChecked).toBe('true');
      expect(ariaChecked.cardsAriaChecked).toBe('false');
    });

    test('should show the table view', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await page.locator('wiki-table').locator('[data-view="cards"]').click();
      await page.locator('wiki-table').locator('[data-view="table"]').click();

      await expect(page.locator('wiki-table').locator('.table-wrapper')).toBeVisible();
    });
  });

  test.describe('keyboard navigation', () => {
    test.describe('when ArrowRight is pressed on the table radio button', () => {
      test('should switch to cards view', async ({ page }) => {
        await page.goto(`/${TEST_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
        await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        // Focus the active (table) radio button and press ArrowRight
        await page.locator('wiki-table').locator('[data-view="table"]').focus();
        await page.keyboard.press('ArrowRight');

        const ariaChecked = await page.evaluate(() => {
          const wikiTable = document.querySelector('wiki-table');
          const cardsBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="cards"]');
          return cardsBtn?.getAttribute('aria-checked') ?? null;
        });

        expect(ariaChecked).toBe('true');
      });
    });

    test.describe('when ArrowDown is pressed on the table radio button', () => {
      test('should switch to cards view', async ({ page }) => {
        await page.goto(`/${TEST_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
        await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        await page.locator('wiki-table').locator('[data-view="table"]').focus();
        await page.keyboard.press('ArrowDown');

        const ariaChecked = await page.evaluate(() => {
          const wikiTable = document.querySelector('wiki-table');
          const cardsBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="cards"]');
          return cardsBtn?.getAttribute('aria-checked') ?? null;
        });

        expect(ariaChecked).toBe('true');
      });
    });

    test.describe('when ArrowLeft is pressed on the cards radio button', () => {
      test('should switch to table view', async ({ page }) => {
        await page.goto(`/${TEST_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
        await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        // First switch to cards
        await page.locator('wiki-table').locator('[data-view="cards"]').click();

        // Now press ArrowLeft to go back to table
        await page.locator('wiki-table').locator('[data-view="cards"]').focus();
        await page.keyboard.press('ArrowLeft');

        const ariaChecked = await page.evaluate(() => {
          const wikiTable = document.querySelector('wiki-table');
          const tableBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
          return tableBtn?.getAttribute('aria-checked') ?? null;
        });

        expect(ariaChecked).toBe('true');
      });
    });

    test.describe('when ArrowUp is pressed on the cards radio button', () => {
      test('should switch to table view', async ({ page }) => {
        await page.goto(`/${TEST_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
        await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        // First switch to cards
        await page.locator('wiki-table').locator('[data-view="cards"]').click();

        await page.locator('wiki-table').locator('[data-view="cards"]').focus();
        await page.keyboard.press('ArrowUp');

        const ariaChecked = await page.evaluate(() => {
          const wikiTable = document.querySelector('wiki-table');
          const tableBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
          return tableBtn?.getAttribute('aria-checked') ?? null;
        });

        expect(ariaChecked).toBe('true');
      });
    });
  });

  test.describe('focus visibility', () => {
    test('should have a focusable active radio button', async ({ page }) => {
      await page.goto(`/${TEST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-table').locator('.table-wrapper, .card-view')).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // The active radio (table) should have tabindex=0, making it reachable via Tab
      const tableButton = page.locator('wiki-table').locator('[data-view="table"]');
      await tableButton.focus();

      const isFocused = await page.evaluate(() => {
        const wikiTable = document.querySelector('wiki-table');
        const tableBtn = wikiTable?.shadowRoot?.querySelector('[role="radio"][data-view="table"]');
        return tableBtn === wikiTable?.shadowRoot?.activeElement;
      });

      expect(isFocused).toBe(true);
    });
  });
});
