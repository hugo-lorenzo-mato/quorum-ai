import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AgentActivity, { AgentActivityCompact } from '../AgentActivity';

describe('AgentActivity', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns null when there is no activity', () => {
    const { container } = render(
      <AgentActivity activity={[]} activeAgents={[]} expanded={false} />
    );

    expect(container.firstChild).toBeNull();
  });

  it('renders expanded activity details', () => {
    const activity = [
      {
        id: '1',
        agent: 'Codex',
        eventKind: 'started',
        message: 'Booting up',
        timestamp: '2024-01-01T00:00:00Z',
        data: { phase: 'init' },
      },
    ];

    const activeAgents = [
      {
        name: 'Codex',
        status: 'thinking',
        message: 'Working',
        timestamp: '2024-01-01T00:00:05Z',
      },
    ];

    render(
      <AgentActivity
        activity={activity}
        activeAgents={activeAgents}
        expanded
        workflowStartTime="2024-01-01T00:00:00Z"
      />
    );

    expect(screen.getByText('Agent Activity')).toBeInTheDocument();
    expect(screen.getByText('Agent Progress')).toBeInTheDocument();
    expect(screen.getByText('Activity Log')).toBeInTheDocument();
    expect(screen.getByText('Codex')).toBeInTheDocument();
    expect(screen.getByText('started')).toBeInTheDocument();
  });
});

describe('AgentActivityCompact', () => {
  it('shows active agent count', () => {
    const activeAgents = [
      { name: 'Codex', status: 'thinking' },
      { name: 'Gemini', status: 'started' },
    ];

    render(<AgentActivityCompact activeAgents={activeAgents} />);

    expect(screen.getByText('2 active')).toBeInTheDocument();
  });
});
