import { test, expect } from './fixtures/config';
import fs from 'fs';

test.describe('Settings ETag Conflict', () => {
  test.beforeEach(async ({ page, resetConfig }) => {
    await resetConfig();
    await page.goto('/settings');
  });

  test('shows conflict dialog when config modified externally', async ({ page, configFile }) => {
    // Make a change in the WebUI
    await page.getByLabel('Log Level').selectOption('debug');

    // Simulate external modification (CLI or another tab)
    const yaml = require('yaml');
    const content = yaml.parse(fs.readFileSync(configFile, 'utf-8'));
    content.log.level = 'warn';
    fs.writeFileSync(configFile, yaml.stringify(content));

    // Try to save
    await page.getByRole('button', { name: 'Save Changes' }).click();

    // Conflict dialog should appear
    await expect(page.getByText('Configuration Conflict')).toBeVisible();
    await expect(page.getByText('The configuration was modified elsewhere')).toBeVisible();
  });

  test('reload option fetches latest config', async ({ page, configFile }) => {
    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');

    // Simulate external modification
    const yaml = require('yaml');
    const content = yaml.parse(fs.readFileSync(configFile, 'utf-8'));
    content.log.level = 'warn';
    fs.writeFileSync(configFile, yaml.stringify(content));

    // Try to save, triggering conflict
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.getByText('Configuration Conflict')).toBeVisible();

    // Click reload
    await page.getByRole('button', { name: 'Reload Latest' }).click();

    // Dialog should close
    await expect(page.getByText('Configuration Conflict')).not.toBeVisible();

    // Should show the external value
    await expect(page.getByLabel('Log Level')).toHaveValue('warn');

    // Local changes should be discarded
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();
  });

  test('overwrite option forces save', async ({ page, configFile }) => {
    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');

    // Simulate external modification
    const yaml = require('yaml');
    const content = yaml.parse(fs.readFileSync(configFile, 'utf-8'));
    content.log.level = 'warn';
    fs.writeFileSync(configFile, yaml.stringify(content));

    // Try to save, triggering conflict
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.getByText('Configuration Conflict')).toBeVisible();

    // Click overwrite
    await page.getByRole('button', { name: 'Overwrite' }).click();

    // Dialog should close
    await expect(page.getByText('Configuration Conflict')).not.toBeVisible();

    // Our value should be saved
    await expect(page.getByLabel('Log Level')).toHaveValue('debug');
    await expect(page.getByText('You have unsaved changes')).not.toBeVisible();

    // Verify file has our value
    const savedContent = yaml.parse(fs.readFileSync(configFile, 'utf-8'));
    expect(savedContent.log.level).toBe('debug');
  });

  test('cancel option keeps dialog open', async ({ page, configFile }) => {
    // Make a change
    await page.getByLabel('Log Level').selectOption('debug');

    // Simulate external modification
    const yaml = require('yaml');
    const content = yaml.parse(fs.readFileSync(configFile, 'utf-8'));
    content.log.level = 'warn';
    fs.writeFileSync(configFile, yaml.stringify(content));

    // Try to save, triggering conflict
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.getByText('Configuration Conflict')).toBeVisible();

    // Click cancel
    await page.getByRole('button', { name: 'Cancel' }).click();

    // Dialog should close but changes preserved
    await expect(page.getByText('Configuration Conflict')).not.toBeVisible();
    await expect(page.getByLabel('Log Level')).toHaveValue('debug');
    await expect(page.getByText('You have unsaved changes')).toBeVisible();
  });
});
