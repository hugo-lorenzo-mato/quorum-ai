import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import GeneralTab from '../GeneralTab';
import { useConfigStore } from '../../../../stores/configStore';

// Mock the config store
vi.mock('../../../../stores/configStore', () => ({
  useConfigStore: vi.fn(),
  getAtPath: (obj, path) => path.split('.').reduce((curr, key) => curr?.[key], obj),
}));

describe('GeneralTab', () => {
  beforeEach(() => {
    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: {
          log: { level: 'info', format: 'auto' },
          chat: { timeout: '3m', progress_interval: '15s', editor: 'vim' },
          report: { enabled: true, base_dir: '.quorum/runs', use_utc: true, include_raw: true },
        },
        localChanges: {},
        enums: {
          log_levels: ['debug', 'info', 'warn', 'error'],
          log_formats: ['auto', 'text', 'json'],
        },
        setField: vi.fn(),
        getFieldValue: vi.fn((path) => {
          const config = {
            log: { level: 'info', format: 'auto' },
            chat: { timeout: '3m', progress_interval: '15s', editor: 'vim' },
            report: { enabled: true, base_dir: '.quorum/runs', use_utc: true, include_raw: true },
          };
          return path.split('.').reduce((obj, key) => obj?.[key], config);
        }),
        isFieldDirty: vi.fn(() => false),
        validationErrors: {},
      };
      return selector ? selector(state) : state;
    });
  });

  it('renders logging section', () => {
    render(<GeneralTab />);
    expect(screen.getByRole('heading', { level: 3, name: 'Logging' })).toBeInTheDocument();
  });

  it('renders log level setting', () => {
    render(<GeneralTab />);
    expect(screen.getByText(/log level/i)).toBeInTheDocument();
  });

  it('renders report section', () => {
    render(<GeneralTab />);
    expect(screen.getByRole('heading', { level: 3, name: 'Report Generation' })).toBeInTheDocument();
  });
});
