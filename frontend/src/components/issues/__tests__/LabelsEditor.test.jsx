import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import LabelsEditor from '../LabelsEditor';

describe('LabelsEditor', () => {
  it('renders existing labels', () => {
    const labels = ['bug', 'feature'];
    render(<LabelsEditor labels={labels} onChange={() => {}} />);
    
    expect(screen.getByText('bug')).toBeInTheDocument();
    expect(screen.getByText('feature')).toBeInTheDocument();
  });

  it('calls onChange when a label is removed', () => {
    const labels = ['bug'];
    const onChange = vi.fn();
    render(<LabelsEditor labels={labels} onChange={onChange} />);
    
    const removeButton = screen.getByLabelText(/Remove bug/i);
    fireEvent.click(removeButton);
    
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it('allows adding a new label', () => {
    const onChange = vi.fn();
    render(<LabelsEditor labels={[]} onChange={onChange} />);
    
    // Click add button
    const addButton = screen.getByRole('button', { name: /Add/i });
    fireEvent.click(addButton);
    
    // Type and submit
    const input = screen.getByPlaceholderText(/Name.../i);
    fireEvent.change(input, { target: { value: 'priority' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });
    
    expect(onChange).toHaveBeenCalledWith(['priority']);
  });

  it('renders in compact mode correctly', () => {
    const { container } = render(<LabelsEditor labels={['test']} onChange={() => {}} compact />);
    // The container should have the flex items-center class instead of space-y-2
    expect(container.firstChild).toHaveClass('flex');
    expect(container.firstChild).toHaveClass('items-center');
  });
});
