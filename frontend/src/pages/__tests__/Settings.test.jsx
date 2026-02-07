import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import Settings from '../Settings';
import { useConfigStore } from '../../stores/configStore';

vi.mock('../../stores/configStore', () => ({
  useConfigStore: vi.fn(),
  getAtPath: (obj, path) => path.split('.').reduce((curr, key) => curr?.[key], obj),
}));

const mockConfig = {
  log: { level: 'info', format: 'auto' },
  chat: { timeout: '3m', progress_interval: '15s', editor: 'vim' },
  report: { enabled: true, base_dir: '.quorum/runs', use_utc: true, include_raw: true },
  workflow: { timeout: '30m', max_retries: 2 },
  agents: {
    claude: { enabled: true, model: 'claude-sonnet-4-20250514' },
    codex: { enabled: false },
    gemini: { enabled: false },
    copilot: { enabled: false },
  },
  phases: {},
  git: { auto_commit: true },
  trace: { enabled: false },
  server: { enabled: true, port: 8080 },
};

describe('Settings', () => {
  const renderWithRouter = (ui) => render(<MemoryRouter>{ui}</MemoryRouter>);

  beforeEach(() => {
    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: mockConfig,
        localChanges: {},
        isLoading: false,
        isSaving: false,
        error: null,
        hasConflict: false,
        conflictConfig: null,
        validationErrors: {},
        isDirty: false,
        enums: { log_levels: ['debug', 'info', 'warn', 'error'] },
        loadConfig: vi.fn(),
        loadMetadata: vi.fn(),
        saveChanges: vi.fn(),
        discardChanges: vi.fn(),
        setField: vi.fn(),
        getFieldValue: vi.fn(),
        isFieldDirty: vi.fn(() => false),
      };
      return selector ? selector(state) : state;
    });
  });

  it('renders page header', () => {
    renderWithRouter(<Settings />);

    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText(/Manage configuration for the selected project/i)).toBeInTheDocument();
  });

  it('renders all tabs', () => {
    renderWithRouter(<Settings />);

    expect(screen.getAllByText('General').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Workflow Defaults').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Agents & Models').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Execution Phases').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Git Integration').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Issue Generation').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Advanced').length).toBeGreaterThan(0);
  });

  it('loads config on mount', () => {
    const loadConfig = vi.fn();
    const loadMetadata = vi.fn();

    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: mockConfig,
        localChanges: {},
        isLoading: false,
        error: null,
        hasConflict: false,
        validationErrors: {},
        isDirty: false,
        loadConfig,
        loadMetadata,
      };
      return selector ? selector(state) : state;
    });

    renderWithRouter(<Settings />);

    expect(loadConfig).toHaveBeenCalled();
    expect(loadMetadata).toHaveBeenCalled();
  });

  it('shows loading state', () => {
    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: null,
        localChanges: {},
        isLoading: true,
        error: null,
        hasConflict: false,
        validationErrors: {},
        isDirty: false,
        loadConfig: vi.fn(),
        loadMetadata: vi.fn(),
      };
      return selector ? selector(state) : state;
    });

    renderWithRouter(<Settings />);

    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows error state with retry', async () => {
    const loadConfig = vi.fn();

    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: null,
        localChanges: {},
        isLoading: false,
        error: 'Failed to load configuration',
        hasConflict: false,
        validationErrors: {},
        isDirty: false,
        loadConfig,
        loadMetadata: vi.fn(),
      };
      return selector ? selector(state) : state;
    });

    renderWithRouter(<Settings />);

    expect(screen.getByText('Failed to load configuration')).toBeInTheDocument();
    expect(screen.getByText('Retry')).toBeInTheDocument();

    await userEvent.click(screen.getByText('Retry'));
    expect(loadConfig).toHaveBeenCalled();
  });

  it('shows save toolbar when changes exist', () => {
    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: mockConfig,
        localChanges: { log: { level: 'debug' } },
        isLoading: false,
        isSaving: false,
        error: null,
        hasConflict: false,
        validationErrors: {},
        isDirty: true,
        loadConfig: vi.fn(),
        loadMetadata: vi.fn(),
        saveChanges: vi.fn(),
        discardChanges: vi.fn(),
      };
      return selector ? selector(state) : state;
    });

    renderWithRouter(<Settings />);

    expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    expect(screen.getByText('Save Changes')).toBeInTheDocument();
    expect(screen.getByText('Discard')).toBeInTheDocument();
  });

  it('hides save toolbar when no changes', () => {
    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: mockConfig,
        localChanges: {},
        isLoading: false,
        error: null,
        hasConflict: false,
        validationErrors: {},
        isDirty: false,
        loadConfig: vi.fn(),
        loadMetadata: vi.fn(),
      };
      return selector ? selector(state) : state;
    });

    renderWithRouter(<Settings />);

    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
  });

  it('shows conflict dialog', () => {
    useConfigStore.mockImplementation((selector) => {
      const state = {
        config: mockConfig,
        localChanges: {},
        isLoading: false,
        error: null,
        hasConflict: true,
        conflictConfig: { log: { level: 'warn' } },
        conflictEtag: '"new-etag"',
        validationErrors: {},
        isDirty: false,
        loadConfig: vi.fn(),
        loadMetadata: vi.fn(),
        acceptServerVersion: vi.fn(),
        forceSave: vi.fn(),
      };
      return selector ? selector(state) : state;
    });

    renderWithRouter(<Settings />);

    expect(screen.getByText('Configuration Conflict')).toBeInTheDocument();
    expect(screen.getByText('Reload Latest')).toBeInTheDocument();
    expect(screen.getByText('Overwrite')).toBeInTheDocument();
  });
});
