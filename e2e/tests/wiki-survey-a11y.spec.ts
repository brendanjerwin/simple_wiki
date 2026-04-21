import { test, expect, type Page } from '@playwright/test';

const TEST_PAGE_NAME = 'e2esurveya11ytest';
const TEST_SURVEY_NAME = 'a11y_test_survey';
const TEST_USERNAME = 'a11ytestuser';

const SAVE_TIMEOUT_MS = 10000;
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const PAGE_LOAD_TIMEOUT_MS = 15000;
const GRPC_RESPONSE_TIMEOUT_MS = 10000;

async function navigateToView(page: Page): Promise<void> {
  await page.goto(`/${TEST_PAGE_NAME}/view`);
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

test.describe('wiki-survey Accessibility E2E Tests', () => {
  test.describe.configure({ mode: 'serial' });
  test.setTimeout(120000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    try {
      await page.goto(`/${TEST_PAGE_NAME}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const content =
        `+++\n` +
        `identifier = "${TEST_PAGE_NAME}"\n` +
        `title = "Survey A11y E2E Test Page"\n` +
        `\n` +
        `[surveys.${TEST_SURVEY_NAME}]\n` +
        `question = "How would you rate your experience?"\n` +
        `\n` +
        `[[surveys.${TEST_SURVEY_NAME}.fields]]\n` +
        `name = "feedback"\n` +
        `type = "text"\n` +
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
      await page.goto(`/${TEST_PAGE_NAME}/edit`);
      const textarea = page.locator('wiki-editor textarea');
      await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await textarea.fill(`+++\nidentifier = "${TEST_PAGE_NAME}"\n+++`);
      await textarea.press('Space');
      await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', {
        timeout: SAVE_TIMEOUT_MS,
      });
    } catch (error: unknown) {
      const msg = error instanceof Error ? error.message : String(error);
      console.log(`Survey a11y test cleanup failed: ${msg}`);
    } finally {
      await ctx.close();
    }
  });

  test.describe('aria-live announcement region', () => {
    test('submit-status region should have role=status and aria-live=polite', async ({ page }) => {
      await injectUsername(page, TEST_USERNAME);
      await navigateToView(page);
      await waitForSurveyLoaded(page);

      const submitStatus = page.locator('wiki-survey [role="status"][aria-live="polite"].submit-status');
      await expect(submitStatus).toBeAttached();
    });

    test('should contain success message after form submission', async ({ page }) => {
      await injectUsername(page, TEST_USERNAME);
      await navigateToView(page);
      await waitForSurveyLoaded(page);

      await page.locator('wiki-survey #field-feedback').fill('great experience');
      await page.locator('wiki-survey .submit-btn').click();

      const submitStatus = page.locator('wiki-survey [aria-live="polite"].submit-status');
      await expect(submitStatus).toContainText('Response saved!', {
        timeout: GRPC_RESPONSE_TIMEOUT_MS,
      });
    });
  });

  test.describe('aria-labelledby on survey fields group', () => {
    test('survey fields group should have aria-labelledby attribute', async ({ page }) => {
      await injectUsername(page, TEST_USERNAME);
      await navigateToView(page);
      await waitForSurveyLoaded(page);

      const surveyFields = page.locator('wiki-survey .survey-fields');
      await expect(surveyFields).toHaveAttribute('aria-labelledby', `survey-question-${TEST_SURVEY_NAME}`);
    });

    test('aria-labelledby should point to element containing survey question text', async ({ page }) => {
      await injectUsername(page, TEST_USERNAME);
      await navigateToView(page);
      await waitForSurveyLoaded(page);

      const questionEl = page.locator(`wiki-survey #survey-question-${TEST_SURVEY_NAME}`);
      await expect(questionEl).toBeVisible();
      await expect(questionEl).toContainText('How would you rate your experience?');
    });
  });

  test.describe('type="button" on submit button', () => {
    test('submit button should have type="button" to prevent accidental form submission', async ({ page }) => {
      await injectUsername(page, TEST_USERNAME);
      await navigateToView(page);
      await waitForSurveyLoaded(page);

      const submitBtn = page.locator('wiki-survey .submit-btn');
      await expect(submitBtn).toHaveAttribute('type', 'button');
    });
  });

  test.describe('keyboard submission', () => {
    test('pressing Enter on focused submit button should submit the form', async ({ page }) => {
      await injectUsername(page, TEST_USERNAME);
      await navigateToView(page);
      await waitForSurveyLoaded(page);

      await page.locator('wiki-survey #field-feedback').fill('keyboard submission test');
      await page.locator('wiki-survey .submit-btn').focus();
      await page.keyboard.press('Enter');

      await expect(page.locator('wiki-survey .success-message')).toBeVisible({
        timeout: GRPC_RESPONSE_TIMEOUT_MS,
      });
      await expect(page.locator('wiki-survey .success-message')).toContainText('Response saved!');
    });

    test('pressing Space on focused submit button should submit the form', async ({ page }) => {
      await injectUsername(page, TEST_USERNAME);
      await navigateToView(page);
      await waitForSurveyLoaded(page);

      await page.locator('wiki-survey #field-feedback').fill('space key submission test');
      await page.locator('wiki-survey .submit-btn').focus();
      await page.keyboard.press('Space');

      await expect(page.locator('wiki-survey .success-message')).toBeVisible({
        timeout: GRPC_RESPONSE_TIMEOUT_MS,
      });
      await expect(page.locator('wiki-survey .success-message')).toContainText('Response saved!');
    });
  });
});
