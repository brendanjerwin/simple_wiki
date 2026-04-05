import { test, expect } from '@playwright/test';

// Timeouts
const COMPONENT_LOAD_TIMEOUT_MS = 15000;
const SAVE_TIMEOUT_MS = 10000;
const BLOG_LOAD_TIMEOUT_MS = 10000;
const DIALOG_APPEAR_TIMEOUT_MS = 5000;

// Test page identifiers
const BLOG_PAGE = 'e2etestblog';
const POST_ONE_ID = 'e2etestblog-2024-01-15-first-post';
const POST_TWO_ID = 'e2etestblog-2024-01-10-external-post';
const BLOG_HIDE_NEW_POST_PAGE = 'e2etestblognewpost';

// Captured during "create post" test for afterAll cleanup
let createdPostIdentifier = '';

test.describe('Blog Features', () => {
  test.setTimeout(90000);

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    const textarea = page.locator('wiki-editor textarea');

    // Create blog listing page
    await page.goto(`/${BLOG_PAGE}/edit`);
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await textarea.fill(`+++
identifier = "${BLOG_PAGE}"
title = "E2E Test Blog"
+++

{{ Blog "${BLOG_PAGE}" 10 }}`);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

    // Create first blog post (newest by date)
    await page.goto(`/${POST_ONE_ID}/edit`);
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await textarea.fill(`+++
identifier = "${POST_ONE_ID}"
title = "First Test Post"

[blog]
identifier = "${BLOG_PAGE}"
published-date = "2024-01-15"
+++

# First Test Post

This is the first test post content.`);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

    // Create second blog post with external URL (older by date)
    await page.goto(`/${POST_TWO_ID}/edit`);
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await textarea.fill(`+++
identifier = "${POST_TWO_ID}"
title = "External Post"

[blog]
identifier = "${BLOG_PAGE}"
published-date = "2024-01-10"
external_url = "https://example.com/external-post"
+++

# External Post

This post links to an external URL.`);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

    // Create blog listing page with hide-new-post flag
    await page.goto(`/${BLOG_HIDE_NEW_POST_PAGE}/edit`);
    await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
    await textarea.fill(`+++
identifier = "${BLOG_HIDE_NEW_POST_PAGE}"
title = "E2E Test Blog No New Post"

[blog]
hide-new-post = true
+++

{{ Blog "${BLOG_HIDE_NEW_POST_PAGE}" 10 }}`);
    await textarea.press('Space');
    await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });

    await ctx.close();
  });

  test.afterAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    const textarea = page.locator('wiki-editor textarea');

    const testPages = [BLOG_PAGE, POST_ONE_ID, POST_TWO_ID, BLOG_HIDE_NEW_POST_PAGE];

    // Also clean up any post created via the dialog
    if (createdPostIdentifier) {
      testPages.push(createdPostIdentifier);
    }

    for (const pageName of testPages) {
      try {
        await page.goto(`/${pageName}/edit`);
        await expect(textarea).toBeVisible({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
        await textarea.fill(`+++\nidentifier = "${pageName}"\n+++`);
        await textarea.press('Space');
        await expect(page.locator('wiki-editor .status-indicator')).toContainText('Saved', { timeout: SAVE_TIMEOUT_MS });
      } catch (_) {
        // Best-effort cleanup: ignore failures
      }
    }

    await ctx.close();
  });

  test.describe('Blog listing page', () => {
    test('should render the wiki-blog custom element', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await expect(page.locator('wiki-blog')).toBeAttached({ timeout: BLOG_LOAD_TIMEOUT_MS });
    });

    test('should display blog posts after loading', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // The wiki-blog component fetches posts via gRPC; wait for the list to appear
      await expect(page.locator('wiki-blog .blog-list')).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-blog .entry-title', { hasText: 'First Test Post' })).toBeVisible();
      await expect(page.locator('wiki-blog .entry-title', { hasText: 'External Post' })).toBeVisible();
    });

    test('should display posts sorted by published-date descending', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-blog .blog-list')).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });

      const entries = page.locator('wiki-blog .blog-entry');
      await expect(entries).toHaveCount(2, { timeout: BLOG_LOAD_TIMEOUT_MS });

      // Newest post (2024-01-15) should appear first
      await expect(entries.nth(0)).toContainText('First Test Post');
      // Older post (2024-01-10) should appear second
      await expect(entries.nth(1)).toContainText('External Post');
    });

    test('should display the published date for each post', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-blog .blog-list')).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });

      await expect(page.locator('wiki-blog time[datetime="2024-01-15"]')).toBeVisible();
      await expect(page.locator('wiki-blog time[datetime="2024-01-10"]')).toBeVisible();
    });

    test('should show the New Post button', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      await expect(page.locator('wiki-blog button', { hasText: 'New Post' })).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
    });
  });

  test.describe('Blog post with external URL', () => {
    test('should link the post title to the external URL', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-blog .blog-list')).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });

      const externalLink = page.locator('wiki-blog .entry-title a[href="https://example.com/external-post"]');
      await expect(externalLink).toBeVisible();
      await expect(externalLink).toContainText('External Post');
    });

    test('should show a wiki link alongside the external-URL post', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-blog .blog-list')).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });

      // The [wiki] link points to the internal wiki page for the post
      const wikiLink = page.locator('wiki-blog .wiki-link');
      await expect(wikiLink).toBeVisible();
      await expect(wikiLink).toContainText('[wiki]');
    });

    test('should link internal-URL posts directly to their wiki page', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-blog .blog-list')).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });

      // The first post has no external URL; it should link to the wiki page
      const internalLink = page.locator('wiki-blog .entry-title a[href="/' + POST_ONE_ID + '"]');
      await expect(internalLink).toBeVisible();
      await expect(internalLink).toContainText('First Test Post');
    });
  });

  test.describe('hide-new-post frontmatter flag', () => {
    test('should hide the New Post button when hide-new-post is true', async ({ page }) => {
      await page.goto(`/${BLOG_HIDE_NEW_POST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      // Wait for wiki-blog to be attached so we know it rendered
      await expect(page.locator('wiki-blog')).toBeAttached({ timeout: BLOG_LOAD_TIMEOUT_MS });

      // Give the component time to finish rendering
      await page.waitForTimeout(1000);

      await expect(page.locator('wiki-blog button', { hasText: 'New Post' })).not.toBeAttached();
    });

    test('should not render blog-new-post-dialog when hide-new-post is true', async ({ page }) => {
      await page.goto(`/${BLOG_HIDE_NEW_POST_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });
      await expect(page.locator('wiki-blog')).toBeAttached({ timeout: BLOG_LOAD_TIMEOUT_MS });

      await page.waitForTimeout(1000);

      await expect(page.locator('wiki-blog blog-new-post-dialog')).not.toBeAttached();
    });
  });

  test.describe('New post dialog', () => {
    test('should open when the New Post button is clicked', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
      await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
      await newPostButton.click();

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });
    });

    test('should close when the Cancel button is clicked', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
      await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
      await newPostButton.click();

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

      const cancelButton = page.locator('wiki-blog blog-new-post-dialog .btn-cancel');
      await cancelButton.click();

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached();
    });

    test('should close when the Escape key is pressed', async ({ page }) => {
      await page.goto(`/${BLOG_PAGE}/view`);
      await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

      const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
      await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
      await newPostButton.click();

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

      await page.keyboard.press('Escape');

      await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached();
    });

    test.describe('when a title is entered', () => {
      test('should display the identifier preview', async ({ page }) => {
        await page.goto(`/${BLOG_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
        await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
        await newPostButton.click();

        await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        // Fill the title input (inside title-input's shadow DOM)
        const titleInput = page.locator('wiki-blog blog-new-post-dialog title-input input');
        await titleInput.fill('My Test Post');

        // The identifier preview should appear and contain the blog-id and slugified title
        const preview = page.locator('wiki-blog blog-new-post-dialog .identifier-preview');
        await expect(preview).toBeVisible({ timeout: 3000 });
        await expect(preview).toContainText(BLOG_PAGE);
        await expect(preview).toContainText('my-test-post');
      });

      test('should include the date in the identifier preview', async ({ page }) => {
        await page.goto(`/${BLOG_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
        await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
        await newPostButton.click();

        await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        // Set a specific date so we can verify it appears in the preview
        const dateInput = page.locator('wiki-blog blog-new-post-dialog input#post-date');
        await dateInput.fill('2024-12-01');

        const titleInput = page.locator('wiki-blog blog-new-post-dialog title-input input');
        await titleInput.fill('Date Check Post');

        const preview = page.locator('wiki-blog blog-new-post-dialog .identifier-preview');
        await expect(preview).toBeVisible({ timeout: 3000 });
        await expect(preview).toContainText('2024-12-01');
      });

      test('should enable the Create Post button', async ({ page }) => {
        await page.goto(`/${BLOG_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
        await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
        await newPostButton.click();

        await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        const createButton = page.locator('wiki-blog blog-new-post-dialog .btn-primary');
        // Button is disabled without a title
        await expect(createButton).toBeDisabled();

        const titleInput = page.locator('wiki-blog blog-new-post-dialog title-input input');
        await titleInput.fill('Enabled Button Post');

        await expect(createButton).toBeEnabled({ timeout: 3000 });
      });
    });

    test.describe('when a new post is successfully created', () => {
      // Use serial mode so cleanup in afterAll captures the identifier
      test.describe.configure({ mode: 'serial' });

      test('should close the dialog and show the new post in the blog list', async ({ page }) => {
        await page.goto(`/${BLOG_PAGE}/view`);
        await expect(page.locator('#rendered')).toBeAttached({ timeout: COMPONENT_LOAD_TIMEOUT_MS });

        const newPostButton = page.locator('wiki-blog button', { hasText: 'New Post' });
        await expect(newPostButton).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });
        await newPostButton.click();

        await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).toBeAttached({ timeout: DIALOG_APPEAR_TIMEOUT_MS });

        // Set a specific date to make cleanup predictable
        const dateInput = page.locator('wiki-blog blog-new-post-dialog input#post-date');
        await dateInput.fill('2024-12-01');

        // Fill the title
        const titleInput = page.locator('wiki-blog blog-new-post-dialog title-input input');
        await titleInput.fill('E2e Dialog Test Post');

        // Wait for Create Post button to become enabled
        const createButton = page.locator('wiki-blog blog-new-post-dialog .btn-primary');
        await expect(createButton).toBeEnabled({ timeout: 3000 });
        await createButton.click();

        // Dialog should close after successful creation
        await expect(page.locator('wiki-blog blog-new-post-dialog[open]')).not.toBeAttached({ timeout: 15000 });

        // Blog list should refresh and show the new post
        const newEntry = page.locator('wiki-blog .entry-title', { hasText: /E2e Dialog Test Post/i });
        await expect(newEntry).toBeVisible({ timeout: BLOG_LOAD_TIMEOUT_MS });

        // Capture the created post identifier for afterAll cleanup
        const postLink = page.locator('wiki-blog .entry-title a', { hasText: /E2e Dialog Test Post/i });
        const href = await postLink.getAttribute('href');
        if (href) {
          createdPostIdentifier = href.replace(/^\//, '');
        }
      });
    });
  });
});
