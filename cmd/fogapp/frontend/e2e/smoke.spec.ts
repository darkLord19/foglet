import { test, expect } from './support/fixtures';

test('app boots without error and shows board', async ({ mockedPage: page }) => {
  // No fault card.
  await expect(page.locator('.fault')).not.toBeVisible();

  // Sidebar brand is visible.
  await expect(page.locator('.sidebar__brand-name')).toHaveText('Fog');

  // Default view is board.
  await expect(page.locator('h1.board__title')).toBeVisible();
});

test('fault card does not appear when API is healthy', async ({ page }) => {
  // Set up mocks and navigate; the app should not fall into the error branch.
  const { setupApiMocks } = await import('./support/mock');
  await setupApiMocks(page);
  await page.goto('/');
  await page.locator('.sidebar__brand-name').waitFor();
  await expect(page.locator('.fault')).not.toBeVisible();
});
