import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { SelectSetting } from '../SelectSetting';

const defaultProps = {
  label: 'Log Level',
  value: 'info',
  onChange: vi.fn(),
  options: [
    { value: 'debug', label: 'Debug' },
    { value: 'info', label: 'Info' },
    { value: 'warn', label: 'Warn' },
    { value: 'error', label: 'Error' },
  ],
};

describe('SelectSetting', () => {
  it('renders label', () => {
    render(<SelectSetting {...defaultProps} />);
    expect(screen.getByText('Log Level')).toBeInTheDocument();
  });

  it('shows current value', () => {
    render(<SelectSetting {...defaultProps} />);
    expect(screen.getByRole('combobox')).toHaveValue('info');
  });

  it('calls onChange when selection changes', async () => {
    const onChange = vi.fn();
    render(<SelectSetting {...defaultProps} onChange={onChange} />);

    await userEvent.selectOptions(screen.getByRole('combobox'), 'debug');
    expect(onChange).toHaveBeenCalledWith('debug');
  });

  it('shows error message', () => {
    render(<SelectSetting {...defaultProps} error="Invalid selection" />);
    expect(screen.getByText('Invalid selection')).toBeInTheDocument();
  });

  it('disables when disabled prop is true', () => {
    render(<SelectSetting {...defaultProps} disabled />);
    expect(screen.getByRole('combobox')).toBeDisabled();
  });
});
