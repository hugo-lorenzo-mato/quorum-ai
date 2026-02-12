import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import GenerationLoadingOverlay from '../GenerationLoadingOverlay';

// Mock the store
vi.mock('../../../stores', () => ({
  useAgentStore: vi.fn((selector) => selector({
    agentActivity: {
      'wf-1': [
        { id: '1', agent: 'Claude', eventKind: 'thinking', message: 'Analyzing code...', timestamp: new Date().toISOString() }
      ]
    },
    currentAgents: {
      'wf-1': {
        'Claude': { status: 'thinking', name: 'Claude' }
      }
    }
  })),
  // Need to mock Button because it might be used
  Button: ({ children, onClick, className }) => <button onClick={onClick} className={className}>{children}</button>,
}));

// Mock Logo component
vi.mock('../../Logo', () => ({
  default: () => <div data-testid="mock-logo">Logo</div>
}));

describe('GenerationLoadingOverlay', () => {
  it('renders the orchestrator title and progress', () => {
    render(
      <GenerationLoadingOverlay
        workflowId="wf-1"
        progress={2}
        total={10}
        generatedIssues={[]}
        onCancel={() => {}}
      />
    );

    expect(screen.getByText(/Orchestrator/)).toBeInTheDocument();
    expect(screen.getByText(/2 \/ 10 Issues/)).toBeInTheDocument();
  });

  it('shows activity from the store', () => {
    render(
      <GenerationLoadingOverlay
        workflowId="wf-1"
        progress={0}
        total={5}
        generatedIssues={[]}
        onCancel={() => {}}
      />
    );

    expect(screen.getByText('Analyzing code...')).toBeInTheDocument();
    expect(screen.getAllByText(/CLAUDE/i).length).toBeGreaterThan(0);
    expect(screen.getByText('thinking')).toBeInTheDocument();
  });

  it('renders generated issues in the pipeline', () => {
    const generatedIssues = [
      { title: 'Security Fix', is_main_issue: true, file_path: 'fix.md' }
    ];

    render(
      <GenerationLoadingOverlay
        workflowId="wf-1"
        progress={1}
        total={5}
        generatedIssues={generatedIssues}
        onCancel={() => {}}
      />
    );

    expect(screen.getByText('Security Fix')).toBeInTheDocument();
    expect(screen.getByText('MAIN')).toBeInTheDocument();
  });

  it('shows stage name based on progress', () => {
    const { rerender } = render(
      <GenerationLoadingOverlay
        workflowId="wf-1"
        progress={0}
        total={5}
        generatedIssues={[]}
        onCancel={() => {}}
      />
    );

    expect(screen.getByText('Context Analysis & Planning')).toBeInTheDocument();

    rerender(
      <GenerationLoadingOverlay
        workflowId="wf-1"
        progress={1}
        total={5}
        generatedIssues={[{}]}
        onCancel={() => {}}
      />
    );

    expect(screen.getByText('Issue Synthesis & Drafting')).toBeInTheDocument();
  });
});
