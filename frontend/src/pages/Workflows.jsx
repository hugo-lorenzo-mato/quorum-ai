import { useEffect, useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { useWorkflowStore, useTaskStore } from '../stores';

function TaskItem({ task, isSelected, onClick }) {
  const statusColors = {
    pending: 'bg-gray-400',
    running: 'bg-blue-500 animate-pulse',
    completed: 'bg-green-500',
    failed: 'bg-red-500',
    skipped: 'bg-yellow-500',
    retrying: 'bg-orange-500 animate-pulse',
  };

  return (
    <button
      onClick={onClick}
      className={`w-full text-left p-4 rounded-lg border transition-all ${
        isSelected
          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
          : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
      }`}
    >
      <div className="flex items-start gap-3">
        <div className={`mt-1 w-3 h-3 rounded-full ${statusColors[task.status] || statusColors.pending}`} />
        <div className="flex-1 min-w-0">
          <p className="font-medium text-gray-900 dark:text-white truncate">{task.name}</p>
          <div className="mt-1 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">{task.agent}</span>
            <span>{task.phase}</span>
          </div>
          {task.status === 'completed' && task.cost_usd && (
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Cost: ${task.cost_usd.toFixed(4)} | Tokens: {task.tokens_in}/{task.tokens_out}
            </p>
          )}
          {task.error && (
            <p className="mt-1 text-xs text-red-600 dark:text-red-400 truncate">{task.error}</p>
          )}
        </div>
      </div>
    </button>
  );
}

function WorkflowDetail({ workflowId }) {
  const { workflows, fetchWorkflow, fetchTasks } = useWorkflowStore();
  const { tasksByWorkflow, selectedTaskId, selectTask } = useTaskStore();
  const [loading, setLoading] = useState(true);

  const workflow = workflows.find(w => w.id === workflowId);
  const tasks = Object.values(tasksByWorkflow[workflowId] || {});

  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      await fetchWorkflow(workflowId);
      await fetchTasks(workflowId);
      setLoading(false);
    };
    loadData();
  }, [workflowId, fetchWorkflow, fetchTasks]);

  const selectedTask = tasks.find(t => t.id === selectedTaskId);

  if (loading) {
    return (
      <div className="flex justify-center py-16">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  if (!workflow) {
    return (
      <div className="text-center py-16">
        <p className="text-gray-500 dark:text-gray-400">Workflow not found</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      {/* Workflow info */}
      <div className="lg:col-span-3 bg-white dark:bg-gray-800 rounded-xl shadow-sm p-6">
        <div className="flex items-start justify-between">
          <div>
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
              {workflow.prompt?.substring(0, 80) || 'Untitled'}...
            </h2>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{workflow.id}</p>
          </div>
          <span className={`px-3 py-1 text-sm font-medium rounded-full ${
            workflow.status === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-400' :
            workflow.status === 'running' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-400' :
            workflow.status === 'failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-400' :
            'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
          }`}>
            {workflow.status}
          </span>
        </div>
        <div className="mt-4 grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          <div>
            <p className="text-gray-500 dark:text-gray-400">Phase</p>
            <p className="font-medium text-gray-900 dark:text-white">{workflow.current_phase || 'N/A'}</p>
          </div>
          <div>
            <p className="text-gray-500 dark:text-gray-400">Tasks</p>
            <p className="font-medium text-gray-900 dark:text-white">{workflow.task_count || tasks.length}</p>
          </div>
          {workflow.metrics && (
            <>
              <div>
                <p className="text-gray-500 dark:text-gray-400">Total Cost</p>
                <p className="font-medium text-gray-900 dark:text-white">${workflow.metrics.total_cost_usd?.toFixed(4) || '0.00'}</p>
              </div>
              <div>
                <p className="text-gray-500 dark:text-gray-400">Tokens</p>
                <p className="font-medium text-gray-900 dark:text-white">
                  {workflow.metrics.total_tokens_in || 0} / {workflow.metrics.total_tokens_out || 0}
                </p>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Task list */}
      <div className="lg:col-span-1">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Tasks</h3>
        <div className="space-y-2">
          {tasks.length > 0 ? (
            tasks.map(task => (
              <TaskItem
                key={task.id}
                task={task}
                isSelected={selectedTaskId === task.id}
                onClick={() => selectTask(task.id)}
              />
            ))
          ) : (
            <p className="text-gray-500 dark:text-gray-400 text-center py-4">No tasks</p>
          )}
        </div>
      </div>

      {/* Task detail */}
      <div className="lg:col-span-2">
        {selectedTask ? (
          <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm p-6">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">{selectedTask.name}</h3>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <p className="text-gray-500 dark:text-gray-400">Status</p>
                  <p className="font-medium text-gray-900 dark:text-white capitalize">{selectedTask.status}</p>
                </div>
                <div>
                  <p className="text-gray-500 dark:text-gray-400">Agent</p>
                  <p className="font-medium text-gray-900 dark:text-white">{selectedTask.agent}</p>
                </div>
                <div>
                  <p className="text-gray-500 dark:text-gray-400">Model</p>
                  <p className="font-medium text-gray-900 dark:text-white">{selectedTask.model || 'N/A'}</p>
                </div>
                <div>
                  <p className="text-gray-500 dark:text-gray-400">Phase</p>
                  <p className="font-medium text-gray-900 dark:text-white">{selectedTask.phase}</p>
                </div>
              </div>
              {selectedTask.error && (
                <div className="p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
                  <p className="text-sm font-medium text-red-800 dark:text-red-400">Error</p>
                  <p className="mt-1 text-sm text-red-700 dark:text-red-300">{selectedTask.error}</p>
                </div>
              )}
              {selectedTask.output && (
                <div>
                  <p className="text-sm font-medium text-gray-900 dark:text-white mb-2">Output</p>
                  <pre className="p-4 bg-gray-50 dark:bg-gray-900 rounded-lg text-sm overflow-x-auto">
                    {selectedTask.output}
                  </pre>
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm p-6 text-center text-gray-500 dark:text-gray-400">
            Select a task to view details
          </div>
        )}
      </div>
    </div>
  );
}

function NewWorkflow() {
  const navigate = useNavigate();
  const { createWorkflow, loading } = useWorkflowStore();
  const [prompt, setPrompt] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!prompt.trim()) return;

    const workflow = await createWorkflow(prompt.trim());
    if (workflow) {
      navigate(`/workflows/${workflow.id}`);
    }
  };

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-6">New Workflow</h1>
      <form onSubmit={handleSubmit} className="bg-white dark:bg-gray-800 rounded-xl shadow-sm p-6">
        <div className="mb-4">
          <label htmlFor="prompt" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Prompt
          </label>
          <textarea
            id="prompt"
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            rows={6}
            className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            placeholder="Describe what you want the AI to accomplish..."
          />
        </div>
        <div className="flex justify-end gap-4">
          <Link
            to="/workflows"
            className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white"
          >
            Cancel
          </Link>
          <button
            type="submit"
            disabled={loading || !prompt.trim()}
            className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {loading ? 'Creating...' : 'Create Workflow'}
          </button>
        </div>
      </form>
    </div>
  );
}

function WorkflowList() {
  const { workflows, fetchWorkflows, loading } = useWorkflowStore();

  useEffect(() => {
    fetchWorkflows();
  }, [fetchWorkflows]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Workflows</h1>
        <Link
          to="/workflows/new"
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          New Workflow
        </Link>
      </div>

      {loading ? (
        <div className="flex justify-center py-16">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
        </div>
      ) : workflows.length > 0 ? (
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Workflow
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Phase
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Tasks
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Updated
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {workflows.map(workflow => (
                <tr key={workflow.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                  <td className="px-6 py-4">
                    <Link
                      to={`/workflows/${workflow.id}`}
                      className="text-blue-600 dark:text-blue-400 hover:underline"
                    >
                      {workflow.prompt?.substring(0, 50) || workflow.id}...
                    </Link>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs font-medium rounded-full ${
                      workflow.status === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-400' :
                      workflow.status === 'running' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-400' :
                      workflow.status === 'failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-400' :
                      'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                    }`}>
                      {workflow.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {workflow.current_phase || 'N/A'}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {workflow.task_count || 0}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {new Date(workflow.updated_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="text-center py-16 bg-white dark:bg-gray-800 rounded-xl shadow-sm">
          <p className="text-gray-500 dark:text-gray-400 mb-4">No workflows yet</p>
          <Link
            to="/workflows/new"
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            Create your first workflow
          </Link>
        </div>
      )}
    </div>
  );
}

export default function Workflows() {
  const { id } = useParams();

  if (id === 'new') {
    return <NewWorkflow />;
  }

  if (id) {
    return <WorkflowDetail workflowId={id} />;
  }

  return <WorkflowList />;
}
