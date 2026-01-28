import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import ToggleSetting from '../ToggleSetting';

const defaultProps = {
  label: 'Enable Feature',
  checked: false,
  onChange: vi.fn(),
};

describe('ToggleSetting', () => {
  it('renders label and description', () => {
    render(
      <ToggleSetting
        {...defaultProps}
        description="Turn on this feature"
      />
    );

    expect(screen.getByText('Enable Feature')).toBeInTheDocument();
    expect(screen.getByText('Turn on this feature')).toBeInTheDocument();
  });

  it('reflects checked state', () => {
    const { rerender } = render(<ToggleSetting {...defaultProps} checked={false} />);
    const toggle = screen.getByRole('switch');
    expect(toggle).not.toBeChecked();

    rerender(<ToggleSetting {...defaultProps} checked={true} />);
    expect(toggle).toBeChecked();
  });

  it('calls onChange when clicked', async () => {
    const onChange = vi.fn();
    render(<ToggleSetting {...defaultProps} onChange={onChange} />);

    await userEvent.click(screen.getByRole('switch'));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('shows helper text', () => {
    render(<ToggleSetting {...defaultProps} helperText="Requires admin access" />);
    expect(screen.getByText('Requires admin access')).toBeInTheDocument();
  });

  it('disables interaction when disabled', async () => {
    const onChange = vi.fn();
    render(<ToggleSetting {...defaultProps} onChange={onChange} disabled />);

    const toggle = screen.getByRole('switch');
    expect(toggle).toBeDisabled();
  });
});
