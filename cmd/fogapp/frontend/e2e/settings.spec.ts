import { test, expect } from './support/fixtures';

test.describe('Settings view', () => {
  test.beforeEach(async ({ mockedPage: page }) => {
    await page.getByRole('button', { name: 'Settings' }).last().click();
    await page.locator('#settings-save').waitFor();
  });

  test('settings page is visible with save button', async ({ mockedPage: page }) => {
    await expect(page.locator('#settings-save')).toBeVisible();
    await expect(page.locator('#settings-save')).toHaveText('Save settings');
  });

  test('branch prefix input shows current value', async ({ mockedPage: page }) => {
    await expect(page.locator('#settings-branch-prefix')).toHaveValue('fog/');
  });

  test('editing branch prefix and saving sends PUT /api/settings', async ({ mockedPage: page }) => {
    await page.locator('#settings-branch-prefix').fill('team/');

    const [request] = await Promise.all([
      page.waitForRequest(
        (req) => req.url().includes('/api/settings') && req.method() === 'PUT',
      ),
      page.locator('#settings-save').click(),
    ]);

    const body = JSON.parse(request.postData() || '{}');
    expect(body.branch_prefix).toBe('team/');
  });

  test('GitHub CLI status badges are visible', async ({ mockedPage: page }) => {
    // gh_installed: true and gh_authenticated: true in mock.
    await expect(page.locator('.badge', { hasText: 'Installed' })).toBeVisible();
    await expect(page.locator('.badge', { hasText: 'Signed in' })).toBeVisible();
  });

  test('workflow toggle fields are visible', async ({ mockedPage: page }) => {
    await expect(page.locator('#auto-pr')).toBeVisible();
    await expect(page.locator('#notify')).toBeVisible();
    await expect(page.locator('#keep-awake')).toBeVisible();
  });

  test('back button navigates to board', async ({ mockedPage: page }) => {
    await page.locator('.st__back').click();
    await expect(page.locator('h1.board__title')).toBeVisible();
  });

  test('agent section shows available tools', async ({ mockedPage: page }) => {
    // The Agent section should have a tool dropdown.
    await expect(page.getByText('Default tool')).toBeVisible();
    await expect(page.getByText('Default model')).toBeVisible();
  });
});
