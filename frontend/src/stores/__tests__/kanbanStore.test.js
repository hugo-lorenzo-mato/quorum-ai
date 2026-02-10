import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.mock('../../lib/api', () => ({
  kanbanApi: {
    getBoard: vi.fn(),
    getEngineState: vi.fn(),
    moveWorkflow: vi.fn(),
    enableEngine: vi.fn(),
    disableEngine: vi.fn(),
    resetCircuitBreaker: vi.fn(),
  },
}));

import useKanbanStore from '../kanbanStore';
import { kanbanApi } from '../../lib/api';

function resetStore() {
  useKanbanStore.setState({
    columns: { refinement: [], todo: [], in_progress: [], to_verify: [], done: [] },
    engine: {
      enabled: false,
      currentWorkflowId: null,
      consecutiveFailures: 0,
      circuitBreakerOpen: false,
      lastFailureAt: null,
    },
    loading: false,
    error: null,
    draggedWorkflow: null,
  });
}

describe('kanbanStore', () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
  });

  it('fetchBoard loads board columns and engine state', async () => {
    kanbanApi.getBoard.mockResolvedValue({
      columns: { refinement: [], todo: [{ id: 'wf-1' }], in_progress: [], to_verify: [], done: [] },
      engine: { enabled: true, currentWorkflowId: null },
    });

    const p = useKanbanStore.getState().fetchBoard();
    expect(useKanbanStore.getState().loading).toBe(true);
    await p;

    expect(useKanbanStore.getState().loading).toBe(false);
    expect(useKanbanStore.getState().columns.todo).toHaveLength(1);
    expect(useKanbanStore.getState().engine.enabled).toBe(true);
  });

  it('moveWorkflow sets error when workflow is not found', async () => {
    const ok = await useKanbanStore.getState().moveWorkflow('missing', 'done', 0);
    expect(ok).toBe(false);
    expect(useKanbanStore.getState().error).toBe('Workflow not found');
  });

  it('moveWorkflow optimistically moves workflow and keeps state on success', async () => {
    useKanbanStore.setState({
      columns: { refinement: [], todo: [{ id: 'wf-1' }], in_progress: [], to_verify: [], done: [] },
    });
    kanbanApi.moveWorkflow.mockResolvedValue({});

    const ok = await useKanbanStore.getState().moveWorkflow('wf-1', 'done', 0);
    expect(ok).toBe(true);
    expect(useKanbanStore.getState().columns.todo).toHaveLength(0);
    expect(useKanbanStore.getState().columns.done[0].id).toBe('wf-1');
    expect(useKanbanStore.getState().columns.done[0].kanban_column).toBe('done');
  });

  it('moveWorkflow reverts on API error', async () => {
    const original = { refinement: [], todo: [{ id: 'wf-1' }], in_progress: [], to_verify: [], done: [] };
    useKanbanStore.setState({ columns: original });
    kanbanApi.moveWorkflow.mockRejectedValue(new Error('boom'));

    const ok = await useKanbanStore.getState().moveWorkflow('wf-1', 'done', 0);
    expect(ok).toBe(false);
    expect(useKanbanStore.getState().columns).toEqual(original);
    expect(useKanbanStore.getState().error).toBe('boom');
  });

  it('enableEngine/disableEngine toggle engine.enabled', async () => {
    kanbanApi.enableEngine.mockResolvedValue({});
    kanbanApi.disableEngine.mockResolvedValue({});

    expect(await useKanbanStore.getState().enableEngine()).toBe(true);
    expect(useKanbanStore.getState().engine.enabled).toBe(true);

    expect(await useKanbanStore.getState().disableEngine()).toBe(true);
    expect(useKanbanStore.getState().engine.enabled).toBe(false);
  });

  it('resetCircuitBreaker closes circuit and resets failure count', async () => {
    useKanbanStore.setState({ engine: { ...useKanbanStore.getState().engine, circuitBreakerOpen: true, consecutiveFailures: 3 } });
    kanbanApi.resetCircuitBreaker.mockResolvedValue({});

    expect(await useKanbanStore.getState().resetCircuitBreaker()).toBe(true);
    expect(useKanbanStore.getState().engine.circuitBreakerOpen).toBe(false);
    expect(useKanbanStore.getState().engine.consecutiveFailures).toBe(0);
  });

  it('handleWorkflowMoved moves workflow to target column', () => {
    useKanbanStore.setState({
      columns: { refinement: [], todo: [{ id: 'wf-1', kanban_column: 'todo' }], in_progress: [], to_verify: [], done: [] },
    });

    useKanbanStore.getState().handleWorkflowMoved({
      workflow_id: 'wf-1',
      to_column: 'done',
      new_position: 0,
    });

    expect(useKanbanStore.getState().columns.todo).toHaveLength(0);
    expect(useKanbanStore.getState().columns.done[0].id).toBe('wf-1');
  });

  it('handleExecutionCompleted stores PR info and clears currentWorkflowId', () => {
    useKanbanStore.setState({
      columns: { refinement: [], todo: [{ id: 'wf-1' }], in_progress: [], to_verify: [], done: [] },
      engine: { ...useKanbanStore.getState().engine, currentWorkflowId: 'wf-1' },
    });

    useKanbanStore.getState().handleExecutionCompleted({
      workflow_id: 'wf-1',
      pr_url: 'u',
      pr_number: 123,
    });

    expect(useKanbanStore.getState().columns.todo[0].pr_number).toBe(123);
    expect(useKanbanStore.getState().engine.currentWorkflowId).toBeNull();
  });

  it('dragged workflow helpers set and clear state', () => {
    useKanbanStore.getState().setDraggedWorkflow({ id: 'wf-1' });
    expect(useKanbanStore.getState().draggedWorkflow.id).toBe('wf-1');
    useKanbanStore.getState().clearDraggedWorkflow();
    expect(useKanbanStore.getState().draggedWorkflow).toBeNull();
  });
});
