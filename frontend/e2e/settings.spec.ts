import { test, expect } from './fixtures/config';

test.describe('Settings Page', () => {
  test.beforeEach(async ({ page, resetConfig }) => {
    await resetConfig();
    await page.goto('/settings');
  });

  test('loads and displays settings page', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
    await expect(page.getByText('Configure Quorum behavior')).toBeVisible();
  });

  test('displays all tabs', async ({ page }) => {
    await expect(page.getByRole('tab', { name: 'General' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Workflow' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Agents' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Phases' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Git' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Advanced' })).toBeVisible();
  });

  test('General tab shows log settings', async ({ page }) => {
    await page.getByRole('tab', { name: 'General' }).click();

    await expect(page.getByText('Logging')).toBeVisible();
    await expect(page.getByLabel('Log Level')).toBeVisible();
    await expect(page.getByLabel('Log Level')).toHaveValue('info');
  });

  test('navigates between tabs', async ({ page }) => {
    // Start on General
    await expect(page.getByText('Logging')).toBeVisible();

    // Go to Agents
    await page.getByRole('tab', { name: 'Agents' }).click();
    await expect(page.getByText('AI Agents')).toBeVisible();
    await expect(page.getByText('Claude')).toBeVisible();

    // Go to Git
    await page.getByRole('tab', { name: 'Git' }).click();
    await expect(page.getByText('Git Automation')).toBeVisible();

    // Go to Advanced
    await page.getByRole('tab', { name: 'Advanced' }).click();
    await expect(page.getByText('Trace Logging')).toBeVisible();
  });

  test('Agents tab shows all 4 agents including Copilot', async ({ page }) => {
    await page.getByRole('tab', { name: 'Agents' }).click();

    await expect(page.getByText('Claude')).toBeVisible();
    await expect(page.getByText('Codex')).toBeVisible();
    await expect(page.getByText('Gemini')).toBeVisible();
    await expect(page.getByText('Copilot')).toBeVisible();
  });

  test('Phases tab shows mode toggle', async ({ page }) => {
    await page.getByRole('tab', { name: 'Phases' }).click();

    await expect(page.getByText('Analyze Phase')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Single Agent' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Moderated Discussion' })).toBeVisible();
  });

  test('Git tab shows dependency chain', async ({ page }) => {
    await page.getByRole('tab', { name: 'Git' }).click();

    // Dependency chain visualization
    await expect(page.getByText('Commit')).toBeVisible();
    await expect(page.getByText('Push')).toBeVisible();
    await expect(page.getByText('PR')).toBeVisible();
    await expect(page.getByText('Merge')).toBeVisible();
  });

  test('tooltips appear on hover', async ({ page }) => {
    await page.getByRole('tab', { name: 'General' }).click();

    // Hover over Log Level tooltip icon
    await page.getByLabel('Log Level tooltip').hover();

    await expect(page.getByText(/Use 'debug' for troubleshooting/)).toBeVisible();
  });
});
