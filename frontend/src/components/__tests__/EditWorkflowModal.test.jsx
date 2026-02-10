import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import EditWorkflowModal from '../EditWorkflowModal';

describe('EditWorkflowModal', () => {
  const workflow = { title: 'Old title', prompt: 'Old prompt' };

  it('does not render when closed', () => {
    render(
      <EditWorkflowModal
        isOpen={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
        workflow={workflow}
      />
    );

    expect(screen.queryByText('Edit Workflow')).not.toBeInTheDocument();
  });

  it('saves updates and closes', async () => {
    const onSave = vi.fn().mockResolvedValue();
    const onClose = vi.fn();

    render(
      <EditWorkflowModal
        isOpen
        onClose={onClose}
        onSave={onSave}
        workflow={workflow}
      />
    );

    fireEvent.change(screen.getByLabelText('Prompt'), {
      target: { value: 'New prompt' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith({ prompt: 'New prompt' });
    });

    expect(onClose).toHaveBeenCalled();
  });

  it('closes on Escape', () => {
    const onClose = vi.fn();

    render(
      <EditWorkflowModal
        isOpen
        onClose={onClose}
        onSave={vi.fn()}
        workflow={workflow}
      />
    );

    fireEvent.keyDown(document, { key: 'Escape' });

    expect(onClose).toHaveBeenCalled();
  });

  it('closes when clicking the overlay', () => {
    const onClose = vi.fn();

    render(
      <EditWorkflowModal
        isOpen
        onClose={onClose}
        onSave={vi.fn()}
        workflow={workflow}
      />
    );

    fireEvent.click(screen.getByLabelText('Close modal'));
    expect(onClose).toHaveBeenCalled();
  });

  it('updates execution mode when pending', async () => {
    const onSave = vi.fn().mockResolvedValue();
    const onClose = vi.fn();

    render(
      <EditWorkflowModal
        isOpen
        onClose={onClose}
        onSave={onSave}
        workflow={{
          ...workflow,
          status: 'pending',
          blueprint: { execution_mode: 'multi_agent' },
        }}
      />
    );

    fireEvent.click(screen.getByRole('radio', { name: /Single Agent/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith({
        blueprint: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
          single_agent_model: '',
          single_agent_reasoning_effort: '',
        },
      });
    });

    expect(onClose).toHaveBeenCalled();
  });

  it('updates execution mode to interactive when pending', async () => {
    const onSave = vi.fn().mockResolvedValue();
    const onClose = vi.fn();

    render(
      <EditWorkflowModal
        isOpen
        onClose={onClose}
        onSave={onSave}
        workflow={{
          ...workflow,
          status: 'pending',
          blueprint: { execution_mode: 'multi_agent' },
        }}
      />
    );

    fireEvent.click(screen.getByRole('radio', { name: /Interactive/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith({
        blueprint: { execution_mode: 'interactive' },
      });
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('initializes execution mode from blueprint (interactive)', () => {
    render(
      <EditWorkflowModal
        isOpen
        onClose={vi.fn()}
        onSave={vi.fn()}
        workflow={{
          ...workflow,
          status: 'pending',
          blueprint: { execution_mode: 'interactive' },
        }}
      />
    );

    const interactive = screen.getByRole('radio', { name: /Interactive/i });
    expect(interactive).toBeChecked();
  });

  it('shows validation error for invalid timeout override and does not save', async () => {
    const onSave = vi.fn().mockResolvedValue();
    const onClose = vi.fn();

    render(
      <EditWorkflowModal
        isOpen
        onClose={onClose}
        onSave={onSave}
        workflow={{
          ...workflow,
          status: 'pending',
          blueprint: { execution_mode: 'multi_agent', timeout_seconds: 0 },
        }}
      />
    );

    fireEvent.change(screen.getByPlaceholderText(/e\.g\., 16h/i), {
      target: { value: 'nope' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(await screen.findByText(/Invalid workflow timeout override/i)).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('closes without calling onSave when no changes were made', async () => {
    const onSave = vi.fn().mockResolvedValue();
    const onClose = vi.fn();

    render(
      <EditWorkflowModal
        isOpen
        onClose={onClose}
        onSave={onSave}
        workflow={{
          ...workflow,
          status: 'pending',
          blueprint: { execution_mode: 'multi_agent', timeout_seconds: 0 },
        }}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSave).not.toHaveBeenCalled();
  });
});
