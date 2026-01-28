import { test, expect } from './fixtures/config';

test.describe('Settings Save/Discard', () => {
  test.beforeEach(async ({ page, resetConfig }) => {
    await resetConfig();
    await page.goto('/settings');
  });

  test('shows save toolbar when changes made', async ({ page }) => {
    // Initially no toolbar
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();

    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');

    // Toolbar appears
    await expect(page.getByText('You have unsaved changes')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Save Changes' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Discard' })).toBeVisible();
  });

  test('discard reverts changes', async ({ page }) => {
    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');
    await expect(page.getByLabel('Log Level')).toHaveValue('debug');

    // Discard
    await page.getByRole('button', { name: 'Discard' }).click();

    // Should revert to original
    await expect(page.getByLabel('Log Level')).toHaveValue('info');
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();
  });

  test('save persists changes', async ({ page }) => {
    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');

    // Save
    await page.getByRole('button', { name: 'Save Changes' }).click();

    // Wait for save to complete
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();

    // Reload page
    await page.reload();

    // Change should persist
    await expect(page.getByLabel('Log Level')).toHaveValue('debug');
  });

  test('save button disabled during save', async ({ page }) => {
    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');

    // Click save
    const saveButton = page.getByRole('button', { name: 'Save Changes' });
    await saveButton.click();

    // Button should show saving state
    await expect(page.getByText('Saving...')).toBeVisible();
  });

  test('multiple changes tracked correctly', async ({ page }) => {
    // Change log level
    await page.getByLabel('Log Level').selectOption('debug');

    // Change log format
    await page.getByLabel('Log Format').selectOption('json');

    // Go to Workflow tab and change something
    await page.getByRole('tab', { name: 'Workflow' }).click();
    await page.getByLabel('Max Retries').fill('5');

    // Save all changes
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();

    // Reload and verify all changes
    await page.reload();

    await expect(page.getByLabel('Log Level')).toHaveValue('debug');
    await expect(page.getByLabel('Log Format')).toHaveValue('json');

    await page.getByRole('tab', { name: 'Workflow' }).click();
    await expect(page.getByLabel('Max Retries')).toHaveValue('5');
  });

  test('toggle changes tracked correctly', async ({ page }) => {
    // Navigate to Advanced tab
    await page.getByRole('tab', { name: 'Advanced' }).click();

    // Enable tracing
    await page.getByLabel('Enable Tracing').click();

    // Toolbar should appear
    await expect(page.getByText('You have unsaved changes')).toBeVisible();

    // Save
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();

    // Reload and verify
    await page.reload();
    await page.getByRole('tab', { name: 'Advanced' }).click();
    await expect(page.getByLabel('Enable Tracing')).toBeChecked();
  });
});
