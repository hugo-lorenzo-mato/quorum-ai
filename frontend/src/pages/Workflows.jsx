import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useWorkflowStore, useTaskStore, useUIStore } from '../stores';
import { fileApi, workflowApi } from '../lib/api';
import MarkdownViewer from '../components/MarkdownViewer';
import {
  GitBranch,
  Plus,
  Play,
  Pause,
  StopCircle,
  CheckCircle2,
  XCircle,
  Clock,
  Activity,
  ArrowLeft,
  ChevronRight,
  Loader2,
  Zap,
  Copy,
  RefreshCw,
} from 'lucide-react';

function normalizeWhitespace(s) {
  return String(s || '').replace(/\s+/g, ' ').trim();
}

function stripCodeFences(s) {
  return String(s || '').replace(/```[\s\S]*?```/g, '');
}

function deriveTitleFromPrompt(prompt) {
  const cleaned = stripCodeFences(prompt);
  const lines = cleaned
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);
  if (lines.length === 0) return '';

  const genericPrefixes = [
    /^analiza\b/i,
    /^analyze\b/i,
    /^implementa\b/i,
    /^implement\b/i,
    /^crea\b/i,
    /^create\b/i,
    /^eres\b/i,
    /^you are\b/i,
  ];
  const isGeneric = (line) => genericPrefixes.some((re) => re.test(line));

  const bestLine = lines.find((l) => !isGeneric(l)) || lines[0];
  const title = normalizeWhitespace(bestLine);

  const maxLen = 110;
  if (title.length <= maxLen) return title;

  const snippet = title.slice(0, maxLen);
  const lastSentence = Math.max(snippet.lastIndexOf('.'), snippet.lastIndexOf('!'), snippet.lastIndexOf('?'));
  if (lastSentence > 50) return snippet.slice(0, lastSentence + 1).trim();
  return snippet.trim();
}

function deriveWorkflowTitle(workflow, tasks = []) {
  const namedTasks = (tasks || []).filter((t) => t?.name && String(t.name).trim().length > 0);
  if (namedTasks.length > 0) {
    const first = String(namedTasks[0].name).trim();
    const extra = Math.max(0, (tasks || []).length - 1);
    return extra > 0 ? `${first} +${extra}` : first;
  }

  const promptTitle = deriveTitleFromPrompt(workflow?.prompt);
  if (promptTitle) return promptTitle;

  return workflow?.id || 'Untitled workflow';
}

