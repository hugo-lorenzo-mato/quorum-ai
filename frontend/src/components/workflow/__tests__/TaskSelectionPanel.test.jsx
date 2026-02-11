import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import TaskSelectionPanel from '../TaskSelectionPanel';

function Harness({ tasks, initialSelected }) {
  const [selected, setSelected] = useState(initialSelected);
  return (
    <TaskSelectionPanel
      tasks={tasks}
      selectedTaskIds={selected}
      onChangeSelectedTaskIds={setSelected}
    />
  );
}

describe('TaskSelectionPanel', () => {
  const tasks = [
    { id: 't1', name: 't1', cli: 'claude', dependencies: [] },
    { id: 't2', name: 't2', cli: 'claude', dependencies: ['t1'] },
    { id: 't3', name: 't3', cli: 'gemini', dependencies: [] },
  ];

  it('defaults to the provided selection', () => {
    render(<Harness tasks={tasks} initialSelected={['t1', 't2', 't3']} />);
    expect(screen.getByText(/3\/3 selected/i)).toBeInTheDocument();
    expect(screen.getByLabelText('Select task t1')).toBeChecked();
    expect(screen.getByLabelText('Select task t2')).toBeChecked();
    expect(screen.getByLabelText('Select task t3')).toBeChecked();
  });

  it('auto-includes dependencies as required and disables them', async () => {
    const user = userEvent.setup();
    render(<Harness tasks={tasks} initialSelected={['t2']} />);

    const t1 = screen.getByLabelText('Select task t1');
    const t2 = screen.getByLabelText('Select task t2');
    const t3 = screen.getByLabelText('Select task t3');

    expect(t1).toBeChecked();
    expect(t1).toBeDisabled();
    expect(t2).toBeChecked();
    expect(t2).not.toBeDisabled();
    expect(t3).not.toBeChecked();

    // Unselecting t2 should release the dependency and leave nothing selected.
    await user.click(t2);
    expect(t2).not.toBeChecked();
    expect(t1).not.toBeChecked();
    expect(t3).not.toBeChecked();
  });

  it('select all / clear buttons update selection', async () => {
    const user = userEvent.setup();
    render(<Harness tasks={tasks} initialSelected={['t2']} />);

    await user.click(screen.getByRole('button', { name: /select all/i }));
    expect(screen.getByText(/3\/3 selected/i)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /clear/i }));
    expect(screen.getByText(/0\/3 selected/i)).toBeInTheDocument();
  });
});

