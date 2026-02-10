import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

const configMocks = vi.hoisted(() => {
  const fieldMap = new Map();
  const selectMap = new Map();
  const setField = vi.fn();
  const resetToDefaults = vi.fn();

  function getField(key) {
    if (!fieldMap.has(key)) {
      fieldMap.set(key, { value: false, onChange: vi.fn(), error: '', disabled: false });
    }
    return fieldMap.get(key);
  }

  function getSelect(key) {
    if (!selectMap.has(key)) {
      selectMap.set(key, { value: '', onChange: vi.fn(), error: '', disabled: false, options: [] });
    }
    return selectMap.get(key);
  }

  return {
    getField,
    getSelect,
    setField,
    resetToDefaults,
    config: {
      phases: { plan: { timeout: '1h' }, execute: { timeout: '2h' } },
      trace: {},
      issues: { enabled: true },
      git: {},
      workflow: {},
      state: {},
    },
    enums: {},
  };
});

vi.mock('../../../../hooks/useConfigField', () => ({
  useConfigField: (key) => configMocks.getField(key),
  useConfigSelect: (key) => configMocks.getSelect(key),
}));

vi.mock('../../../../stores/configStore', () => ({
  useConfigStore: (selector) => selector({
    config: configMocks.config,
    enums: configMocks.enums,
    setField: configMocks.setField,
    resetToDefaults: configMocks.resetToDefaults,
    isLoading: false,
  }),
}));

vi.mock('../../../../lib/agents', () => ({
  getModelsForAgent: () => [],
  getReasoningLevels: () => [],
  useEnums: () => {},
}));

vi.mock('../../index', () => ({
  SettingSection: ({ title, children }) => (
    <section>
      <h2>{title}</h2>
      {children}
    </section>
  ),
  TextInputSetting: ({ label }) => <div>{label}</div>,
  SelectSetting: ({ label }) => <div>{label}</div>,
  ToggleSetting: ({ label }) => <div>{label}</div>,
  DurationInputSetting: ({ label }) => <div>{label}</div>,
  NumberInputSetting: ({ label }) => <div>{label}</div>,
  ArrayInputSetting: ({ label }) => <div>{label}</div>,
  ConfirmDialog: () => null,
}));

vi.mock('../../AnalyzePhaseCard', () => ({
  AnalyzePhaseCard: () => <div>AnalyzePhaseCard</div>,
}));

import { AdvancedTab } from '../AdvancedTab';
import { GitTab } from '../GitTab';
import { IssuesTab } from '../IssuesTab';
import { PhasesTab } from '../PhasesTab';
import { WorkflowTab } from '../WorkflowTab';

describe('Config Tabs Smoke', () => {
  it('renders IssuesTab', () => {
    render(<IssuesTab />);
    expect(screen.getByText('Issue Generation')).toBeTruthy();
  });

  it('renders GitTab', () => {
    render(<GitTab />);
    expect(screen.getByText('Worktree Management')).toBeTruthy();
  });

  it('renders AdvancedTab', () => {
    render(<AdvancedTab />);
    expect(screen.getByText('Trace Logging')).toBeTruthy();
  });

  it('renders WorkflowTab', () => {
    render(<WorkflowTab />);
    expect(screen.getByText('Workflow Execution')).toBeTruthy();
  });

  it('renders PhasesTab', () => {
    render(<PhasesTab />);
    expect(screen.getByText('Workflow Phases')).toBeTruthy();
  });
});

