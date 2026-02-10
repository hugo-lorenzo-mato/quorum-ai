import { test as base } from '@playwright/test';
import fs from 'fs';
import path from 'path';
import YAML from 'yaml';

// Default test configuration
export const defaultConfig = {
  log: {
    level: 'info',
    format: 'auto',
  },
  trace: {
    enabled: false,
    path: '.quorum/traces',
    max_size: 100,
    max_age: 7,
    max_backups: 3,
    compress: true,
  },
  chat: {
    timeout: '3m',
    progress_interval: '15s',
    editor: 'vim',
  },
  report: {
    enabled: true,
    base_dir: '.quorum/runs',
    use_utc: true,
    include_raw: true,
  },
  workflow: {
    timeout: '30m',
    max_retries: 2,
    dry_run: false,
    deny_tools: [],
  },
  state: {
    backend: 'sqlite',
    path: '.quorum/state.db',
    lock_ttl: '5m',
  },
  agents: {
    claude: {
      enabled: true,
      model: 'claude-sonnet-4-20250514',
      max_tokens: 16000,
      temperature: 0.7,
      timeout: '5m',
      max_retries: 3,
    },
    codex: {
      enabled: false,
      model: 'gpt-4o',
      max_tokens: 16000,
      temperature: 0.7,
      timeout: '5m',
      max_retries: 3,
    },
    gemini: {
      enabled: false,
      model: 'gemini-2.0-flash',
      max_tokens: 16000,
      temperature: 0.7,
      timeout: '5m',
      max_retries: 3,
    },
    copilot: {
      enabled: false,
      model: 'gpt-4o',
      max_tokens: 16000,
      temperature: 0.7,
      timeout: '5m',
      max_retries: 3,
    },
  },
  phases: {
    analyze: { single_agent: 'claude' },
    plan: { single_agent: 'claude' },
    execute: { moderator: 'claude', participants: ['claude', 'codex'], rounds: 3 },
    review: { single_agent: 'claude' },
  },
  git: {
    auto_commit: true,
    auto_push: false,
    auto_pr: false,
    auto_merge: false,
    commit_prefix: '[quorum]',
    branch_prefix: 'quorum/',
  },
  github: {
    owner: '',
    repo: '',
  },
  server: {
    enabled: true,
    port: 8080,
    host: '127.0.0.1',
  },
};

// Extend base test with config fixture
export const test = base.extend<{
  configFile: string;
  resetConfig: () => Promise<void>;
}>({
  configFile: async (_args, use) => {
    const configDir = path.join(process.cwd(), '.test-configs');
    fs.mkdirSync(configDir, { recursive: true });

    const configFile = path.join(configDir, `config-${Date.now()}.yaml`);

    await use(configFile);

    // Cleanup
    try {
      fs.unlinkSync(configFile);
    } catch {}
  },

  resetConfig: async ({ configFile }, use) => {
    const reset = async () => {
      const yaml = require('yaml');
      fs.writeFileSync(configFile, yaml.stringify(defaultConfig));
    };

    await reset();
    await use(reset);
  },
});

export { expect } from '@playwright/test';
