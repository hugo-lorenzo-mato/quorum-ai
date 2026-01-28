import { test, expect } from './fixtures/config';

test.describe('Settings Validation', () => {
  test.beforeEach(async ({ page, resetConfig }) => {
    await resetConfig();
    await page.goto('/settings');
  });

  test('shows validation error for invalid value', async ({ page }) => {
    // Go to Workflow tab
    await page.getByRole('tab', { name: 'Workflow' }).click();

    // Enter invalid max_retries (negative)
    await page.getByLabel('Max Retries').fill('-1');
    await page.getByLabel('Max Retries').blur();

    // Error should appear
    await expect(page.getByText(/must be between 0 and 10/i)).toBeVisible();
  });

  test('save button disabled when validation errors exist', async ({ page }) => {
    // Go to Workflow tab
    await page.getByRole('tab', { name: 'Workflow' }).click();

    // Enter invalid value
    await page.getByLabel('Max Retries').fill('-1');
    await page.getByLabel('Max Retries').blur();

    // Try to save
    const saveButton = page.getByRole('button', { name: 'Save Changes' });
    await expect(saveButton).toBeDisabled();

    // Error count shown in toolbar
    await expect(page.getByText(/1 validation error/i)).toBeVisible();
  });

  test('validation error clears when fixed', async ({ page }) => {
    // Go to Workflow tab
    await page.getByRole('tab', { name: 'Workflow' }).click();

    // Enter invalid value
    await page.getByLabel('Max Retries').fill('-1');
    await page.getByLabel('Max Retries').blur();

    // Error appears
    await expect(page.getByText(/must be between 0 and 10/i)).toBeVisible();

    // Fix the value
    await page.getByLabel('Max Retries').fill('5');
    await page.getByLabel('Max Retries').blur();

    // Error should clear
    await expect(page.getByText(/must be between 0 and 10/i)).not.toBeVisible();

    // Save should be enabled
    const saveButton = page.getByRole('button', { name: 'Save Changes' });
    await expect(saveButton).toBeEnabled();
  });

  test('phases mutual exclusion validation', async ({ page }) => {
    // Go to Phases tab
    await page.getByRole('tab', { name: 'Phases' }).click();

    // The UI should prevent setting both single_agent and moderator
    // by using a mode toggle that's mutually exclusive
    const analyzePhase = page.locator('[data-phase="analyze"]');

    // Click Single Agent mode
    await analyzePhase.getByRole('button', { name: 'Single Agent' }).click();

    // single_agent field should be visible
    await expect(analyzePhase.getByLabel('Agent')).toBeVisible();

    // Click Moderated mode
    await analyzePhase.getByRole('button', { name: 'Moderated Discussion' }).click();

    // moderator and participants should be visible, single_agent hidden
    await expect(analyzePhase.getByLabel('Moderator')).toBeVisible();
    await expect(analyzePhase.getByLabel('Participants')).toBeVisible();
  });

  test('git dependency chain validation', async ({ page }) => {
    // Go to Git tab
    await page.getByRole('tab', { name: 'Git' }).click();

    // Try to enable auto_push without auto_commit
    // First disable auto_commit
    const autoCommit = page.getByLabel('Auto Commit');
    if (await autoCommit.isChecked()) {
      await autoCommit.click();
    }

    // auto_push should be disabled
    const autoPush = page.getByLabel('Auto Push');
    await expect(autoPush).toBeDisabled();

    // Helper text should explain
    await expect(page.getByText(/Enable 'Auto Commit' first/i)).toBeVisible();
  });

  test('report fields disabled when reports disabled', async ({ page }) => {
    // On General tab
    await page.getByRole('tab', { name: 'General' }).click();

    // Disable reports
    await page.getByLabel('Enable Reports').click();

    // Other report fields should be disabled
    await expect(page.getByLabel('Report Directory')).toBeDisabled();
    await expect(page.getByLabel('Use UTC Timestamps')).toBeDisabled();
    await expect(page.getByLabel('Include Raw Output')).toBeDisabled();
  });

  test('server validation for port range', async ({ page }) => {
    // Go to Advanced tab
    await page.getByRole('tab', { name: 'Advanced' }).click();

    // Enter invalid port
    await page.getByLabel('Port').fill('80'); // Below 1024
    await page.getByLabel('Port').blur();

    // Should show validation error
    await expect(page.getByText(/Port must be between 1024 and 65535/i)).toBeVisible();
  });

  test('reset to defaults works', async ({ page }) => {
    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();

    // Go to Advanced tab for reset
    await page.getByRole('tab', { name: 'Advanced' }).click();

    // Click reset
    await page.getByRole('button', { name: 'Reset to Defaults' }).click();

    // Confirm dialog appears
    await expect(page.getByText('Reset Configuration?')).toBeVisible();

    // Confirm reset
    await page.getByRole('button', { name: 'Reset Everything' }).click();

    // Go back to General tab
    await page.getByRole('tab', { name: 'General' }).click();

    // Value should be back to default
    await expect(page.getByLabel('Log Level')).toHaveValue('info');
  });
});
