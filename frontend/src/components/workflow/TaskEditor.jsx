import { useState, useEffect } from 'react';
import { Plus, Trash2, ChevronUp, ChevronDown, Pencil, Check, X } from 'lucide-react';
import useWorkflowStore from '../../stores/workflowStore';

/**
 * TaskEditor allows editing, adding, deleting, and reordering tasks
 * when a workflow is in awaiting_review or completed state after planning.
 */
export default function TaskEditor({ workflowId }) {
  const { tasks, fetchTasks, createTask, updateTask, deleteTask, reorderTasks } = useWorkflowStore();
  const [editingId, setEditingId] = useState(null);
  const [editForm, setEditForm] = useState({});
  const [showAddForm, setShowAddForm] = useState(false);
  const [newTask, setNewTask] = useState({ name: '', cli: '', description: '' });

  const taskList = tasks[workflowId] || [];

  useEffect(() => {
    fetchTasks(workflowId);
  }, [workflowId, fetchTasks]);

  const handleStartEdit = (task) => {
    setEditingId(task.id);
    setEditForm({ name: task.name, cli: task.cli, description: task.description || '' });
  };

  const handleSaveEdit = async () => {
    if (!editingId) return;
    const updates = {};
    if (editForm.name) updates.name = editForm.name;
    if (editForm.cli) updates.cli = editForm.cli;
    updates.description = editForm.description;
    await updateTask(workflowId, editingId, updates);
    setEditingId(null);
  };

  const handleCancelEdit = () => {
    setEditingId(null);
    setEditForm({});
  };

  const handleDelete = async (taskId) => {
    await deleteTask(workflowId, taskId);
  };

  const handleAdd = async () => {
    if (!newTask.name || !newTask.cli) return;
    await createTask(workflowId, newTask);
    setNewTask({ name: '', cli: '', description: '' });
    setShowAddForm(false);
  };

  const handleMoveUp = async (index) => {
    if (index <= 0) return;
    const order = taskList.map(t => t.id);
    [order[index - 1], order[index]] = [order[index], order[index - 1]];
    await reorderTasks(workflowId, order);
  };

  const handleMoveDown = async (index) => {
    if (index >= taskList.length - 1) return;
    const order = taskList.map(t => t.id);
    [order[index], order[index + 1]] = [order[index + 1], order[index]];
    await reorderTasks(workflowId, order);
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium text-foreground">
          Task Plan ({taskList.length} tasks)
        </h4>
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs bg-primary/10 text-primary hover:bg-primary/20 transition-colors"
        >
          <Plus className="w-3 h-3" />
          Add Task
        </button>
      </div>

      {/* Task list */}
      <div className="space-y-2">
        {taskList.map((task, index) => (
          <div key={task.id} className="flex items-center gap-2 p-2 rounded-lg border border-border bg-card">
            {/* Reorder buttons */}
            <div className="flex flex-col gap-0.5">
              <button
                onClick={() => handleMoveUp(index)}
                disabled={index === 0}
                className="p-0.5 rounded hover:bg-muted disabled:opacity-30 transition-colors"
              >
                <ChevronUp className="w-3 h-3" />
              </button>
              <button
                onClick={() => handleMoveDown(index)}
                disabled={index === taskList.length - 1}
                className="p-0.5 rounded hover:bg-muted disabled:opacity-30 transition-colors"
              >
                <ChevronDown className="w-3 h-3" />
              </button>
            </div>

            {/* Task content */}
            <div className="flex-1 min-w-0">
              {editingId === task.id ? (
                <div className="space-y-1.5">
                  <input
                    value={editForm.name}
                    onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                    placeholder="Task name"
                    className="w-full px-2 py-1 rounded border border-input bg-background text-sm text-foreground"
                  />
                  <div className="flex gap-2">
                    <input
                      value={editForm.cli}
                      onChange={(e) => setEditForm({ ...editForm, cli: e.target.value })}
                      placeholder="Agent (e.g., claude)"
                      className="w-32 px-2 py-1 rounded border border-input bg-background text-sm text-foreground"
                    />
                    <input
                      value={editForm.description}
                      onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                      placeholder="Description (optional)"
                      className="flex-1 px-2 py-1 rounded border border-input bg-background text-sm text-foreground"
                    />
                  </div>
                </div>
              ) : (
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                      {task.cli}
                    </span>
                    <span className="text-sm font-medium text-foreground truncate">
                      {task.name}
                    </span>
                  </div>
                  {task.description && (
                    <p className="text-xs text-muted-foreground mt-0.5 truncate">{task.description}</p>
                  )}
                  {task.dependencies && task.dependencies.length > 0 && (
                    <p className="text-xs text-muted-foreground mt-0.5">
                      Depends on: {task.dependencies.join(', ')}
                    </p>
                  )}
                </div>
              )}
            </div>

            {/* Action buttons */}
            <div className="flex items-center gap-1">
              {editingId === task.id ? (
                <>
                  <button
                    onClick={handleSaveEdit}
                    className="p-1 rounded hover:bg-success/20 text-success transition-colors"
                  >
                    <Check className="w-3.5 h-3.5" />
                  </button>
                  <button
                    onClick={handleCancelEdit}
                    className="p-1 rounded hover:bg-muted text-muted-foreground transition-colors"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                </>
              ) : (
                <>
                  <button
                    onClick={() => handleStartEdit(task)}
                    className="p-1 rounded hover:bg-muted text-muted-foreground transition-colors"
                  >
                    <Pencil className="w-3.5 h-3.5" />
                  </button>
                  <button
                    onClick={() => handleDelete(task.id)}
                    className="p-1 rounded hover:bg-destructive/20 text-destructive transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Add task form */}
      {showAddForm && (
        <div className="p-3 rounded-lg border border-dashed border-primary/30 bg-primary/5 space-y-2">
          <input
            value={newTask.name}
            onChange={(e) => setNewTask({ ...newTask, name: e.target.value })}
            placeholder="Task name"
            className="w-full px-2 py-1.5 rounded border border-input bg-background text-sm text-foreground"
          />
          <div className="flex gap-2">
            <input
              value={newTask.cli}
              onChange={(e) => setNewTask({ ...newTask, cli: e.target.value })}
              placeholder="Agent (e.g., claude)"
              className="w-32 px-2 py-1.5 rounded border border-input bg-background text-sm text-foreground"
            />
            <input
              value={newTask.description}
              onChange={(e) => setNewTask({ ...newTask, description: e.target.value })}
              placeholder="Description (optional)"
              className="flex-1 px-2 py-1.5 rounded border border-input bg-background text-sm text-foreground"
            />
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleAdd}
              disabled={!newTask.name || !newTask.cli}
              className="px-3 py-1 rounded text-xs bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-colors"
            >
              Add
            </button>
            <button
              onClick={() => { setShowAddForm(false); setNewTask({ name: '', cli: '', description: '' }); }}
              className="px-3 py-1 rounded text-xs text-muted-foreground hover:bg-muted transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
