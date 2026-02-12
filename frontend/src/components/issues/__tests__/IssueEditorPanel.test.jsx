import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import IssueEditorPanel from '../IssueEditorPanel';

// Mock monaco editor
vi.mock('@monaco-editor/react', () => ({
  default: ({ value, onChange }) => (
    <textarea 
      data-testid="monaco-mock" 
      value={value} 
      onChange={(e) => onChange(e.target.value)} 
    />
  )
}));

// Mock stores
vi.mock('../../../stores', () => ({
  useUIStore: () => ({
    theme: 'light',
    notifySuccess: vi.fn(),
    notifyError: vi.fn(),
  }),
}));

vi.mock('../../../stores/issuesStore', () => ({
  default: () => ({
    updateIssue: vi.fn(),
    setIssueFilePath: vi.fn(),
    workflowId: 'wf-1',
  })
}));

describe('IssueEditorPanel', () => {
  const mockIssue = {
    _localId: 'local-1',
    title: 'Test Issue',
    body: 'Test Body',
    labels: ['bug'],
    assignees: ['user1'],
    task_id: 'TASK-1'
  };

  it('renders the compact header correctly', () => {
    render(
      <IssueEditorPanel
        issue={mockIssue}
        viewMode="edit"
        onToggleView={() => {}}
        workflowId="wf-1"
      />
    );

    // Title should be in an input now
    const titleInput = screen.getByPlaceholderText(/Issue title.../i);
    expect(titleInput).toBeInTheDocument();
    expect(titleInput.value).toBe('Test Issue');

    // Labels and Assignees should be present
    expect(screen.getByText(/Labels/i)).toBeInTheDocument();
    expect(screen.getByText(/bug/i)).toBeInTheDocument();
    expect(screen.getByText(/Assignees/i)).toBeInTheDocument();
    expect(screen.getByText(/@user1/i)).toBeInTheDocument();
  });

  it('renders the preview mode correctly', () => {
    render(
      <IssueEditorPanel
        issue={mockIssue}
        viewMode="preview"
        onToggleView={() => {}}
        workflowId="wf-1"
      />
    );

    // In preview, title is an h1
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Test Issue');
    expect(screen.getByText('Test Body')).toBeInTheDocument();
  });

  it('switches between edit and preview via props', () => {
    const { rerender } = render(
      <IssueEditorPanel
        issue={mockIssue}
        viewMode="edit"
        onToggleView={() => {}}
        workflowId="wf-1"
      />
    );

    expect(screen.getByTestId('monaco-mock')).toBeInTheDocument();

    rerender(
      <IssueEditorPanel
        issue={mockIssue}
        viewMode="preview"
        onToggleView={() => {}}
        workflowId="wf-1"
      />
    );

    expect(screen.queryByTestId('monaco-mock')).not.toBeInTheDocument();
    expect(screen.getByText('Test Body')).toBeInTheDocument();
  });
});
