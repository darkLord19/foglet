import { test, expect } from './support/fixtures';

test.describe('New session view', () => {
  test.beforeEach(async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: 'New session' }).click();
    await page.locator('section.composer[aria-label="New session"]').waitFor();
  });

  test('shows prompt textarea and submit button', async ({ mockedPage: page }) => {
    await expect(page.locator('#chat-prompt')).toBeVisible();
    await expect(page.locator('#chat-submit')).toBeVisible();
  });

  test('submit is disabled until prompt is filled', async ({ mockedPage: page }) => {
    await expect(page.locator('#chat-submit')).toBeDisabled();

    await page.locator('#chat-prompt').fill('Add a README file');
    await expect(page.locator('#chat-submit')).toBeEnabled();
  });

  test('repo dropdown trigger is visible', async ({ mockedPage: page }) => {
    // The Dropdown component renders a button with aria-haspopup="listbox".
    const trigger = page.locator('section.composer button[aria-haspopup="listbox"]').first();
    await expect(trigger).toBeVisible();
  });

  test('submitting sends POST /api/sessions with the prompt', async ({ mockedPage: page }) => {
    await page.locator('#chat-prompt').fill('Add a README file');

    const [request] = await Promise.all([
      page.waitForRequest(
        (req) => req.url().includes('/api/sessions') && req.method() === 'POST',
      ),
      page.locator('#chat-submit').click(),
    ]);

    const body = JSON.parse(request.postData() || '{}');
    expect(body.prompt).toBe('Add a README file');
  });

  test('Cmd+Enter submits the form', async ({ mockedPage: page }) => {
    await page.locator('#chat-prompt').fill('Keyboard shortcut prompt');

    const [request] = await Promise.all([
      page.waitForRequest(
        (req) => req.url().includes('/api/sessions') && req.method() === 'POST',
      ),
      page.locator('#chat-prompt').press('Meta+Enter'),
    ]);

    const body = JSON.parse(request.postData() || '{}');
    expect(body.prompt).toBe('Keyboard shortcut prompt');
  });
});
