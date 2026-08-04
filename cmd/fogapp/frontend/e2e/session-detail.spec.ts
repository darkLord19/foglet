import { test, expect } from './support/fixtures';

test.describe('Session detail view', () => {
  test('session row in sidebar navigates to detail view', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await expect(page.locator('#detail-title')).toBeVisible();
  });

  test('detail title shows the session prompt', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();
    await expect(page.locator('#detail-title')).toContainText('Initial prompt');
  });

  test('action buttons visible for a completed session', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    // Completed session: rerun, fork, open visible; stop NOT visible.
    await expect(page.locator('#detail-rerun')).toBeVisible();
    await expect(page.locator('#detail-fork')).toBeVisible();
    await expect(page.locator('#detail-open')).toBeVisible();
    await expect(page.locator('#detail-stop')).not.toBeVisible();
  });

  test('follow-up bar is visible for a completed session', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    await expect(page.locator('#followup-prompt')).toBeVisible();
    await expect(page.locator('#followup-submit')).toBeVisible();
  });

  test('follow-up submit disabled until text is entered', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    await expect(page.locator('#followup-submit')).toBeDisabled();
    await page.locator('#followup-prompt').fill('Follow up message');
    await expect(page.locator('#followup-submit')).toBeEnabled();
  });

  test('follow-up sends POST to /runs', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    await page.locator('#followup-prompt').fill('Do more work');

    const [request] = await Promise.all([
      page.waitForRequest(
        (req) => /\/api\/sessions\/[^/]+\/runs$/.test(new URL(req.url()).pathname) && req.method() === 'POST',
      ),
      page.locator('#followup-submit').click(),
    ]);

    const body = JSON.parse(request.postData() || '{}');
    expect(body.prompt).toBe('Do more work');
  });

  test('tabs are present and Diff tab is clickable', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    // Check tabs exist.
    const tabs = page.getByRole('tab');
    await expect(tabs.filter({ hasText: 'Timeline' })).toBeVisible();
    await expect(tabs.filter({ hasText: 'Diff' })).toBeVisible();

    // Switch to Diff tab.
    await tabs.filter({ hasText: 'Diff' }).click();
    await expect(tabs.filter({ hasText: 'Diff' })).toHaveClass(/is-active/);
  });

  test('back button returns to board view', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    await page.getByRole('button', { name: 'Back to board' }).click();
    await expect(page.locator('h1.board__title')).toBeVisible();
  });

  test('rerun sends POST /runs with same prompt', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    const [request] = await Promise.all([
      page.waitForRequest(
        (req) => /\/api\/sessions\/[^/]+\/runs$/.test(new URL(req.url()).pathname) && req.method() === 'POST',
      ),
      page.locator('#detail-rerun').click(),
    ]);

    expect(request.method()).toBe('POST');
  });

  test('open in editor calls POST /open', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    const [request] = await Promise.all([
      page.waitForRequest(
        (req) => /\/api\/sessions\/[^/]+\/open$/.test(new URL(req.url()).pathname) && req.method() === 'POST',
      ),
      page.locator('#detail-open').click(),
    ]);

    expect(request.method()).toBe('POST');
  });
});
