import { render, screen, fireEvent } from '@testing-library/react';
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
    expect(screen.getAllByText('Codex').length).toBeGreaterThan(0);
    expect(screen.getByText('started')).toBeInTheDocument();
  });

  it('ActivityEntry expands to show tool details on click', () => {
    const activity = [
      {
        id: '1',
        agent: 'Claude',
        eventKind: 'tool_use',
        message: 'Using tool: Bash',
        timestamp: '2024-01-01T00:00:00Z',
        data: {
          tool: 'Bash',
          args: { command: 'git status' },
        },
      },
    ];

    render(
      <AgentActivity activity={activity} activeAgents={[]} expanded />
    );

    // Should show chevron indicator
    expect(screen.getByTestId('expand-chevron')).toBeInTheDocument();

    // Detail panel should not be visible initially
    expect(screen.queryByTestId('entry-detail')).not.toBeInTheDocument();

    // Click to expand
    fireEvent.click(screen.getByTestId('activity-entry'));

    // Detail panel should now be visible
    expect(screen.getByTestId('entry-detail')).toBeInTheDocument();
    expect(screen.getAllByText('Bash').length).toBeGreaterThan(0);
    expect(screen.getByText('Tool')).toBeInTheDocument();
    expect(screen.getByText('Args')).toBeInTheDocument();
  });

  it('shows token usage for completed agents in progress bar', () => {
    const activeAgents = [
      {
        name: 'Codex',
        status: 'completed',
        message: 'Completed',
        timestamp: '2024-01-01T00:00:00Z',
        startedAt: '2024-01-01T00:00:00Z',
        completedAt: '2024-01-01T00:00:30Z',
        durationMs: 30000,
        data: { tokens_in: 1500, tokens_out: 800 },
      },
    ];

    render(
      <AgentActivity activity={[]} activeAgents={activeAgents} expanded />
    );

    expect(screen.getByTestId('progress-tokens')).toBeInTheDocument();
    expect(screen.getByTestId('progress-tokens').textContent).toContain('1.5K');
    expect(screen.getByTestId('progress-tokens').textContent).toContain('800');
  });

  it('shows exit code badge in expanded entry', () => {
    const activity = [
      {
        id: '1',
        agent: 'Codex',
        eventKind: 'progress',
        message: 'Command completed',
        timestamp: '2024-01-01T00:00:00Z',
        data: { command: 'ls -la', exit_code: 0 },
      },
    ];

    render(
      <AgentActivity activity={activity} activeAgents={[]} expanded />
    );

    // Click to expand
    fireEvent.click(screen.getByTestId('activity-entry'));

    expect(screen.getByTestId('exit-code-badge')).toBeInTheDocument();
    expect(screen.getByTestId('exit-code-badge').textContent).toBe('0');
    expect(screen.getByTestId('exit-code-badge').className).toContain('text-success');
  });

  it('shows error exit code badge for non-zero exit', () => {
    const activity = [
      {
        id: '1',
        agent: 'Codex',
        eventKind: 'progress',
        message: 'Command completed',
        timestamp: '2024-01-01T00:00:00Z',
        data: { command: 'false', exit_code: 1 },
      },
    ];

    render(
      <AgentActivity activity={activity} activeAgents={[]} expanded />
    );

    fireEvent.click(screen.getByTestId('activity-entry'));

    expect(screen.getByTestId('exit-code-badge').textContent).toBe('1');
    expect(screen.getByTestId('exit-code-badge').className).toContain('text-error');
  });

  it('handles missing data fields gracefully', () => {
    const activity = [
      {
        id: '1',
        agent: 'Claude',
        eventKind: 'started',
        message: 'Initialized',
        timestamp: '2024-01-01T00:00:00Z',
      },
      {
        id: '2',
        agent: 'Claude',
        eventKind: 'progress',
        message: 'Processing',
        timestamp: '2024-01-01T00:00:01Z',
        data: null,
      },
    ];

    // Should not throw
    render(
      <AgentActivity activity={activity} activeAgents={[]} expanded />
    );

    expect(screen.getAllByText('Initialized').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Processing').length).toBeGreaterThan(0);
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
