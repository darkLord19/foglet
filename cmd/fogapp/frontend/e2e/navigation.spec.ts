import { test, expect } from './support/fixtures';

test.describe('Sidebar navigation', () => {
  test('Board button is active on initial load', async ({ mockedPage: page }) => {
    const boardBtn = page.getByRole('button', { name: 'Board' });
    await expect(boardBtn).toHaveAttribute('aria-current', 'page');
  });

  test('clicking New session switches view', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: 'New session' }).click();
    await expect(page.locator('section.composer[aria-label="New session"]')).toBeVisible();
    await expect(page.getByRole('button', { name: 'New session' })).toHaveAttribute('aria-current', 'page');
  });

  test('clicking Settings switches view', async ({ mockedPage: page }) => {
    await page.locator('.sidebar__foot button.row').click();
    await expect(page.locator('#settings-save')).toBeVisible();
    await expect(page.locator('.sidebar__foot button.row')).toHaveAttribute('aria-current', 'page');
  });

  test('clicking Board from another view returns to board', async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: 'New session' }).click();
    await page.getByRole('button', { name: 'Board' }).click();
    await expect(page.locator('h1.board__title')).toBeVisible();
  });

  test('nav has accessible label', async ({ mockedPage: page }) => {
    await expect(page.locator('nav[aria-label="Application navigation"]')).toBeVisible();
  });
});

test.describe('Keyboard shortcuts', () => {
  test('Cmd+, opens settings', async ({ mockedPage: page }) => {
    await page.keyboard.press('Meta+,');
    await expect(page.locator('#settings-save')).toBeVisible();
  });

  test('Cmd+N opens new session', async ({ mockedPage: page }) => {
    await page.keyboard.press('Meta+n');
    await expect(page.locator('#chat-prompt')).toBeVisible();
  });

  test('Escape from session detail returns to board', async ({ mockedPage: page }) => {
    // Navigate to session detail first.
    await page.getByRole('button', { name: /Initial prompt/ }).first().click();
    await page.locator('#detail-title').waitFor();

    await page.keyboard.press('Escape');
    await expect(page.locator('h1.board__title')).toBeVisible();
  });

  test('Escape is a no-op on the board view', async ({ mockedPage: page }) => {
    // Already on board; pressing Escape should not crash or navigate away.
    await page.keyboard.press('Escape');
    await expect(page.locator('h1.board__title')).toBeVisible();
  });
});
