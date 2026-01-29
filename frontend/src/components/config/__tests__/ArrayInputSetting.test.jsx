import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { ArrayInputSetting } from '../ArrayInputSetting';

const defaultProps = {
  label: 'Denied Tools',
  value: ['Bash', 'Write'],
  onChange: vi.fn(),
};

describe('ArrayInputSetting', () => {
  it('renders existing items', () => {
    render(<ArrayInputSetting {...defaultProps} />);

    expect(screen.getByText('Bash')).toBeInTheDocument();
    expect(screen.getByText('Write')).toBeInTheDocument();
  });

  it('adds new item', async () => {
    const onChange = vi.fn();
    render(<ArrayInputSetting {...defaultProps} onChange={onChange} />);

    const input = screen.getByRole('textbox');
    await userEvent.type(input, 'Edit{Enter}');

    expect(onChange).toHaveBeenCalledWith(['Bash', 'Write', 'Edit']);
  });

  it('removes item when clicking remove button', async () => {
    const onChange = vi.fn();
    render(<ArrayInputSetting {...defaultProps} onChange={onChange} />);

    // Find all remove buttons and click the first one (for 'Bash')
    const removeButtons = screen.getAllByRole('button', { name: /remove|delete|x/i });
    if (removeButtons.length > 0) {
      await userEvent.click(removeButtons[0]);
      expect(onChange).toHaveBeenCalledWith(['Write']);
    }
  });

  it('prevents duplicates', async () => {
    const onChange = vi.fn();
    render(<ArrayInputSetting {...defaultProps} onChange={onChange} />);

    const input = screen.getByRole('textbox');
    await userEvent.type(input, 'Bash{Enter}');

    // Should not add duplicate
    expect(onChange).not.toHaveBeenCalled();
  });
});
