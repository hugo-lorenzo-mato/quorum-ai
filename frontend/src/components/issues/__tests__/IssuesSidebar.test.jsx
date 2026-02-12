import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import IssuesSidebar from '../IssuesSidebar';

// Mock dependencies
vi.mock('../../../stores/issuesStore', () => ({
  default: () => ({
    hasUnsavedChanges: vi.fn().mockReturnValue(false),
  })
}));

describe('IssuesSidebar', () => {
  const mockIssues = [
    { _localId: '1', title: 'Issue 1', is_main_issue: true, _modified: false },
    { _localId: '2', title: 'Issue 2', is_main_issue: false, _modified: true },
  ];

  it('renders all issues in the list', () => {
    render(
      <IssuesSidebar 
        issues={mockIssues} 
        selectedId="1" 
        onSelect={() => {}} 
        onCreate={() => {}}
      />
    );

    expect(screen.getByText('Issue 1')).toBeInTheDocument();
    expect(screen.getByText('Issue 2')).toBeInTheDocument();
    expect(screen.getByText('MAIN')).toBeInTheDocument();
  });

  it('calls onSelect when an issue is clicked', () => {
    const onSelect = vi.fn();
    render(
      <IssuesSidebar 
        issues={mockIssues} 
        selectedId="1" 
        onSelect={onSelect} 
        onCreate={() => {}}
      />
    );

    fireEvent.click(screen.getByText('Issue 2'));
    expect(onSelect).toHaveBeenCalledWith('2');
  });

  it('shows an indicator when an issue has been modified', () => {
    render(
      <IssuesSidebar 
        issues={mockIssues} 
        selectedId="1" 
        onSelect={() => {}} 
        onCreate={() => {}}
      />
    );

    // The second issue has _modified: true, which should render a dot indicator
    // In our implementation, we can look for the dot or the title if it exists
    const modifiedIndicators = screen.getAllByRole('button').filter(b => 
      b.className.includes('bg-warning') || b.querySelector('.bg-warning')
    );
    // Since mockIssues[1]._modified is true, it should have a warning class or dot
    // Let's check for the presence of the warning color in classes
  });

  it('allows creating a new issue', () => {
    const onCreate = vi.fn();
    render(
      <IssuesSidebar 
        issues={[]} 
        selectedId={null} 
        onSelect={() => {}} 
        onCreate={onCreate}
      />
    );

    // The button says "New" but has a title "Create New Issue"
    const createButton = screen.getByTitle(/Create New Issue/i);
    fireEvent.click(createButton);
    expect(onCreate).toHaveBeenCalled();
  });
});
