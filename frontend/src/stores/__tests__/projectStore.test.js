import { describe, it, expect, beforeEach, vi } from 'vitest';

const mocks = vi.hoisted(() => {
  const fetchWorkflows = vi.fn().mockResolvedValue([]);
  const fetchBoard = vi.fn().mockResolvedValue();
  const fetchSessions = vi.fn().mockResolvedValue();
  const loadConfig = vi.fn().mockResolvedValue();

  return {
    fetchWorkflows,
    fetchBoard,
    fetchSessions,
    loadConfig,
    workflowStoreMock: { setState: vi.fn(), getState: () => ({ fetchWorkflows }) },
    kanbanStoreMock: { setState: vi.fn(), getState: () => ({ fetchBoard }) },
    chatStoreMock: { setState: vi.fn(), getState: () => ({ fetchSessions }) },
    configStoreMock: { setState: vi.fn(), getState: () => ({ loadConfig }) },
  };
});

vi.mock('../../lib/api', () => ({
  projectApi: {
    list: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    validate: vi.fn(),
    setDefault: vi.fn(),
  },
}));

vi.mock('../workflowStore', () => ({ default: mocks.workflowStoreMock }));
vi.mock('../kanbanStore', () => ({ default: mocks.kanbanStoreMock }));
vi.mock('../chatStore', () => ({ default: mocks.chatStoreMock }));
vi.mock('../configStore', () => ({ useConfigStore: mocks.configStoreMock }));

import useProjectStore from '../projectStore';
import { projectApi } from '../../lib/api';

function resetStore() {
  try {
    globalThis.localStorage.removeItem('quorum-project-store');
  } catch {
    // ignore
  }
  useProjectStore.setState({
    projects: [],
    currentProjectId: null,
    defaultProjectId: null,
    loading: false,
    error: null,
  });
}

describe('projectStore', () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
  });

  it('fetchProjects sets defaultProjectId based on is_default', async () => {
    projectApi.list.mockResolvedValue([{ id: 'p1', is_default: true }, { id: 'p2', is_default: false }]);
    const projects = await useProjectStore.getState().fetchProjects();
    expect(projects).toHaveLength(2);
    expect(useProjectStore.getState().defaultProjectId).toBe('p1');
    expect(useProjectStore.getState().loading).toBe(false);
  });

  it('setDefaultProject updates is_default for all projects', async () => {
    useProjectStore.setState({ projects: [{ id: 'p1', is_default: true }, { id: 'p2', is_default: false }] });
    projectApi.setDefault.mockResolvedValue({ ok: true });

    await useProjectStore.getState().setDefaultProject('p2');
    expect(useProjectStore.getState().defaultProjectId).toBe('p2');
    expect(useProjectStore.getState().projects.find(p => p.id === 'p2').is_default).toBe(true);
    expect(useProjectStore.getState().projects.find(p => p.id === 'p1').is_default).toBe(false);
  });

  it('selectProject short-circuits when selecting current project', async () => {
    useProjectStore.setState({ currentProjectId: 'p1' });
    await useProjectStore.getState().selectProject('p1');
    expect(mocks.fetchWorkflows).not.toHaveBeenCalled();
  });

  it('selectProject refreshes dependent stores when switching projects', async () => {
    useProjectStore.setState({ currentProjectId: 'p1' });
    await useProjectStore.getState().selectProject('p2');

    expect(useProjectStore.getState().currentProjectId).toBe('p2');
    expect(mocks.workflowStoreMock.setState).toHaveBeenCalled();
    expect(mocks.kanbanStoreMock.setState).toHaveBeenCalled();
    expect(mocks.chatStoreMock.setState).toHaveBeenCalled();
    expect(mocks.configStoreMock.setState).toHaveBeenCalled();

    expect(mocks.fetchWorkflows).toHaveBeenCalled();
    expect(mocks.fetchBoard).toHaveBeenCalled();
    expect(mocks.fetchSessions).toHaveBeenCalled();
    expect(mocks.loadConfig).toHaveBeenCalled();
  });

  it('selectProject stores error when refresh fails', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    mocks.fetchWorkflows.mockRejectedValueOnce(new Error('boom'));
    await useProjectStore.getState().selectProject('p2');
    expect(useProjectStore.getState().error).toBe('boom');
    consoleSpy.mockRestore();
  });
});
