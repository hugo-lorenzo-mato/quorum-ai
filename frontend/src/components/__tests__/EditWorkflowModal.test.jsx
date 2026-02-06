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
});
