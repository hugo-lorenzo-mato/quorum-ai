import { render, screen } from '@testing-library/react';
import { ExecutionModeBadge } from '../ExecutionModeBadge';

describe('ExecutionModeBadge', () => {
  it('renders single-agent badge with agent name', () => {
    render(
      <ExecutionModeBadge
        config={{
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        }}
      />
    );

    expect(screen.getByText(/Single Agent/i)).toBeInTheDocument();
    expect(screen.getByText(/Claude/i)).toBeInTheDocument();
  });

  it('renders multi-agent badge for multi_agent mode', () => {
    render(
      <ExecutionModeBadge
        config={{ execution_mode: 'multi_agent' }}
      />
    );

    expect(screen.getByText(/Multi-Agent/i)).toBeInTheDocument();
  });

  it('renders multi-agent badge when config is undefined', () => {
    render(<ExecutionModeBadge config={undefined} />);

    expect(screen.getByText(/Multi-Agent/i)).toBeInTheDocument();
  });

  it('renders multi-agent badge when execution_mode is empty', () => {
    render(<ExecutionModeBadge config={{ execution_mode: '' }} />);

    expect(screen.getByText(/Multi-Agent/i)).toBeInTheDocument();
  });

  it('renders single-agent badge without agent name when not provided', () => {
    render(
      <ExecutionModeBadge
        config={{ execution_mode: 'single_agent' }}
      />
    );

    expect(screen.getByText(/Single Agent/i)).toBeInTheDocument();
    expect(screen.queryByText(/•/)).not.toBeInTheDocument();
  });

  it('renders inline variant correctly for single-agent', () => {
    render(
      <ExecutionModeBadge
        config={{ execution_mode: 'single_agent', single_agent_name: 'gemini' }}
        variant="inline"
      />
    );

    expect(screen.getByText(/Single Agent/i)).toBeInTheDocument();
    expect(screen.getByText(/Gemini/i)).toBeInTheDocument();
  });

  it('renders inline variant correctly for multi-agent', () => {
    render(
      <ExecutionModeBadge
        config={{ execution_mode: 'multi_agent' }}
        variant="inline"
      />
    );

    expect(screen.getByText(/Multi-Agent/i)).toBeInTheDocument();
  });

  it('renders detailed variant correctly for single-agent', () => {
    render(
      <ExecutionModeBadge
        config={{ execution_mode: 'single_agent', single_agent_name: 'codex' }}
        variant="detailed"
      />
    );

    expect(screen.getByText(/Single Agent/i)).toBeInTheDocument();
    expect(screen.getByText(/Codex/i)).toBeInTheDocument();
  });

  it('renders detailed variant correctly for multi-agent', () => {
    render(
      <ExecutionModeBadge
        config={{ execution_mode: 'multi_agent' }}
        variant="detailed"
      />
    );

    expect(screen.getByText(/Multi-Agent Consensus/i)).toBeInTheDocument();
  });

  it('capitalizes agent name correctly', () => {
    render(
      <ExecutionModeBadge
        config={{ execution_mode: 'single_agent', single_agent_name: 'openai' }}
      />
    );

    expect(screen.getByText(/Openai/i)).toBeInTheDocument();
  });
});