function StatusBadge({ status }) {
  const config = {
    pending: { color: 'bg-muted text-muted-foreground', icon: Clock },
    running: { color: 'bg-info/10 text-info', icon: Activity },
    completed: { color: 'bg-success/10 text-success', icon: CheckCircle2 },
    failed: { color: 'bg-error/10 text-error', icon: XCircle },
    paused: { color: 'bg-warning/10 text-warning', icon: Pause },
  };

  const { color, icon: Icon } = config[status] || config.pending;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${color}`}>
      <Icon className="w-3 h-3" />
      {status}
    </span>
  );
}

function WorkflowCard({ workflow, onClick }) {
  return (
    <button
      onClick={onClick}
      className="w-full text-left p-4 rounded-xl border border-border bg-card hover:border-muted-foreground/30 hover:shadow-md transition-all group"
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-foreground line-clamp-2">
            {deriveWorkflowTitle(workflow)}
          </p>
          <p className="text-xs text-muted-foreground mt-1">{workflow.id}</p>
        </div>
        <StatusBadge status={workflow.status} />
      </div>
      <div className="flex items-center gap-4 text-xs text-muted-foreground">
        <span>Phase: {workflow.current_phase || 'N/A'}</span>
        <span>Tasks: {workflow.task_count || 0}</span>
      </div>
      <ChevronRight className="absolute right-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
    </button>
  );
}

function TaskItem({ task, selected, onClick }) {
  const config = {
    pending: { color: 'text-muted-foreground', bg: 'bg-muted' },
    running: { color: 'text-info', bg: 'bg-info/10' },
    completed: { color: 'text-success', bg: 'bg-success/10' },
    failed: { color: 'text-error', bg: 'bg-error/10' },
  };

  const { color, bg } = config[task.status] || config.pending;

  return (
    <button
      type="button"
      onClick={onClick}
      className={`w-full flex items-center gap-3 p-3 rounded-lg border transition-colors ${
        selected
          ? 'border-primary/40 bg-primary/5'
          : 'border-border bg-card hover:border-muted-foreground/30 hover:bg-accent/30'
      }`}
    >
      <div className={`p-2 rounded-lg ${bg}`}>
        {task.status === 'running' ? (
          <Loader2 className={`w-4 h-4 ${color} animate-spin`} />
        ) : task.status === 'completed' ? (
          <CheckCircle2 className={`w-4 h-4 ${color}`} />
        ) : task.status === 'failed' ? (
          <XCircle className={`w-4 h-4 ${color}`} />
        ) : (
          <Clock className={`w-4 h-4 ${color}`} />
        )}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-foreground truncate">{task.name || task.id}</p>
        <p className="text-xs text-muted-foreground">{task.phase || task.type || 'Task'}</p>
      </div>
      <StatusBadge status={task.status} />
    </button>
  );
}

function WorkflowDetail({ workflow, tasks, onBack }) {
  const { startWorkflow, pauseWorkflow, stopWorkflow, error, clearError } = useWorkflowStore();
  const notifyInfo = useUIStore((s) => s.notifyInfo);
  const notifyError = useUIStore((s) => s.notifyError);
  const workflowTitle = useMemo(() => deriveWorkflowTitle(workflow, tasks), [workflow, tasks]);

  const cacheRef = useRef(new Map());
  const [artifactsLoading, setArtifactsLoading] = useState(false);
  const [artifactsError, setArtifactsError] = useState(null);
  const [artifactIndex, setArtifactIndex] = useState(null);

  const [selectedDoc, setSelectedDoc] = useState(null);
  const [docLoading, setDocLoading] = useState(false);
  const [docError, setDocError] = useState(null);
  const [docContent, setDocContent] = useState('');

  const inferReportPath = useCallback(async (workflowId) => {
    try {
      const entries = await fileApi.list('.quorum/output');
      const candidates = entries
        .filter((e) => e.is_dir && e.name?.endsWith(`-${workflowId}`))
        .sort((a, b) => (b.name || '').localeCompare(a.name || ''));

      return candidates[0]?.path || null;
    } catch {
      return null;
    }
  }, []);

  const buildArtifactIndex = useCallback(async () => {
    setArtifactsLoading(true);
    setArtifactsError(null);

    try {
      const reportPath = workflow.report_path || (await inferReportPath(workflow.id));

      if (!reportPath) {
        setArtifactIndex(null);
        return;
      }

      const safeList = async (path) => {
        try {
          return await fileApi.list(path);
        } catch {
          return null;
        }
      };

      const stripMd = (name) => (name || '').replace(/\.md$/i, '');
      const parseNumber = (s) => {
        const match = String(s || '').match(/\d+/);
        return match ? Number(match[0]) : Number.NaN;
      };

      const docs = { prompts: [], analyses: [], comparisons: [], plan: [] };
      const planTaskFiles = [];

      // Analyze phase
      const analyzePath = `${reportPath}/analyze-phase`;
      const analyzeEntries = (await safeList(analyzePath)) || [];
      const analyzeFiles = new Set(analyzeEntries.filter((e) => !e.is_dir).map((e) => e.name));

      if (analyzeFiles.has('00-original-prompt.md')) {
        docs.prompts.push({
          key: 'prompt:original',
          title: 'Prompt original',
          path: `${analyzePath}/00-original-prompt.md`,
        });
      } else if (workflow.prompt) {
        docs.prompts.push({
          key: 'prompt:original:state',
          title: 'Prompt original',
          getContent: async () => `# Prompt original\n\n\`\`\`\n${workflow.prompt}\n\`\`\`\n`,
        });
      }

      if (analyzeFiles.has('01-refined-prompt.md')) {
        docs.prompts.push({
          key: 'prompt:optimized',
          title: 'Prompt optimizado',
          path: `${analyzePath}/01-refined-prompt.md`,
        });
      } else if (workflow.optimized_prompt) {
        docs.prompts.push({
          key: 'prompt:optimized:state',
          title: 'Prompt optimizado',
          getContent: async () => `# Prompt optimizado\n\n${workflow.optimized_prompt}\n`,
        });
      }

      if (analyzeFiles.has('consolidated.md')) {
        docs.analyses.push({
          key: 'analysis:consolidated',
          title: 'Análisis condensado',
          path: `${analyzePath}/consolidated.md`,
        });
      }

      const versionDirs = analyzeEntries
        .filter((e) => e.is_dir && /^v\d+$/i.test(e.name))
        .sort((a, b) => (parseNumber(a.name) || 0) - (parseNumber(b.name) || 0));

      for (const dir of versionDirs) {
        const versionEntries = (await safeList(dir.path)) || [];
        const mdFiles = versionEntries
          .filter((e) => !e.is_dir && /\.md$/i.test(e.name))
          .sort((a, b) => (a.name || '').localeCompare(b.name || ''));

        for (const file of mdFiles) {
          docs.analyses.push({
            key: `analysis:${dir.name}:${file.name}`,
            title: `${dir.name.toUpperCase()} · ${stripMd(file.name)}`,
            path: file.path,
          });
        }
      }

      const singleAgentDir = analyzeEntries.find((e) => e.is_dir && e.name === 'single-agent');
      if (singleAgentDir) {
        const singleAgentEntries = (await safeList(singleAgentDir.path)) || [];
        const mdFiles = singleAgentEntries
          .filter((e) => !e.is_dir && /\.md$/i.test(e.name))
          .sort((a, b) => (a.name || '').localeCompare(b.name || ''));

        for (const file of mdFiles) {
          docs.analyses.push({
            key: `analysis:single-agent:${file.name}`,
            title: `Single-agent · ${stripMd(file.name)}`,
            path: file.path,
          });
        }
      }

      const consensusDir = analyzeEntries.find((e) => e.is_dir && e.name === 'consensus');
      if (consensusDir) {
        const consensusEntries = (await safeList(consensusDir.path)) || [];
        const mdFiles = consensusEntries
          .filter((e) => !e.is_dir && /\.md$/i.test(e.name))
          .sort((a, b) => (parseNumber(a.name) || 0) - (parseNumber(b.name) || 0));

        for (const file of mdFiles) {
          docs.comparisons.push({
            key: `consensus:${file.name}`,
            title: `Consenso · ${stripMd(file.name)}`,
            path: file.path,
          });
        }
      }

      const moderatorDir = analyzeEntries.find((e) => e.is_dir && e.name === 'moderator');
      if (moderatorDir) {
        const moderatorEntries = (await safeList(moderatorDir.path)) || [];
        const mdFiles = moderatorEntries
          .filter((e) => !e.is_dir && /\.md$/i.test(e.name))
          .sort((a, b) => (parseNumber(a.name) || 0) - (parseNumber(b.name) || 0));

        for (const file of mdFiles) {
          docs.comparisons.push({
            key: `moderator:${file.name}`,
            title: `Moderador · ${stripMd(file.name)}`,
            path: file.path,
          });
        }
      }

      // Plan phase
      const planPath = `${reportPath}/plan-phase`;
      const planEntries = (await safeList(planPath)) || [];
      const planFiles = new Set(planEntries.filter((e) => !e.is_dir).map((e) => e.name));

      if (planFiles.has('execution-graph.md')) {
        docs.plan.push({
          key: 'plan:execution-graph',
          title: 'Execution graph',
          path: `${planPath}/execution-graph.md`,
        });
      }
      if (planFiles.has('consolidated-plan.md')) {
        docs.plan.push({
          key: 'plan:consolidated',
          title: 'Plan consolidado',
          path: `${planPath}/consolidated-plan.md`,
        });
      }
      if (planFiles.has('final-plan.md')) {
        docs.plan.push({
          key: 'plan:final',
          title: 'Plan final',
          path: `${planPath}/final-plan.md`,
        });
      }

      const planV1Dir = planEntries.find((e) => e.is_dir && e.name === 'v1');
      if (planV1Dir) {
        const v1Entries = (await safeList(planV1Dir.path)) || [];
        const mdFiles = v1Entries
          .filter((e) => !e.is_dir && /\.md$/i.test(e.name))
          .sort((a, b) => (a.name || '').localeCompare(b.name || ''));

        for (const file of mdFiles) {
          docs.plan.push({
            key: `plan:v1:${file.name}`,
            title: `Plan v1 · ${stripMd(file.name)}`,
            path: file.path,
          });
        }
      }

      const tasksDir = planEntries.find((e) => e.is_dir && e.name === 'tasks');
      if (tasksDir) {
        const taskEntries = (await safeList(tasksDir.path)) || [];
        planTaskFiles.push(
          ...taskEntries
            .filter((e) => !e.is_dir && /\.md$/i.test(e.name))
            .sort((a, b) => (a.name || '').localeCompare(b.name || '')),
        );
      }

      setArtifactIndex({ reportPath, docs, planTaskFiles });
    } catch (err) {
      setArtifactIndex(null);
      setArtifactsError(err?.message || 'Failed to load artifacts');
    } finally {
      setArtifactsLoading(false);
    }
  }, [inferReportPath, workflow]);

  useEffect(() => {
    buildArtifactIndex();
  }, [buildArtifactIndex]);

  const taskPlanById = useMemo(() => {
    const map = {};
    const files = artifactIndex?.planTaskFiles || [];
    if (files.length === 0 || tasks.length === 0) return map;

    for (const task of tasks) {
      const match = files.find((f) => f.name === `${task.id}.md` || f.name?.startsWith(`${task.id}-`));
      if (match?.path) {
        map[task.id] = match.path;
      }
    }

    return map;
  }, [artifactIndex?.planTaskFiles, tasks]);

  const loadDoc = useCallback(async (doc, { force = false } = {}) => {
    if (!doc) return;

    const cacheKey = doc.path || doc.key;
    if (!force && cacheRef.current.has(cacheKey)) {
      setDocError(null);
      setDocContent(cacheRef.current.get(cacheKey));
      return;
    }

    setDocLoading(true);
    setDocError(null);

    try {
      let markdown = '';
      if (doc.getContent) {
        markdown = await doc.getContent();
      } else if (doc.path) {
        const file = await fileApi.getContent(doc.path);
        if (file.binary) {
          throw new Error('File is binary');
        }
        markdown = file.content || '';
      }

      cacheRef.current.set(cacheKey, markdown);
      setDocContent(markdown);
    } catch (err) {
      setDocContent('');
      setDocError(err?.message || 'Failed to load document');
    } finally {
      setDocLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!selectedDoc && artifactIndex) {
      const first =
        artifactIndex.docs.prompts[0] ||
        artifactIndex.docs.analyses[0] ||
        artifactIndex.docs.comparisons[0] ||
        artifactIndex.docs.plan[0];
      if (first) setSelectedDoc(first);
    }
  }, [artifactIndex, selectedDoc]);

  useEffect(() => {
    loadDoc(selectedDoc);
  }, [loadDoc, selectedDoc]);

  const selectTask = useCallback((task) => {
    const planPath = taskPlanById[task.id];
    if (planPath) {
      setSelectedDoc({
        key: `task-plan:${task.id}`,
        title: `${task.name || task.id} · Plan`,
        path: planPath,
      });
      return;
    }

    setSelectedDoc({
      key: `task-output:${task.id}`,
      title: `${task.name || task.id} · Output`,
      getContent: async () => {
        const taskData = await workflowApi.getTask(workflow.id, task.id);
        let output = taskData.output || '';

        if (taskData.output_file) {
          try {
            const file = await fileApi.getContent(taskData.output_file);
            if (!file.binary && file.content) output = file.content;
          } catch {
            // Fallback to inline output
          }
        }

        if (!output) return '_No output captured for this task._\n';
        return `# Output\n\n\`\`\`\n${output}\n\`\`\`\n`;
      },
    });
  }, [taskPlanById, workflow.id]);

  const handleCopy = useCallback(async () => {
    if (!docContent) return;
    try {
      await navigator.clipboard.writeText(docContent);
      notifyInfo('Copied to clipboard');
    } catch {
      notifyError('Failed to copy');
    }
  }, [docContent, notifyError, notifyInfo]);

  const handleRefresh = useCallback(() => {
    loadDoc(selectedDoc, { force: true });
  }, [loadDoc, selectedDoc]);

  const docGroups = useMemo(() => ([
    { id: 'prompts', label: 'Prompts', docs: artifactIndex?.docs.prompts || [] },
    { id: 'analyses', label: 'Análisis', docs: artifactIndex?.docs.analyses || [] },
    { id: 'comparisons', label: 'Comparativos', docs: artifactIndex?.docs.comparisons || [] },
    { id: 'plan', label: 'Plan', docs: artifactIndex?.docs.plan || [] },
  ]), [artifactIndex]);

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center gap-4">
        <button
          onClick={onBack}
          className="p-2 rounded-lg hover:bg-accent transition-colors"
        >
          <ArrowLeft className="w-5 h-5 text-muted-foreground" />
        </button>
        <div className="flex-1">
          <h1 className="text-xl font-semibold text-foreground line-clamp-2">{workflowTitle}</h1>
          <p className="text-sm text-muted-foreground">{workflow.id}</p>
        </div>
        <div className="flex items-center gap-2">
          {workflow.status === 'pending' && (
            <button
              onClick={() => startWorkflow(workflow.id)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
            >
              <Play className="w-4 h-4" />
              Start
            </button>
          )}
          {workflow.status === 'running' && (
            <>
              <button
                onClick={() => pauseWorkflow(workflow.id)}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 transition-colors"
              >
                <Pause className="w-4 h-4" />
                Pause
              </button>
              <button
                onClick={() => stopWorkflow(workflow.id)}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-destructive text-destructive-foreground text-sm font-medium hover:bg-destructive/90 transition-colors"
              >
                <StopCircle className="w-4 h-4" />
                Stop
              </button>
            </>
          )}
        </div>
      </div>

      {/* Error Banner */}
      {error && (
        <div className="p-4 rounded-lg bg-warning/10 border border-warning/20 flex items-start justify-between">
          <p className="text-sm text-warning">{error}</p>
          <button onClick={clearError} className="text-warning hover:text-warning/80 text-sm">
            Dismiss
          </button>
        </div>
      )}

      {/* Info Card */}
      <div className="p-6 rounded-xl border border-border bg-card">
        <div className="flex items-start justify-between mb-4">
          <div>
            {workflow.prompt ? (
              <p className="text-sm text-muted-foreground mb-2 line-clamp-3">
                {normalizeWhitespace(workflow.prompt)}
              </p>
            ) : (
              <p className="text-sm text-muted-foreground mb-2">No prompt</p>
            )}
            <StatusBadge status={workflow.status} />
          </div>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4 pt-4 border-t border-border">
          <div>
            <p className="text-xs text-muted-foreground">Phase</p>
            <p className="text-sm font-medium text-foreground">{workflow.current_phase || 'N/A'}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Tasks</p>
            <p className="text-sm font-medium text-foreground">{tasks.length}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Created</p>
            <p className="text-sm font-medium text-foreground">
              {workflow.created_at ? new Date(workflow.created_at).toLocaleDateString() : '—'}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Updated</p>
            <p className="text-sm font-medium text-foreground">
              {workflow.updated_at ? new Date(workflow.updated_at).toLocaleDateString() : '—'}
            </p>
          </div>
        </div>
      </div>

      {/* Inspector */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="space-y-6">
          {/* Tasks */}
          <div className="p-4 rounded-xl border border-border bg-card">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-foreground">Tasks ({tasks.length})</h3>
            </div>
            {tasks.length > 0 ? (
              <div className="space-y-2 max-h-[45vh] overflow-y-auto pr-1">
                {tasks.map((task) => (
                  <TaskItem
                    key={task.id}
                    task={task}
                    selected={selectedDoc?.key?.includes(`:${task.id}`)}
                    onClick={() => selectTask(task)}
                  />
                ))}
              </div>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                <p className="text-sm">No tasks yet</p>
              </div>
            )}
          </div>

          {/* Artifacts */}
          <div className="p-4 rounded-xl border border-border bg-card">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-foreground">Artifacts</h3>
              {artifactsLoading && <Loader2 className="w-4 h-4 text-muted-foreground animate-spin" />}
            </div>

            {artifactsError && (
              <p className="text-sm text-error mb-3">{artifactsError}</p>
            )}

            {!artifactIndex?.reportPath ? (
              <p className="text-sm text-muted-foreground">
                No report artifacts found for this workflow yet.
              </p>
            ) : (
              <div className="space-y-4">
                <p className="text-xs text-muted-foreground truncate">{artifactIndex.reportPath}</p>
                <div className="space-y-4 max-h-[45vh] overflow-y-auto pr-1">
                  {docGroups.map((group) => (
                    group.docs.length > 0 && (
                      <div key={group.id} className="space-y-2">
                        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                          {group.label}
                        </p>
                        <div className="space-y-1">
                          {group.docs.map((doc) => (
                            <button
                              key={doc.key}
                              type="button"
                              onClick={() => setSelectedDoc(doc)}
                              className={`w-full px-3 py-2 rounded-lg text-left text-sm transition-colors ${
                                selectedDoc?.key === doc.key
                                  ? 'bg-primary/10 text-foreground border border-primary/20'
                                  : 'hover:bg-accent/40 text-muted-foreground'
                              }`}
                              title={doc.path || doc.title}
                            >
                              <span className="block truncate">{doc.title}</span>
                            </button>
                          ))}
                        </div>
                      </div>
                    )
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Preview */}
        <div className="lg:col-span-2 p-6 rounded-xl border border-border bg-card">
          <div className="flex items-start justify-between gap-4 mb-4">
            <div className="min-w-0">
              <h3 className="text-lg font-semibold text-foreground truncate">
                {selectedDoc?.title || 'Preview'}
              </h3>
              {selectedDoc?.path && (
                <p className="text-xs text-muted-foreground truncate">{selectedDoc.path}</p>
              )}
            </div>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={handleCopy}
                disabled={!docContent}
                className="p-2 rounded-lg hover:bg-accent disabled:opacity-50 transition-colors"
                title="Copy raw markdown"
              >
                <Copy className="w-4 h-4 text-muted-foreground" />
              </button>
              <button
                type="button"
                onClick={handleRefresh}
                disabled={!selectedDoc}
                className="p-2 rounded-lg hover:bg-accent disabled:opacity-50 transition-colors"
                title="Refresh"
              >
                <RefreshCw className="w-4 h-4 text-muted-foreground" />
              </button>
            </div>
          </div>

          {docLoading ? (
            <div className="space-y-3">
              <div className="h-4 w-1/3 bg-muted rounded animate-pulse" />
              <div className="h-4 w-2/3 bg-muted rounded animate-pulse" />
              <div className="h-4 w-1/2 bg-muted rounded animate-pulse" />
              <div className="h-4 w-3/4 bg-muted rounded animate-pulse" />
            </div>
          ) : docError ? (
            <div className="text-sm text-error">{docError}</div>
          ) : selectedDoc ? (
            <MarkdownViewer markdown={docContent} />
          ) : (
            <div className="text-sm text-muted-foreground">
              Select a task or document to preview.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function NewWorkflowForm({ onSubmit, onCancel, loading }) {
  const [prompt, setPrompt] = useState('');

  const handleSubmit = (e) => {
    e.preventDefault();
    if (prompt.trim()) onSubmit(prompt.trim());
  };

  return (
    <div className="max-w-2xl mx-auto p-6 rounded-xl border border-border bg-card animate-fade-up">
      <h2 className="text-xl font-semibold text-foreground mb-4">Create New Workflow</h2>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-foreground mb-2">
            Workflow Prompt
          </label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="Describe what you want the AI agents to accomplish..."
            rows={4}
            className="w-full px-4 py-3 border border-input rounded-lg bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background resize-none"
          />
        </div>
        <div className="flex gap-3">
          <button
            type="submit"
            disabled={loading || !prompt.trim()}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
            Create Workflow
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2.5 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 transition-colors"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}

export default function Workflows() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { workflows, loading, fetchWorkflows, fetchWorkflow, createWorkflow } = useWorkflowStore();
  const { getTasksForWorkflow, setTasks } = useTaskStore();
  const [showNewForm, setShowNewForm] = useState(false);

  useEffect(() => {
    fetchWorkflows();
  }, [fetchWorkflows]);

  useEffect(() => {
    if (id && id !== 'new') {
      fetchWorkflow(id);
    }
  }, [fetchWorkflow, id]);

  // Fetch tasks for the selected workflow
  const fetchTasks = useCallback(async (workflowId) => {
    try {
      const taskList = await workflowApi.getTasks(workflowId);
      setTasks(workflowId, taskList);
    } catch (error) {
      console.error('Failed to fetch tasks:', error);
    }
  }, [setTasks]);

  useEffect(() => {
    if (id && id !== 'new') {
      fetchTasks(id);
    }
  }, [fetchTasks, id]);

  const selectedWorkflow = workflows.find(w => w.id === id);
  const workflowTasks = id ? getTasksForWorkflow(id) : [];

  const handleCreate = async (prompt) => {
    const workflow = await createWorkflow(prompt);
    if (workflow) {
      setShowNewForm(false);
      navigate(`/workflows/${workflow.id}`);
    }
  };

  // Show new workflow form
  if (id === 'new' || showNewForm) {
    return (
      <NewWorkflowForm
        onSubmit={handleCreate}
        onCancel={() => {
          setShowNewForm(false);
          if (id === 'new') navigate('/workflows');
        }}
        loading={loading}
      />
    );
  }

  // Show workflow detail
  if (selectedWorkflow) {
    return (
      <WorkflowDetail
        workflow={selectedWorkflow}
        tasks={workflowTasks}
        onBack={() => navigate('/workflows')}
      />
    );
  }

  // Show workflow list
  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">Workflows</h1>
          <p className="text-sm text-muted-foreground mt-1">Manage your AI automation workflows</p>
        </div>
        <Link
          to="/workflows/new"
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Workflow
        </Link>
      </div>

      {loading && workflows.length === 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="h-32 rounded-xl bg-muted animate-pulse" />
          ))}
        </div>
      ) : workflows.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {workflows.map((workflow) => (
            <WorkflowCard
              key={workflow.id}
              workflow={workflow}
              onClick={() => navigate(`/workflows/${workflow.id}`)}
            />
          ))}
        </div>
      ) : (
        <div className="text-center py-16">
          <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-muted flex items-center justify-center">
            <GitBranch className="w-8 h-8 text-muted-foreground" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-2">No workflows yet</h3>
          <p className="text-sm text-muted-foreground mb-4">Create your first workflow to get started</p>
          <Link
            to="/workflows/new"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
          >
            <Plus className="w-4 h-4" />
            Create Workflow
          </Link>
        </div>
      )}
    </div>
  );
}
