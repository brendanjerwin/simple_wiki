import { test, expect, type Page } from '@playwright/test';

const TEST_PAGE_NAME = 'E2ESurveyTest';
const TEST_SURVEY_NAME = 'e2e_test_survey';
const TEST_USERNAME = 'e2etestuser';

const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const GRPC_RESPONSE_TIMEOUT_MS = 10000;

async function navigateToView(page: Page): Promise<void> {
  await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
  await expect(page.locator('#rendered')).toBeVisible({ timeout: PAGE_LOAD_TIMEOUT_MS });
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

test.describe('wiki-survey E2E Tests', () => {
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
        `title = "Survey E2E Test Page"\n` +
        `\n` +
        `[surveys.${TEST_SURVEY_NAME}]\n` +
        `question = "What is your experience level?"\n` +
        `\n` +
        `[[surveys.${TEST_SURVEY_NAME}.fields]]\n` +
        `name = "level"\n` +
        `type = "text"\n` +
        `\n` +
        `[[surveys.${TEST_SURVEY_NAME}.fields]]\n` +
        `name = "rating"\n` +
        `type = "number"\n` +
        `min = 1\n` +
        `max = 5\n` +
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
      console.log(`Survey test cleanup failed: ${msg}`);
    } finally {
      await ctx.close();
    }
  });

  test('should render wiki-survey web component when page contains Survey macro', async ({ page }) => {
    await navigateToView(page);

    const survey = page.locator('wiki-survey');
    await expect(survey).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(survey).toHaveAttribute('name', TEST_SURVEY_NAME);
    await expect(survey).toHaveAttribute('page', TEST_PAGE_NAME.toLowerCase());
  });

  test('should exit loading state and display survey question', async ({ page }) => {
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await expect(page.locator('wiki-survey .survey-question')).toContainText(
      'What is your experience level?',
    );
  });

  test('should show login-required message for anonymous users', async ({ page }) => {
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await expect(page.locator('wiki-survey .login-required')).toBeVisible();
  });

  test('should not show submit button for anonymous users', async ({ page }) => {
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await expect(page.locator('wiki-survey .submit-btn')).not.toBeVisible();
  });

  test('should show survey form for authenticated users', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await expect(page.locator('wiki-survey .survey-fields')).toBeVisible();
    await expect(page.locator('wiki-survey .submit-btn')).toBeVisible();
  });

  test('should render text input field for authenticated user', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    const levelField = page.locator('wiki-survey #field-level');
    await expect(levelField).toBeVisible();
    await expect(levelField).toHaveAttribute('type', 'text');
  });

  test('should render number input field for authenticated user', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    const ratingField = page.locator('wiki-survey #field-rating');
    await expect(ratingField).toBeVisible();
    await expect(ratingField).toHaveAttribute('type', 'number');
  });

  test('should submit response and show success message', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await page.locator('wiki-survey #field-level').fill('intermediate');
    await page.locator('wiki-survey #field-rating').fill('4');
    await page.locator('wiki-survey .submit-btn').click();

    await expect(page.locator('wiki-survey .success-message')).toBeVisible({
      timeout: GRPC_RESPONSE_TIMEOUT_MS,
    });
    await expect(page.locator('wiki-survey .success-message')).toContainText('Response saved!');
  });

  test('should display submitted response in responses section', async ({ page }) => {
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await expect(page.locator('wiki-survey .responses-section')).toBeVisible();
    await expect(page.locator('wiki-survey .response-item')).toBeVisible();
    await expect(page.locator('wiki-survey .response-user')).toContainText(TEST_USERNAME);
  });

  test('should show correct response count', async ({ page }) => {
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await expect(page.locator('wiki-survey .responses-title')).toContainText('1 response');
  });

  test('should prefill existing user response on reload', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await expect(page.locator('wiki-survey #field-level')).toHaveValue('intermediate');
    await expect(page.locator('wiki-survey #field-rating')).toHaveValue('4');
  });

  test('loading state should have role="status" and aria-live="polite"', async ({ page }) => {
    // Delay gRPC responses so the loading state is visible long enough to assert.
    await page.route('**/*GetFrontmatter*', async (route) => {
      await new Promise<void>((resolve) => setTimeout(resolve, 600));
      await route.continue();
    });

    await page.goto(`/${TEST_PAGE_NAME.toLowerCase()}/view`);
    const survey = page.locator('wiki-survey');
    await expect(survey).toBeAttached({ timeout: PAGE_LOAD_TIMEOUT_MS });

    const loadingEl = survey.locator('[role="status"][aria-live="polite"]');
    await expect(loadingEl).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await expect(loadingEl).toContainText('Loading survey');
  });

  test('should allow Tab navigation between form fields', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await page.locator('wiki-survey #field-level').focus();
    await page.keyboard.press('Tab');

    await expect(page.locator('wiki-survey #field-rating')).toBeFocused();
  });

  test('should allow Tab navigation to submit button', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await page.locator('wiki-survey #field-level').focus();
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');

    await expect(page.locator('wiki-survey .submit-btn')).toBeFocused();
  });

  test('should submit form with Enter key on focused submit button', async ({ page }) => {
    await injectUsername(page, TEST_USERNAME);
    await navigateToView(page);
    await waitForSurveyLoaded(page);

    await page.locator('wiki-survey #field-level').fill('advanced');
    await page.locator('wiki-survey .submit-btn').focus();
    await page.keyboard.press('Enter');

    await expect(page.locator('wiki-survey .success-message')).toBeVisible({
      timeout: GRPC_RESPONSE_TIMEOUT_MS,
    });
  });
});
