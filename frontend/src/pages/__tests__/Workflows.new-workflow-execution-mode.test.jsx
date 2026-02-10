import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import Workflows from '../Workflows';

const mockNavigate = vi.fn();

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockCreateWorkflow = vi.fn();
const mockFetchWorkflows = vi.fn();

vi.mock('../../stores', () => ({
  useWorkflowStore: Object.assign(
    () => ({
      workflows: [],
      loading: false,
      fetchWorkflows: mockFetchWorkflows,
      fetchWorkflow: vi.fn(),
      createWorkflow: mockCreateWorkflow,
      deleteWorkflow: vi.fn(),
      clearError: vi.fn(),
    }),
    {
      getState: () => ({ error: null }),
      setState: vi.fn(),
    }
  ),
  useTaskStore: () => ({
    getTasksForWorkflow: () => [],
    setTasks: vi.fn(),
  }),
  useUIStore: (selector) => selector({
    notifyInfo: vi.fn(),
    notifyError: vi.fn(),
    connectionMode: 'sse',
  }),
  useExecutionStore: () => ({}),
  useProjectStore: () => ({}),
  useConfigStore: () => ({
    config: {
      agents: {
        claude: { enabled: true },
        codex: { enabled: true },
        gemini: { enabled: true },
        copilot: { enabled: true },
      },
    },
  }),
}));

vi.mock('../../lib/agents', async () => {
  const actual = await vi.importActual('../../lib/agents');
  return {
    ...actual,
    useEnums: () => undefined,
  };
});

describe('Workflows (new workflow execution mode)', () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockCreateWorkflow.mockReset();
    mockFetchWorkflows.mockReset();
  });

  const renderAtNewWorkflow = () => render(
    <MemoryRouter initialEntries={['/workflows/new']}>
      <Routes>
        <Route path="/workflows/:id" element={<Workflows />} />
      </Routes>
    </MemoryRouter>
  );

  it('allows selecting Interactive mode when creating a workflow', async () => {
    mockCreateWorkflow.mockResolvedValue({ id: 'wf-1' });

    renderAtNewWorkflow();

    await userEvent.type(
      screen.getByPlaceholderText(/Describe what you want the AI agents to accomplish/i),
      'hello'
    );

    await userEvent.click(screen.getByRole('button', { name: /Interactive/i }));
    await userEvent.click(screen.getByRole('button', { name: /Start Workflow/i }));

    expect(mockCreateWorkflow).toHaveBeenCalledWith('hello', {
      blueprint: { execution_mode: 'interactive' },
    });
  });

  it('sends single-agent blueprint on create (regression: was ignored due to wrong option key)', async () => {
    mockCreateWorkflow.mockResolvedValue({ id: 'wf-2' });

    renderAtNewWorkflow();

    await userEvent.type(
      screen.getByPlaceholderText(/Describe what you want the AI agents to accomplish/i),
      'do it'
    );

    await userEvent.click(screen.getByRole('button', { name: /Single Agent/i }));
    await userEvent.click(screen.getByRole('button', { name: /Start Workflow/i }));

    expect(mockCreateWorkflow).toHaveBeenCalledWith('do it', {
      blueprint: {
        execution_mode: 'single_agent',
        single_agent_name: 'claude',
      },
    });
  });
});

