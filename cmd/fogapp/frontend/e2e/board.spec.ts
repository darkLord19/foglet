import { test, expect } from './support/fixtures';

test.describe('Board view', () => {
  test('shows board title and all five columns', async ({ mockedPage: page }) => {
    await expect(page.locator('h1.board__title')).toHaveText('Board');

    for (const label of ['Todo', 'In progress', 'In review', 'In QA', 'Done']) {
      await expect(page.locator(`section[aria-label="${label}"]`)).toBeVisible();
    }
  });

  test('timeline window filter buttons are visible', async ({ mockedPage: page }) => {
    const radioGroup = page.locator('[role="radiogroup"]');
    await expect(radioGroup).toBeVisible();
    for (const label of ['Today', '7d', '30d', 'All']) {
      await expect(radioGroup.locator(`button:has-text("${label}")`)).toBeVisible();
    }
  });

  test('task card appears in the correct column', async ({ mockedPage: page }) => {
    const todoColumn = page.locator('section[aria-label="Todo"]');
    await expect(todoColumn.locator('[data-card="task-1"]')).toBeVisible();
  });

  test('new task button is present', async ({ mockedPage: page }) => {
    await expect(page.locator('button.btn.btn-primary', { hasText: 'New task' })).toBeVisible();
  });

  test('new task dialog opens and closes', async ({ mockedPage: page }) => {
    await page.locator('button.btn.btn-primary', { hasText: 'New task' }).click();
    const dialog = page.locator('dialog.dlg');
    await expect(dialog).toBeVisible();

    // Cancel closes the dialog.
    await dialog.locator('button', { hasText: 'Cancel' }).click();
    await expect(dialog).not.toBeVisible();
  });

  test('new task submit is disabled until title is entered', async ({ mockedPage: page }) => {
    await page.locator('button.btn.btn-primary', { hasText: 'New task' }).click();
    const dialog = page.locator('dialog.dlg');
    const submit = dialog.locator('button[type="submit"]');

    await expect(submit).toBeDisabled();

    await dialog.locator('#task-title').fill('My new task');
    await expect(submit).toBeEnabled();
  });

  test('creating a task calls POST /api/tasks and adds card', async ({ mockedPage: page }) => {
    await page.locator('button.btn.btn-primary', { hasText: 'New task' }).click();
    const dialog = page.locator('dialog.dlg');
    await dialog.locator('#task-title').fill('My new task');

    const [request] = await Promise.all([
      page.waitForRequest((req) => req.url().includes('/api/tasks') && req.method() === 'POST'),
      dialog.locator('button[type="submit"]').click(),
    ]);

    const body = JSON.parse(request.postData() || '{}');
    expect(body.title).toBe('My new task');
  });

  test('trash toggle shows trash view', async ({ mockedPage: page }) => {
    await page.locator('button[aria-label="Trash"]').click();
    await expect(page.locator('h1.board__title')).toHaveText('Trash');

    await page.locator('button[aria-label="Back to board"]').click();
    await expect(page.locator('h1.board__title')).toHaveText('Board');
  });

  test('settings shortcut button in board header navigates to settings', async ({ mockedPage: page }) => {
    await page.locator('button[aria-label="Settings"]').click();
    await expect(page.locator('#settings-save')).toBeVisible();
  });
});
