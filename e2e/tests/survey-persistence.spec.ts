import { test, expect, type Page } from '@playwright/test';

const TEST_PAGE_NAME = 'E2ESurveyPersistence';
const TEST_SURVEY_NAME = 'persist_survey';
const TEST_USERNAME = 'e2epersistuser';

const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const GRPC_RESPONSE_TIMEOUT_MS = 10000;

async function navigateToView(page: Page): Promise<void> {
  await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
  await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
}

async function navigateToEdit(page: Page): Promise<void> {
  await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
  const textarea = page.locator('wiki-editor textarea');
  await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

async function waitForSurveyLoaded(page: Page): Promise<void> {
  const survey = page.locator('wiki-survey');
  await expect(survey).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  await expect(survey.locator('.loading')).not.toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
}

/**
 * Inject a test username via a getter/setter trap before the page loads.
 * Must be called before page.goto() so the init script runs before the
 * server-rendered template script overwrites window.simple_wiki.
 */
async function injectUsername(page: Page, username: string): Promise<void> {
  await page.addInitScript((user: string) => {
    let stored: Record<string, unknown> = {};
    Object.defineProperty(window, 'simple_wiki', {
      get() {
        return stored;
      },
      set(v: Record<string, unknown>) {
        stored = { ...v, username: user };
      },
      configurable: true,
      enumerable: true,
    });
  }, username);
}

async function getEditorContent(page: Page): Promise<string> {
  const textarea = page.locator('wiki-editor textarea');
  await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
  return textarea.inputValue();
}

test.describe('Survey Response Persistence E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(120000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const content =
        `+++\n` +
        `identifier = "${TEST_PAGE_NAME.toLowerCase()}"\n` +
        `title = "Survey Persistence E2E Test Page"\n` +
        `\n` +
        `[surveys.${TEST_SURVEY_NAME}]\n` +
        `question = "Tell us about yourself"\n` +
        `\n` +
        `[[surveys.${TEST_SURVEY_NAME}.fields]]\n` +
        `name = "text_field"\n` +
        `type = "text"\n` +
        `\n` +
        `[[surveys.${TEST_SURVEY_NAME}.fields]]\n` +
        `name = "number_field"\n` +
        `type = "number"\n` +
        `min = 1\n` +
        `max = 10\n` +
        `\n` +
        `[[surveys.${TEST_SURVEY_NAME}.fields]]\n` +
        `name = "bool_field"\n` +
        `type = "boolean"\n` +
        `+++\n` +
        `\n` +
        `{{ Survey "${TEST_SURVEY_NAME}" }}`;

      await textarea.fill(content);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });
    } finally {
      await ctx.close();
    }
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++\nidentifier = "${TEST_PAGE_NAME.toLowerCase()}"\n+++`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });
    } catch (error: unknown) {
      const msg = error instanceof Error ? error.message : String(error);
      console.log(`Survey persistence test cleanup failed: ${msg}`);
    } finally {
      await ctx.close();
    }
  });

  test('should persist submitted response for all field types in page frontmatter', async ({
    page,
  }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await page.locator('wiki-survey #field-text_field').fill('hello world');
    await page.locator('wiki-survey #field-number_field').fill('7');
    await page.locator('wiki-survey #field-bool_field').check();
    await page.locator('wiki-survey .submit-btn').click();

    await expect(page.locator('wiki-survey .success-message')).toBeVisible({
      timeout: GRPC_RESPONSE_TIMEOUT_MS,
    });

    // Navigate to edit page to inspect the raw frontmatter
    await navigateToEdit(page);
    const content = await getEditorContent(page);

    // Verify the response is stored under the user key
    expect(content).toContain(`user = "${TEST_USERNAME}"`);

    // Verify text field persisted correctly
    expect(content).toContain('text_field = "hello world"');

    // Verify number field persisted as a numeric value
    expect(content).toMatch(/number_field\s*=\s*7(?:\.0)?/);

    // Verify boolean field persisted as true
    expect(content).toMatch(/bool_field\s*=\s*true/);
  });

  test('should pre-populate all field types from saved response on page reload', async ({
    page,
  }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    // Text field should be restored
    await expect(page.locator('wiki-survey #field-text_field')).toHaveValue('hello world');

    // Number field should be restored
    await expect(page.locator('wiki-survey #field-number_field')).toHaveValue('7');

    // Boolean field should be restored as checked
    await expect(page.locator('wiki-survey #field-bool_field')).toBeChecked();
  });

  test('should update existing response without duplication on re-submit', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    // Change all field values and re-submit
    await page.locator('wiki-survey #field-text_field').fill('updated value');
    await page.locator('wiki-survey #field-number_field').fill('3');
    await page.locator('wiki-survey #field-bool_field').uncheck();
    await page.locator('wiki-survey .submit-btn').click();

    await expect(page.locator('wiki-survey .success-message')).toBeVisible({
      timeout: GRPC_RESPONSE_TIMEOUT_MS,
    });

    // Navigate to edit page and verify updated values
    await navigateToEdit(page);
    const content = await getEditorContent(page);

    expect(content).toContain('text_field = "updated value"');
    expect(content).toMatch(/number_field\s*=\s*3(?:\.0)?/);
    expect(content).toMatch(/bool_field\s*=\s*false/);

    // Verify the response was updated in-place, not duplicated
    const userEntries = content.match(new RegExp(`user = "${TEST_USERNAME}"`, 'g'));
    expect(userEntries).toHaveLength(1);
  });
});
