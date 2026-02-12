import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import IssuesActionBar from '../IssuesActionBar';

// Mock dependencies
vi.mock('../../../stores', () => ({
  useUIStore: () => ({
    notifySuccess: vi.fn(),
    notifyError: vi.fn(),
  }),
}));

vi.mock('../../../stores/issuesStore', () => ({
  default: () => ({
    updatePublishingProgress: vi.fn(),
    reset: vi.fn(),
    getIssuesForSubmission: vi.fn().mockReturnValue([{ title: 'Test' }]),
    publishingProgress: 0,
    publishingTotal: 0,
    publishingMessage: '',
  })
}));

// Mock API
vi.mock('../../../lib/api', () => ({
  workflowApi: {
    createIssues: vi.fn().mockResolvedValue({ success: true, issues: [] }),
  }
}));

describe('IssuesActionBar', () => {
  it('renders correctly with issue count', () => {
    render(
      <MemoryRouter>
        <IssuesActionBar 
          issueCount={5} 
          hasUnsavedChanges={true} 
          submitting={false} 
          error={null} 
          workflowId="wf-1" 
        />
      </MemoryRouter>
    );

    expect(screen.getByText(/5 Issues ready/i)).toBeInTheDocument();
    expect(screen.getByText(/Create Issues/i)).toBeInTheDocument();
  });

  it('shows submitting state', () => {
    render(
      <MemoryRouter>
        <IssuesActionBar 
          issueCount={5} 
          hasUnsavedChanges={false} 
          submitting={true} 
          error={null} 
          workflowId="wf-1" 
        />
      </MemoryRouter>
    );

    expect(screen.getByText(/Creating/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Creating/i })).toBeDisabled();
  });

  it('shows error message if provided', () => {
    render(
      <MemoryRouter>
        <IssuesActionBar 
          issueCount={5} 
          hasUnsavedChanges={false} 
          submitting={false} 
          error="API failure" 
          workflowId="wf-1" 
        />
      </MemoryRouter>
    );

    expect(screen.getByText(/API failure/i)).toBeInTheDocument();
  });
});
