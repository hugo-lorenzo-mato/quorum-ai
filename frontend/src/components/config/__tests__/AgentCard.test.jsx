import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import PropTypes from 'prop-types';

const fieldState = vi.hoisted(() => {
  const values = new Map();
  const onChange = new Map();

  function ensure(key) {
    if (!onChange.has(key)) onChange.set(key, vi.fn());
    if (!values.has(key)) values.set(key, undefined);
  }

  return {
    values,
    onChange,
    set(key, val) {
      ensure(key);
      values.set(key, val);
    },
    get(key) {
      ensure(key);
      return {
        get value() {
          return values.get(key);
        },
        onChange: onChange.get(key),
        error: '',
        disabled: false,
      };
    },
    reset() {
      values.clear();
      onChange.clear();
    },
  };
});

vi.mock('../../../hooks/useConfigField', () => ({
  useConfigField: (key) => fieldState.get(key),
}));

vi.mock('../../../stores/configStore', () => ({
  useConfigStore: (selector) => selector({
    enums: { phase_model_keys: ['refine', 'analyze', 'plan', 'execute'] },
  }),
}));

vi.mock('../../../lib/agents', () => ({
  AGENT_INFO: { claude: { name: 'Claude', description: 'Desc' } },
  PHASE_MODEL_KEYS: ['refine', 'analyze', 'plan', 'execute'],
  PHASE_LABELS: { refine: 'Refine', analyze: 'Analyze', plan: 'Plan', execute: 'Execute' },
  supportsReasoning: () => true,
  getModelsForAgent: () => [{ value: 'm1', label: 'm1' }],
  getReasoningLevels: () => [{ value: 'low', label: 'Low' }],
  useEnums: () => {},
}));

vi.mock('../index', () => ({
  TextInputSetting: Object.assign(({ label }) => <div>{label}</div>, {
    propTypes: { label: PropTypes.string },
  }),
  SelectSetting: Object.assign(({ label }) => <div>{label}</div>, {
    propTypes: { label: PropTypes.string },
  }),
  ToggleSetting: Object.assign(({ label, checked, onChange }) => (
    <button type="button" onClick={() => onChange(!checked)}>
      {label}
    </button>
  ), {
    propTypes: {
      label: PropTypes.string,
      checked: PropTypes.bool,
      onChange: PropTypes.func,
    },
  }),
}));

import { AgentCard } from '../AgentCard';

describe('AgentCard', () => {
  beforeEach(() => {
    fieldState.reset();
    vi.clearAllMocks();
  });

  it('enabling an agent with no phases defaults to all phases', () => {
    fieldState.set('agents.claude.enabled', false);
    fieldState.set('agents.claude.phases', {});

    render(<AgentCard agentKey="claude" />);

    fireEvent.click(screen.getByText('Enable Claude'));

    // enabled.onChange called with true
    expect(fieldState.onChange.get('agents.claude.enabled')).toHaveBeenCalledWith(true);
    // phases.onChange called with all phases enabled
    expect(fieldState.onChange.get('agents.claude.phases')).toHaveBeenCalledWith({
      refine: true,
      analyze: true,
      plan: true,
      execute: true,
    });
  });

  it('phase toggles update allowlist and prevent disabling the last phase', () => {
    fieldState.set('agents.claude.enabled', true);
    fieldState.set('agents.claude.phases', { refine: true, analyze: true });

    const { unmount } = render(<AgentCard agentKey="claude" />);

    // Toggle off one phase
    fireEvent.click(screen.getByText('Refine'));
    expect(fieldState.onChange.get('agents.claude.phases')).toHaveBeenCalled();

    unmount();

    // Try to toggle off the last remaining phase; handler should early-return (no onChange call for empty set)
    fieldState.onChange.get('agents.claude.phases').mockClear();
    fieldState.set('agents.claude.phases', { analyze: true });
    render(<AgentCard agentKey="claude" />);
    fireEvent.click(screen.getByText('Analyze'));
    expect(fieldState.onChange.get('agents.claude.phases')).not.toHaveBeenCalled();
  });
});
