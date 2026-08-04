import { test as base, expect, type Page } from '@playwright/test';
import { setupApiMocks } from './mock';

type Fixtures = {
  /** Page with all API routes mocked and the app fully booted. */
  mockedPage: Page;
};

export const test = base.extend<Fixtures>({
  mockedPage: async ({ page }, use) => {
    await setupApiMocks(page);
    await page.goto('/');
    // Wait for bootstrap to complete: sidebar brand must be visible.
    await page.locator('.sidebar__brand-name').waitFor({ timeout: 15_000 });
    await use(page);
  },
});

export { expect };
