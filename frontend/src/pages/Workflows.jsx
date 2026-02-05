import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useParams, useNavigate, Link, useSearchParams, useLocation } from 'react-router-dom';
import { useWorkflowStore, useTaskStore, useUIStore, useAgentStore, useConfigStore } from '../stores';
import { fileApi, workflowApi } from '../lib/api';
import { getModelsForAgent, getReasoningLevels, supportsReasoning, useEnums } from '../lib/agents';
import { getStatusColor } from '../lib/theme';
import MarkdownViewer from '../components/MarkdownViewer';
import AgentActivity, { AgentActivityCompact } from '../components/AgentActivity';
import EditWorkflowModal from '../components/EditWorkflowModal';
import VoiceInputButton from '../components/VoiceInputButton';
import FAB from '../components/FAB';
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
  ChevronDown,
  ChevronUp,
  Loader2,
  Zap,
  Copy,
  Download,
  Upload,
  RefreshCw,
  Pencil,
  Trash2,
  FastForward,
  RotateCcw,
  Search,
  List,
  FileText,
  Network,
  LayoutList,
  Layers,
  FolderTree,
  Filter,
  Sparkles,
} from 'lucide-react';
import { ConfirmDialog } from '../components/config/ConfirmDialog';
import { ExecutionModeBadge, PhaseStepper, ReplanModal } from '../components/workflow';
import { GenerationOptionsModal } from '../components/issues';
import useIssuesStore from '../stores/issuesStore';
import FileTree from '../components/FileTree';
import CodeEditor from '../components/CodeEditor';
import WorkflowGraph from '../components/WorkflowGraph';

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
  // Use explicit title if available
  if (workflow?.title && String(workflow.title).trim().length > 0) {
    return String(workflow.title).trim();
  }

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
  const { bg, text } = getStatusColor(status);
  // Map icon based on status since getStatusColor returns generic props
  // We can override the icon from the theme if we want, but for now we'll just use the status string to pick icon
  
  const iconMap = {
    pending: Clock,
    running: Activity,
    completed: CheckCircle2,
    failed: XCircle,
    paused: Pause,
  };
  
  const StatusIcon = iconMap[status] || Clock;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${bg} ${text}`}>
      <StatusIcon className="w-3 h-3" />
      {status}
    </span>
  );
}

function WorkflowCard({ workflow, onClick, onDelete }) {
  const canDelete = workflow.status !== 'running';

  const handleDeleteClick = (e) => {
    e.stopPropagation();
    if (canDelete && onDelete) {
      onDelete(workflow);
    }
  };

  return (
    <div
      onClick={onClick}
      className="relative w-full text-left p-4 rounded-xl border border-border bg-card hover:border-muted-foreground/30 hover:shadow-md transition-all group cursor-pointer"
    >
      <div className="flex items-start justify-between gap-2 mb-3">
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-foreground line-clamp-2">
            {deriveWorkflowTitle(workflow)}
          </p>
          <p className="text-xs text-muted-foreground mt-1 font-mono">{workflow.id}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <StatusBadge status={workflow.status} />
          {canDelete && (
            <button
              onClick={handleDeleteClick}
              className="p-1.5 rounded-lg md:opacity-0 md:group-hover:opacity-100 hover:bg-destructive/10 text-muted-foreground hover:text-destructive transition-all"
              title="Delete workflow"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>
      <div className="flex items-center gap-4 text-xs text-muted-foreground">
        <span>Phase: {workflow.current_phase || 'N/A'}</span>
        <span>Tasks: {workflow.task_count || 0}</span>
        <ExecutionModeBadge config={workflow.config} variant="inline" />
      </div>
      <ChevronRight className="absolute right-4 bottom-4 w-4 h-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
    </div>
  );
}

function TaskItem({ task, selected, onClick }) {
  const { bg, text } = getStatusColor(task.status);
  
  // Custom icon map for tasks
  const iconMap = {
    pending: Clock,
    running: Loader2,
    completed: CheckCircle2,
    failed: XCircle,
    paused: Pause,
  };
  
  const StatusIcon = iconMap[task.status] || Clock;
  const isRunning = task.status === 'running';

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
        <StatusIcon className={`w-4 h-4 ${text} ${isRunning ? 'animate-spin' : ''}`} />
      </div>
      <div className="flex-1 min-w-0 text-left">
        <p className="text-sm font-medium text-foreground truncate">{task.name || task.id}</p>
        <p className="text-xs text-muted-foreground font-mono mt-0.5">{task.id}</p>
      </div>
      <div className={`px-2 py-0.5 rounded text-[10px] uppercase font-bold tracking-wider ${bg} ${text}`}>
        {task.status}
      </div>
    </button>
  );
}

function WorkflowDetail({ workflow, tasks, onBack }) {
  const {
    startWorkflow,
    pauseWorkflow,
    resumeWorkflow,
    stopWorkflow,
    deleteWorkflow,
    updateWorkflow,
    fetchWorkflow,
    analyzeWorkflow,
    planWorkflow,
    replanWorkflow,
    executeWorkflow,
    loading,
    error,
    clearError,
  } = useWorkflowStore();
  const notifyInfo = useUIStore((s) => s.notifyInfo);
  const notifyError = useUIStore((s) => s.notifyError);
  const navigate = useNavigate();

  // Edit modal state
  const [editModalOpen, setEditModalOpen] = useState(false);
  // Replan modal state
  const [isReplanModalOpen, setReplanModalOpen] = useState(false);

  // Delete confirmation state
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const canDelete = workflow.status !== 'running';
  const canModifyAttachments = workflow.status !== 'running';
  const attachmentInputRef = useRef(null);
  const [attachmentUploading, setAttachmentUploading] = useState(false);

  const handleUploadAttachments = useCallback(async (fileList) => {
    if (!fileList || fileList.length === 0) return;
    if (!canModifyAttachments) return;
    setAttachmentUploading(true);
    try {
      await workflowApi.uploadAttachments(workflow.id, fileList);
      await fetchWorkflow(workflow.id);
      notifyInfo(`Uploaded ${fileList.length} attachment(s)`);
    } catch (err) {
      notifyError(err.message || 'Failed to upload attachments');
    } finally {
      setAttachmentUploading(false);
    }
  }, [workflow.id, canModifyAttachments, fetchWorkflow, notifyInfo, notifyError]);

  const handleAttachmentSelect = (e) => {
    const selected = Array.from(e.target.files || []);
    e.target.value = '';
    handleUploadAttachments(selected);
  };

  const handleDeleteAttachment = useCallback(async (attachment) => {
    if (!canModifyAttachments) return;
    if (!window.confirm(`Delete "${attachment.name}"?`)) return;
    try {
      await workflowApi.deleteAttachment(workflow.id, attachment.id);
      await fetchWorkflow(workflow.id);
      notifyInfo('Attachment deleted');
    } catch (err) {
      notifyError(err.message || 'Failed to delete attachment');
    }
  }, [workflow.id, canModifyAttachments, fetchWorkflow, notifyInfo, notifyError]);

  const handleDownloadAttachment = (attachment) => {
    window.open(`/api/v1/workflows/${workflow.id}/attachments/${attachment.id}/download`, '_blank');
  };

  const handleDownloadArtifacts = () => {
    window.open(`/api/v1/workflows/${workflow.id}/download`, '_blank');
  };

  // Issues store hooks - must be before callbacks that use them
  const {
    setWorkflow: setIssuesWorkflow,
    loadIssues,
    startGeneration,
    updateGenerationProgress,
    cancelGeneration,
  } = useIssuesStore();

  // Handle issues generation mode selection
  const handleIssuesModeSelect = useCallback(async (mode) => {
    setShowIssuesModal(false);
    
    // Set workflow context
    setIssuesWorkflow(workflow.id, workflow.title || workflow.id);

    if (mode === 'fast') {
      // Fast mode: instant generation, then navigate
      setIssuesGenerating(true);
      try {
        const response = await workflowApi.previewIssues(workflow.id, true);
        const issues = response.preview_issues || [];

        if (issues.length > 0) {
          loadIssues(issues, {
            ai_used: response.ai_used,
            ai_errors: response.ai_errors,
          });
          navigate(`/workflows/${workflow.id}/issues`);
        } else {
          notifyError('No issues generated from workflow artifacts');
        }
      } catch (err) {
        notifyError(err.message || 'Failed to generate issues');
      } finally {
        setIssuesGenerating(false);
      }
    } else {
      // AI mode: show loading, start generation with streaming effect
      startGeneration('ai', 10);
      navigate(`/workflows/${workflow.id}/issues`);

      // Generate in background with AI
      try {
        const response = await workflowApi.previewIssues(workflow.id, false);
        const issues = response.preview_issues || [];

        if (response.ai_errors && response.ai_errors.length > 0) {
          console.warn('AI generation errors:', response.ai_errors);
        }

        for (let i = 0; i < issues.length; i++) {
          await new Promise(resolve => setTimeout(resolve, 200));
          updateGenerationProgress(i + 1, issues[i]);
        }

        loadIssues(issues, {
          ai_used: response.ai_used,
          ai_errors: response.ai_errors,
        });
      } catch (err) {
        cancelGeneration();
        notifyError(err.message || 'Failed to generate issues');
        navigate(`/workflows/${workflow.id}`);
      }
    }
  }, [workflow.id, workflow.title, setIssuesWorkflow, loadIssues, startGeneration, updateGenerationProgress, cancelGeneration, navigate, notifyError]);

  const handleDelete = useCallback(async () => {
    const success = await deleteWorkflow(workflow.id);
    if (success) {
      notifyInfo('Workflow deleted');
      navigate('/workflows');
    } else {
      notifyError('Failed to delete workflow');
    }
  }, [workflow.id, deleteWorkflow, notifyInfo, notifyError, navigate]);
  // Title can be edited anytime except when running
  // Prompt can only be edited when pending
  const canEdit = workflow.status !== 'running';
  const canEditPrompt = workflow.status === 'pending';
  const displayTitle = workflow.title || deriveWorkflowTitle(workflow, tasks);

  const handleSaveWorkflow = useCallback(async (updates) => {
    try {
      await updateWorkflow(workflow.id, updates);
      notifyInfo('Workflow updated');
    } catch (err) {
      notifyError(err.message || 'Failed to update workflow');
      throw err;
    }
  }, [workflow.id, updateWorkflow, notifyInfo, notifyError]);

  // Agent activity
  const [activityExpanded, setActivityExpanded] = useState(true);
  const [attachmentsExpanded, setAttachmentsExpanded] = useState(false);
  
  // Issues generation state
  const [showIssuesModal, setShowIssuesModal] = useState(false);
  const [issuesGenerating, setIssuesGenerating] = useState(false);

  const agentActivityMap = useAgentStore((s) => s.agentActivity);
  const currentAgentsMap = useAgentStore((s) => s.currentAgents);

  const agentActivity = useMemo(
    () => agentActivityMap[workflow?.id] || [],
    [agentActivityMap, workflow?.id]
  );

  const activeAgents = useMemo(() => {
    const agents = currentAgentsMap[workflow?.id] || {};
    return Object.entries(agents)
      .filter(([, info]) => ['started', 'thinking', 'tool_use', 'progress'].includes(info.status))
      .map(([name, info]) => ({ name, ...info }));
  }, [currentAgentsMap, workflow?.id]);

  const cacheRef = useRef(new Map());
  const [artifactsLoading, setArtifactsLoading] = useState(false);
  const [artifactsError, setArtifactsError] = useState(null);
  const [artifactIndex, setArtifactIndex] = useState(null);

  const [selectedDoc, setSelectedDoc] = useState(null);
  const [docLoading, setDocLoading] = useState(false);
  const [docError, setDocError] = useState(null);
  const [docContent, setDocContent] = useState('');
  const [copied, setCopied] = useState(false);

  const inferReportPath = useCallback(async (workflowId) => {
    try {
      const entries = await fileApi.list('.quorum/runs');
      // Report directories are named directly after the workflowId (e.g., wf-20250121-153045-k7m9p)
      const match = entries.find((e) => e.is_dir && e.name === workflowId);
      return match?.path || null;
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
            .sort((a, b) => (parseNumber(a.name) || 0) - (parseNumber(b.name) || 0)),
        );
      }

      // Also check global .quorum/tasks/ directory for task files
      if (planTaskFiles.length === 0) {
        const globalTaskEntries = (await safeList('.quorum/tasks')) || [];
        planTaskFiles.push(
          ...globalTaskEntries
            .filter((e) => !e.is_dir && /\.md$/i.test(e.name))
            .sort((a, b) => (parseNumber(a.name) || 0) - (parseNumber(b.name) || 0)),
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
    // Collapse activity panel to show task content prominently
    setActivityExpanded(false);

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
  }, [taskPlanById, workflow.id, setActivityExpanded]);

  const handleCopy = useCallback(async () => {
    if (!docContent) return;
    try {
      await navigator.clipboard.writeText(docContent);
      setCopied(true);
      notifyInfo('Copied to clipboard');
      setTimeout(() => setCopied(false), 2000);
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

  const [activeMobileTab, setActiveMobileTab] = useState('tasks'); // 'tasks', 'preview', 'activity'
  const [taskView, setTaskView] = useState('list'); // 'list' | 'graph'

  // Contextual Log Filtering
  const selectedTaskId = useMemo(() => {
    if (!selectedDoc?.key) return null;
    // Match task-plan:task-ID or task-output:task-ID
    const match = selectedDoc.key.match(/^task-(?:plan|output):(.+)$/);
    return match ? match[1] : null;
  }, [selectedDoc]);

  const filteredActivity = useMemo(() => {
    if (!selectedTaskId) return agentActivity;
    return agentActivity.filter(entry => {
      // Check if event data contains specific task_id
      if (entry.data?.task_id === selectedTaskId) return true;
      // Fallback: check if message mentions task ID (less precise but useful)
      if (entry.message && entry.message.includes(selectedTaskId)) return true;
      // Fallback: check if task name is in message (if available)
      const taskName = tasks.find(t => t.id === selectedTaskId)?.name;
      if (taskName && entry.message && entry.message.includes(taskName)) return true;
      
      return false;
    });
  }, [agentActivity, selectedTaskId, tasks]);

  return (
    <div className="space-y-6 animate-fade-in pb-10">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 space-y-6">
      {/* Header */}
      <div className="md:sticky md:top-14 z-20 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 -mx-4 sm:-mx-6 px-4 sm:px-6 py-4 border-b border-border shadow-sm transition-all">
        <div className="flex flex-col md:flex-row md:items-center gap-4">
          <div className="flex items-center gap-4 w-full md:w-auto">
            <button
              onClick={onBack}
              className="p-2 rounded-lg hover:bg-accent transition-colors shrink-0"
            >
              <ArrowLeft className="w-5 h-5 text-muted-foreground" />
            </button>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 group">
                <h1 className="text-xl font-semibold text-foreground line-clamp-1">{displayTitle}</h1>
                {canEdit && (
                  <button
                    onClick={() => setEditModalOpen(true)}
                    className="p-1.5 rounded-lg opacity-0 group-hover:opacity-100 hover:bg-accent text-muted-foreground hover:text-foreground transition-all"
                    title="Edit workflow"
                  >
                    <Pencil className="w-4 h-4" />
                  </button>
                )}
              </div>
              <div className="flex items-center gap-3 mt-1">
                <p className="text-sm text-muted-foreground">{workflow.id}</p>
                <ExecutionModeBadge config={workflow.config} variant="inline" />
              </div>
            </div>
          </div>

          {/* Phase Progress Stepper - inline */}
          <div className="hidden md:flex flex-1 justify-center">
            <PhaseStepper workflow={workflow} compact />
          </div>

          <div className="flex items-center gap-2 flex-wrap md:justify-end w-full md:w-auto">
            {workflow.status === 'running' && (
              <AgentActivityCompact activeAgents={activeAgents} />
            )}

            {/* Phase action buttons - inline */}
            {workflow.status === 'pending' && (
              <>
                <button
                  onClick={() => startWorkflow(workflow.id)}
                  disabled={loading}
                  className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-all"
                >
                  {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <FastForward className="w-4 h-4" />}
                  Run All
                </button>
                <button
                  onClick={() => analyzeWorkflow(workflow.id)}
                  disabled={loading}
                  className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-info/10 text-info text-sm font-medium hover:bg-info/20 disabled:opacity-50 transition-all"
                >
                  {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                  Analyze
                </button>
              </>
            )}

            {/* Plan button - after analyze completes */}
            {workflow.status === 'completed' && workflow.current_phase === 'plan' && (
              <button
                onClick={() => planWorkflow(workflow.id)}
                disabled={loading}
                className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-info/10 text-info text-sm font-medium hover:bg-info/20 disabled:opacity-50 transition-all"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                Plan
              </button>
            )}

            {/* Replan button - when plan or execute phase completed */}
            {workflow.status === 'completed' && (workflow.current_phase === 'plan' || workflow.current_phase === 'execute') && (
              <button
                onClick={() => setReplanModalOpen(true)}
                disabled={loading}
                className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-warning/10 text-warning text-sm font-medium hover:bg-warning/20 disabled:opacity-50 transition-all"
              >
                <RefreshCw className="w-4 h-4" />
                Replan
              </button>
            )}

            {/* Execute button - after plan completes */}
            {workflow.status === 'completed' && workflow.current_phase === 'execute' && (
              <button
                onClick={() => executeWorkflow(workflow.id)}
                disabled={loading}
                className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-success/10 text-success text-sm font-medium hover:bg-success/20 disabled:opacity-50 transition-all"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                Execute
              </button>
            )}

            {/* Create Issues button - when execution is done or partly done */}
            {['execute', 'done'].includes(workflow.current_phase) && (
              <button
                onClick={() => setShowIssuesModal(true)}
                disabled={issuesGenerating}
                className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 text-sm font-medium hover:bg-indigo-500/20 disabled:opacity-50 transition-all"
              >
                {issuesGenerating ? <Loader2 className="w-4 h-4 animate-spin" /> : <FileText className="w-4 h-4" />}
                Create Issues
              </button>
            )}

            {/* Resume button - for paused workflows */}
            {workflow.status === 'paused' && (
              <button
                onClick={() => resumeWorkflow(workflow.id)}
                disabled={loading}
                className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-all"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                Resume
              </button>
            )}

            {/* Retry button - for failed workflows */}
            {workflow.status === 'failed' && (
              <button
                onClick={() => startWorkflow(workflow.id)}
                disabled={loading}
                className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-all"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RotateCcw className="w-4 h-4" />}
                Retry
              </button>
            )}

            {/* Running indicator */}
            {workflow.status === 'running' && (
              <div className="inline-flex items-center gap-2 px-3 py-2 rounded-lg bg-info/10 text-info text-sm">
                <Loader2 className="w-4 h-4 animate-spin" />
                {workflow.current_phase ? `Running ${workflow.current_phase}...` : 'Running...'}
              </div>
            )}

            {/* Pause/Stop controls - when running */}
            {workflow.status === 'running' && (
              <>
                <button
                  onClick={() => pauseWorkflow(workflow.id)}
                  className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 transition-colors"
                >
                  <Pause className="w-4 h-4" />
                  Pause
                </button>
                <button
                  onClick={() => stopWorkflow(workflow.id)}
                  className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-destructive text-destructive-foreground text-sm font-medium hover:bg-destructive/90 transition-colors"
                >
                  <StopCircle className="w-4 h-4" />
                  Stop
                </button>
              </>
            )}

                      {/* Stop when paused */}
                      {workflow.status === 'paused' && (
                        <button
                          onClick={() => stopWorkflow(workflow.id)}
                          className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-destructive text-destructive-foreground text-sm font-medium hover:bg-destructive/90 transition-colors"
                        >
                          <StopCircle className="w-4 h-4" />
                          Stop
                        </button>
                      )}
            
                      {/* Download Artifacts */}
                      {workflow.status !== 'pending' && (
                        <button
                          onClick={handleDownloadArtifacts}
                          className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 transition-colors"
                          title="Download artifacts"
                        >
                          <Download className="w-4 h-4" />
                          Download
                        </button>
                      )}
            
                      {/* Delete button */}            {canDelete && (
              <button
                onClick={() => setDeleteDialogOpen(true)}
                className="flex-1 md:flex-none inline-flex justify-center items-center gap-2 px-3 py-2 rounded-lg bg-destructive/10 text-destructive text-sm font-medium hover:bg-destructive/20 transition-colors"
                title="Delete workflow"
              >
                <Trash2 className="w-4 h-4" />
                Delete
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Workflow Error Banner - Shows when workflow failed */}
      {workflow.status === 'failed' && workflow.error && (
        <div className="p-4 rounded-lg bg-destructive/10 border border-destructive/20">
          <div className="flex items-start gap-3">
            <XCircle className="w-5 h-5 text-destructive flex-shrink-0 mt-0.5" />
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-destructive mb-1">Workflow Failed</p>
              <p className="text-sm text-destructive/80 break-words">{workflow.error}</p>
            </div>
          </div>
        </div>
      )}

      {/* Store Error Banner */}
      {error && (
        <div className="p-4 rounded-lg bg-warning/10 border border-warning/20 flex items-start justify-between">
          <p className="text-sm text-warning">{error}</p>
          <button onClick={clearError} className="text-warning hover:text-warning/80 text-sm">
            Dismiss
          </button>
        </div>
      )}

      {/* Info Card */}
      <div className="p-4 rounded-xl border border-border bg-card">
        <div className="flex items-start justify-between mb-3">
          <div className="flex-1 min-w-0">
            <p className="text-sm text-muted-foreground mb-2 line-clamp-3">
              {workflow.prompt || 'No prompt'}
            </p>
            <div className="flex flex-wrap items-center gap-3">
              <StatusBadge status={workflow.status} />
              <ExecutionModeBadge config={workflow.config} variant="badge" />
            </div>
          </div>
          {canEdit && (
            <button
              onClick={() => setEditModalOpen(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-accent transition-colors ml-4 shrink-0"
            >
              <Pencil className="w-3.5 h-3.5" />
              Edit
            </button>
          )}
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-3 pt-3 border-t border-border">
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

      {/* Mobile Tab Bar */}
      <div className="md:hidden flex border-b border-border mb-4">
        <button
          onClick={() => setActiveMobileTab('tasks')}
          className={`flex-1 pb-3 text-sm font-medium text-center border-b-2 transition-colors ${
            activeMobileTab === 'tasks'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          <div className="flex items-center justify-center gap-2">
            <List className="w-4 h-4" />
            Tasks
          </div>
        </button>
        <button
          onClick={() => setActiveMobileTab('preview')}
          className={`flex-1 pb-3 text-sm font-medium text-center border-b-2 transition-colors ${
            activeMobileTab === 'preview'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          <div className="flex items-center justify-center gap-2">
            <FileText className="w-4 h-4" />
            Preview
          </div>
        </button>
        <button
          onClick={() => setActiveMobileTab('activity')}
          className={`flex-1 pb-3 text-sm font-medium text-center border-b-2 transition-colors ${
            activeMobileTab === 'activity'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          <div className="flex items-center justify-center gap-2">
            <Activity className="w-4 h-4" />
            Activity
          </div>
        </button>
      </div>

      {/* Agent Activity Panel - Conditionally visible on mobile */}
      <div className={`${activeMobileTab === 'activity' ? 'block' : 'hidden'} md:block`}>
        {(agentActivity.length > 0 || activeAgents.length > 0) && (
          <>
            {selectedTaskId && filteredActivity.length !== agentActivity.length && (
              <div className="mb-2 px-4 py-2 bg-accent/50 rounded-lg flex items-center justify-between text-xs">
                <span className="text-muted-foreground">
                  Showing logs for <span className="font-mono font-medium text-foreground">{selectedTaskId}</span> ({filteredActivity.length}/{agentActivity.length} events)
                </span>
                {/* Optional: Add a button to disable filtering while keeping doc selected if desired */}
              </div>
            )}
            <AgentActivity
              workflowId={workflow.id}
              activity={filteredActivity}
              activeAgents={activeAgents}
              expanded={activityExpanded}
              onToggle={() => setActivityExpanded(!activityExpanded)}
              workflowStartTime={workflow.status === 'running' ? workflow.updated_at : null}
            />
          </>
        )}
      </div>

      {/* Inspector */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className={`space-y-6 ${activeMobileTab === 'tasks' ? 'block' : 'hidden'} md:block`}>
          {/* Attachments */}
          <div className="p-4 rounded-xl border border-border bg-card">
            <div className="flex items-center justify-between mb-3">
              <button
                type="button"
                onClick={() => setAttachmentsExpanded(!attachmentsExpanded)}
                className="flex items-center gap-2 text-sm font-semibold text-foreground hover:text-primary transition-colors"
              >
                {attachmentsExpanded ? (
                  <ChevronUp className="w-4 h-4" />
                ) : (
                  <ChevronDown className="w-4 h-4" />
                )}
                Attachments ({workflow.attachments?.length || 0})
              </button>
              {canModifyAttachments && (
                <div className="flex items-center gap-2">
                  <input
                    ref={attachmentInputRef}
                    type="file"
                    multiple
                    className="hidden"
                    disabled={attachmentUploading}
                    onChange={handleAttachmentSelect}
                  />
                  <button
                    type="button"
                    onClick={() => attachmentInputRef.current?.click()}
                    disabled={attachmentUploading}
                    className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 disabled:opacity-50 transition-colors"
                  >
                    {attachmentUploading ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <Upload className="w-4 h-4" />
                    )}
                    Upload
                  </button>
                </div>
              )}
            </div>

            {attachmentsExpanded && (
              <>
                {workflow.attachments && workflow.attachments.length > 0 ? (
                  <div className="space-y-2">
                    {workflow.attachments.map((a) => (
                      <div
                        key={a.id}
                        className="flex items-center justify-between gap-3 p-2 rounded-lg border border-border bg-background"
                      >
                        <div className="min-w-0">
                          <p className="text-sm font-medium text-foreground truncate">{a.name}</p>
                          <p className="text-xs text-muted-foreground">
                            {a.size >= 1024 ? `${Math.round(a.size / 1024)} KB` : `${a.size} B`}
                          </p>
                        </div>
                        <div className="flex items-center gap-1 shrink-0">
                          <button
                            type="button"
                            onClick={() => handleDownloadAttachment(a)}
                            className="p-2 rounded-lg hover:bg-accent transition-colors"
                            title="Download"
                          >
                            <Download className="w-4 h-4 text-muted-foreground" />
                          </button>
                          {canModifyAttachments && (
                            <button
                              type="button"
                              onClick={() => handleDeleteAttachment(a)}
                              className="p-2 rounded-lg hover:bg-destructive/10 transition-colors"
                              title="Delete"
                            >
                              <Trash2 className="w-4 h-4 text-destructive" />
                            </button>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="text-center py-8 text-muted-foreground">
                    <p className="text-sm">No attachments</p>
                    <p className="text-xs mt-1">Upload documents to add context</p>
                  </div>
                )}
              </>
            )}
          </div>

          {/* Tasks */}
          <div className="p-4 rounded-xl border border-border bg-card">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-semibold text-foreground">Tasks ({tasks.length})</h3>
              </div>
              <div className="flex bg-muted/50 p-0.5 rounded-lg">
                <button
                  onClick={() => setTaskView('list')}
                  className={`p-1.5 rounded-md transition-colors ${taskView === 'list' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                  title="List View"
                >
                  <LayoutList className="w-3.5 h-3.5" />
                </button>
                <button
                  onClick={() => setTaskView('graph')}
                  className={`p-1.5 rounded-md transition-colors ${taskView === 'graph' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                  title="Graph View"
                >
                  <Network className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
            
            {taskView === 'list' ? (
              tasks.length > 0 ? (
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
              )
            ) : (
              <WorkflowGraph tasks={tasks} />
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
                <p className="text-xs text-muted-foreground truncate flex items-center gap-1">
                  <FolderTree className="w-3 h-3" />
                  {artifactIndex.reportPath}
                </p>
                <div className="space-y-4 max-h-[45vh] overflow-y-auto pr-1">
                  <FileTree
                    items={docGroups.flatMap(g => g.docs.map(d => ({
                      ...d,
                      treePath: `${g.label}/${d.title}` // Artificial path for tree structure, preserves original path
                    })))}
                    onSelect={setSelectedDoc}
                    selectedKey={selectedDoc?.key}
                  />
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Preview */}
        <div className={`lg:col-span-2 p-6 rounded-xl border border-border bg-card flex flex-col max-h-[80vh] ${activeMobileTab === 'preview' ? 'block' : 'hidden'} md:flex`}>
          <div className="flex items-start justify-between gap-4 mb-4 flex-none">
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
                className={`p-2 rounded-lg hover:bg-accent disabled:opacity-50 transition-colors ${copied ? 'text-green-500' : ''}`}
                title={copied ? 'Copied!' : 'Copy raw markdown'}
              >
                {copied ? (
                  <CheckCircle2 className="w-4 h-4" />
                ) : (
                  <Copy className="w-4 h-4 text-muted-foreground" />
                )}
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

          <div className="flex-1 overflow-y-auto min-h-0 pr-2 scrollbar-thin">
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
              selectedDoc.path && !selectedDoc.path.endsWith('.md') ? (
                <CodeEditor 
                  value={docContent} 
                  language={selectedDoc.path.split('.').pop()} 
                  readOnly={true} 
                />
              ) : (
                <MarkdownViewer markdown={docContent} />
              )
            ) : (
              <div className="text-sm text-muted-foreground">
                Select a task or document to preview.
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Edit Modal */}
      <EditWorkflowModal
        isOpen={editModalOpen}
        onClose={() => setEditModalOpen(false)}
        workflow={workflow}
        onSave={handleSaveWorkflow}
        canEditPrompt={canEditPrompt}
      />

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        onConfirm={handleDelete}
        title="Delete Workflow?"
        message={`This will permanently delete "${displayTitle}" and all its associated data. This action cannot be undone.`}
        confirmText="Delete"
        variant="danger"
      />

      {/* Replan Modal */}
      <ReplanModal
        isOpen={isReplanModalOpen}
        onClose={() => setReplanModalOpen(false)}
        onSubmit={async (context) => {
          await replanWorkflow(workflow.id, context);
          setReplanModalOpen(false);
        }}
        loading={loading}
      />

      {/* Issues Generation Modal */}
      <GenerationOptionsModal
        isOpen={showIssuesModal}
        onClose={() => setShowIssuesModal(false)}
        onSelect={handleIssuesModeSelect}
        loading={issuesGenerating}
      />
      </div>
    </div>
  );
}

const AGENT_OPTIONS = [
  { value: 'claude', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'codex', label: 'Codex' },
];

function NewWorkflowForm({ onSubmit, onCancel, loading }) {
  const location = useLocation();
  const navigate = useNavigate();
  const template = location.state?.template;
  
  const [title, setTitle] = useState('');
  const [prompt, setPrompt] = useState('');
  const [files, setFiles] = useState([]);
  const fileInputRef = useRef(null);

  // Execution mode state
  const [executionMode, setExecutionMode] = useState('multi_agent');
  const [singleAgentName, setSingleAgentName] = useState('claude');
  const [singleAgentModel, setSingleAgentModel] = useState('');
  const [singleAgentReasoningEffort, setSingleAgentReasoningEffort] = useState('');

  // Subscribe for enums updates (models/reasoning)
  useEnums();

  // Apply template if provided
  useEffect(() => {
    if (template) {
      if (template.name) setTitle(template.name);
      if (template.prompt) setPrompt(template.prompt);
      if (template.executionStrategy === 'single-agent') {
        setExecutionMode('single_agent');
      } else if (template.executionStrategy === 'multi-agent-consensus') {
        setExecutionMode('multi_agent');
      }
    }
  }, [template]);

  // Get enabled agents from config store
  const { config } = useConfigStore();
  const enabledAgents = useMemo(() => {
    if (!config?.agents) return AGENT_OPTIONS;
    return AGENT_OPTIONS.filter(opt => config.agents[opt.value]?.enabled !== false);
  }, [config]);

  const effectiveSingleAgentName = enabledAgents.some((a) => a.value === singleAgentName)
    ? singleAgentName
    : (enabledAgents[0]?.value || singleAgentName);

  const modelOptions = getModelsForAgent(effectiveSingleAgentName);
  const reasoningLevels = getReasoningLevels();
  const agentSupportsReasoning = supportsReasoning(effectiveSingleAgentName);

  const effectiveSingleAgentModel = modelOptions.some((m) => m.value === singleAgentModel)
    ? singleAgentModel
    : '';
  const effectiveSingleAgentReasoningEffort = agentSupportsReasoning && reasoningLevels.some((r) => r.value === singleAgentReasoningEffort)
    ? singleAgentReasoningEffort
    : '';

  const selectedModel = modelOptions.find((m) => m.value === effectiveSingleAgentModel);
  const selectedReasoning = reasoningLevels.find((r) => r.value === effectiveSingleAgentReasoningEffort);

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!prompt.trim()) return;

    // Build config based on execution mode
    const workflowConfig = executionMode === 'single_agent'
      ? {
          execution_mode: 'single_agent',
          single_agent_name: effectiveSingleAgentName,
          ...(effectiveSingleAgentModel ? { single_agent_model: effectiveSingleAgentModel } : {}),
          ...(agentSupportsReasoning && effectiveSingleAgentReasoningEffort
            ? { single_agent_reasoning_effort: effectiveSingleAgentReasoningEffort }
            : {}),
        }
      : undefined;

    onSubmit(prompt, files, title.trim() || undefined, workflowConfig);
  };

  const handleFilesSelected = (e) => {
    const selected = Array.from(e.target.files || []);
    e.target.value = '';
    if (selected.length === 0) return;
    setFiles((prev) => [...prev, ...selected]);
  };

  const removeFile = (index) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  };

  return (
    <div className="w-full animate-fade-in pb-10">
      <div className="max-w-2xl mx-auto p-6 rounded-xl border border-border bg-card animate-fade-up">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold text-foreground">Create New Workflow</h2>
          <Link
            to="/templates"
            className="inline-flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-primary hover:text-primary/80 bg-primary/10 hover:bg-primary/20 rounded-lg transition-colors"
          >
            <Sparkles className="w-4 h-4" />
            Browse Templates
          </Link>
        </div>
      <form onSubmit={handleSubmit} className="space-y-8">
        {/* Step 1: Definition */}
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              Title <span className="text-muted-foreground font-normal">(optional)</span>
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Give your workflow a descriptive name..."
              className="w-full px-3 py-2 border border-input rounded-lg bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              Prompt
            </label>
            <div className="relative">
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder="Describe what you want the AI agents to accomplish..."
                rows={6}
                spellCheck={false}
                className="w-full px-3 py-2 pr-12 border border-input rounded-lg bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background resize-none font-mono text-sm leading-6"
              />
              <VoiceInputButton
                onTranscript={(text) => setPrompt((prev) => (prev ? prev + ' ' + text : text))}
                disabled={loading}
                className="absolute top-2 right-2"
              />
            </div>
          </div>
        </div>

        {/* Step 2: Strategy */}
        <div className="space-y-4 pt-4 border-t border-border">
           <h3 className="text-sm font-medium text-foreground">Execution Strategy</h3>
           
           <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {/* Multi-Agent Tile */}
            <button
              type="button"
              onClick={() => setExecutionMode('multi_agent')}
              className={`relative flex flex-col items-start p-4 rounded-xl border-2 transition-all text-left ${
                executionMode === 'multi_agent'
                  ? 'border-primary bg-primary/5 shadow-sm'
                  : 'border-border bg-background hover:border-muted-foreground/30 hover:bg-accent'
              }`}
            >
              <div className="flex items-center justify-between w-full mb-2">
                <Network className={`w-6 h-6 ${executionMode === 'multi_agent' ? 'text-primary' : 'text-muted-foreground'}`} />
                {executionMode === 'multi_agent' && (
                  <CheckCircle2 className="w-4 h-4 text-primary" />
                )}
              </div>
              <span className="font-semibold text-foreground text-sm">Multi-Agent Consensus</span>
              <span className="text-xs text-muted-foreground mt-1">Iterative refinement and debate between agents. Best for complex tasks.</span>
            </button>

            {/* Single-Agent Tile */}
            <button
              type="button"
              onClick={() => setExecutionMode('single_agent')}
              className={`relative flex flex-col items-start p-4 rounded-xl border-2 transition-all text-left ${
                executionMode === 'single_agent'
                  ? 'border-primary bg-primary/5 shadow-sm'
                  : 'border-border bg-background hover:border-muted-foreground/30 hover:bg-accent'
              }`}
            >
              <div className="flex items-center justify-between w-full mb-2">
                <Zap className={`w-6 h-6 ${executionMode === 'single_agent' ? 'text-primary' : 'text-muted-foreground'}`} />
                {executionMode === 'single_agent' && (
                  <CheckCircle2 className="w-4 h-4 text-primary" />
                )}
              </div>
              <span className="font-semibold text-foreground text-sm">Single Agent</span>
              <span className="text-xs text-muted-foreground mt-1">Fast, direct execution by one specialized model. Best for simple tasks.</span>
            </button>
           </div>

          {/* Config Panel - Smooth Expand */}
          <div className={`overflow-hidden transition-all duration-300 ease-in-out ${executionMode === 'single_agent' ? 'max-h-[500px] opacity-100' : 'max-h-0 opacity-0'}`}>
            <div className="mt-4 p-4 rounded-xl border border-border bg-muted/30 space-y-4">
              <div className="flex items-center gap-2 mb-2">
                <Pencil className="w-4 h-4 text-primary" />
                <h4 className="text-sm font-medium text-foreground">Agent Configuration</h4>
              </div>
              
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-muted-foreground mb-1.5">
                    Agent Provider
                  </label>
                  <select
                    value={effectiveSingleAgentName}
                    onChange={(e) => {
                      const nextAgent = e.target.value;
                      setSingleAgentName(nextAgent);
                      setSingleAgentModel('');
                      setSingleAgentReasoningEffort('');
                    }}
                    className="w-full px-3 py-2 border border-input rounded-lg bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                  >
                    {enabledAgents.map(agent => (
                      <option key={agent.value} value={agent.value}>
                        {agent.label}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-muted-foreground mb-1.5">
                    Model
                  </label>
                  <select
                    value={effectiveSingleAgentModel}
                    onChange={(e) => setSingleAgentModel(e.target.value)}
                    className="w-full px-3 py-2 border border-input rounded-lg bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                  >
                    {modelOptions.map((model) => (
                      <option key={`${effectiveSingleAgentName}-${model.value || 'default'}`} value={model.value}>
                        {model.label}
                      </option>
                    ))}
                  </select>
                </div>

                {agentSupportsReasoning && (
                  <div className="sm:col-span-2">
                    <label className="block text-xs font-medium text-muted-foreground mb-1.5">
                      Reasoning Effort
                    </label>
                    <select
                      value={effectiveSingleAgentReasoningEffort}
                      onChange={(e) => setSingleAgentReasoningEffort(e.target.value)}
                      className="w-full px-3 py-2 border border-input rounded-lg bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                    >
                      <option value="">Default</option>
                      {reasoningLevels.map((level) => (
                        <option key={level.value} value={level.value}>
                          {level.label}
                        </option>
                      ))}
                    </select>
                    {selectedReasoning?.description && (
                      <p className="mt-1.5 text-xs text-muted-foreground">
                        {selectedReasoning.description}
                      </p>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Attachments */}
        <div className="pt-4 border-t border-border">
          <label className="block text-sm font-medium text-foreground mb-2">
            Attachments (optional)
          </label>
          <div className="flex flex-wrap items-center gap-3">
            <input
              ref={fileInputRef}
              type="file"
              multiple
              onChange={handleFilesSelected}
              className="hidden"
              disabled={loading}
            />
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              disabled={loading}
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-input bg-background hover:bg-accent hover:text-accent-foreground text-sm font-medium transition-colors"
            >
              <Upload className="w-4 h-4" />
              Upload Files
            </button>
            <p className="text-xs text-muted-foreground">
              Supports code files, text, and images.
            </p>
          </div>
          {files.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {files.map((f, idx) => (
                <div
                  key={`${f.name}-${f.size}-${idx}`}
                  className="inline-flex items-center gap-2 pl-3 pr-1 py-1 rounded-full bg-secondary text-secondary-foreground text-xs border border-border"
                >
                  <span className="truncate max-w-[150px]">{f.name}</span>
                  <button
                    type="button"
                    onClick={() => removeFile(idx)}
                    className="p-1 hover:bg-destructive/10 hover:text-destructive rounded-full transition-colors"
                    title="Remove"
                  >
                    <XCircle className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="flex gap-3 pt-4">
          <button
            type="submit"
            disabled={loading || !prompt.trim()}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors shadow-sm"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
            Start Workflow
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2.5 rounded-lg border border-input bg-background hover:bg-accent hover:text-accent-foreground text-sm font-medium transition-colors"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  </div>
);
}

// Status Filter Tabs
const STATUS_FILTERS = [
  { value: 'all', label: 'All', icon: null },
  { value: 'running', label: 'Running', icon: Activity },
  { value: 'completed', label: 'Completed', icon: CheckCircle2 },
  { value: 'failed', label: 'Failed', icon: XCircle },
];

function StatusFilterTabs({ status, setStatus }) {
  return (
    <div className="flex items-center gap-1 p-1 rounded-lg bg-muted/50">
      {STATUS_FILTERS.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          onClick={() => setStatus(value)}
          className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-all ${
            status === value
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          {Icon && <Icon className="w-3.5 h-3.5" />}
          {label}
        </button>
      ))}
    </div>
  );
}

// Text Filter Component
function WorkflowFilters({ filter, setFilter }) {
  return (
    <div className="relative w-full sm:w-64">
      <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
      <input
        type="text"
        placeholder="Search workflows..."
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className="h-10 w-full pl-9 pr-4 rounded-lg border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring/20 transition-all"
      />
    </div>
  );
}

export default function Workflows() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { workflows, loading, error, fetchWorkflows, fetchWorkflow, createWorkflow, deleteWorkflow, clearError } = useWorkflowStore();
  const { getTasksForWorkflow, setTasks } = useTaskStore();
  const notifyInfo = useUIStore((s) => s.notifyInfo);
  const notifyError = useUIStore((s) => s.notifyError);
  const [showNewForm, setShowNewForm] = useState(false);
  const [filter, setFilter] = useState('');

  // Get status filter from URL params
  const statusFilter = searchParams.get('status') || 'all';
  const setStatusFilter = useCallback((status) => {
    if (status === 'all') {
      searchParams.delete('status');
    } else {
      searchParams.set('status', status);
    }
    setSearchParams(searchParams);
  }, [searchParams, setSearchParams]);

  // Delete from list state
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [workflowToDelete, setWorkflowToDelete] = useState(null);

  useEffect(() => {
    fetchWorkflows();
  }, [fetchWorkflows]);

  useEffect(() => {
    if (id && id !== 'new') {
      fetchWorkflow(id);
    }
  }, [fetchWorkflow, id]);

  // Filter workflows by status and text
  const filteredWorkflows = useMemo(() => {
    let result = workflows;

    // Apply status filter
    if (statusFilter && statusFilter !== 'all') {
      result = result.filter(w => w.status === statusFilter);
    }

    // Apply text filter
    if (filter) {
      const lowerFilter = filter.toLowerCase();
      result = result.filter(w =>
        (deriveWorkflowTitle(w).toLowerCase().includes(lowerFilter)) ||
        (w.id && w.id.toLowerCase().includes(lowerFilter)) ||
        (w.prompt && w.prompt.toLowerCase().includes(lowerFilter))
      );
    }

    return result;
  }, [workflows, filter, statusFilter]);

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

  const handleCreate = async (prompt, files = [], title, workflowConfig) => {
    const workflow = await createWorkflow(prompt, { title, config: workflowConfig });
    if (!workflow) {
      // Get the error from the store and show it
      const storeError = useWorkflowStore.getState().error;
      if (storeError) {
        notifyError(storeError);
        clearError();
      } else {
        notifyError('Failed to create workflow');
      }
      return;
    }

    if (files.length > 0) {
      try {
        await workflowApi.uploadAttachments(workflow.id, files);
        await fetchWorkflow(workflow.id);
        notifyInfo(`Uploaded ${files.length} attachment(s)`);
      } catch (err) {
        notifyError(err.message || 'Failed to upload attachments');
      }
    }

    setShowNewForm(false);
    navigate(`/workflows/${workflow.id}`);
  };

  // Delete from list handlers
  const handleDeleteClick = useCallback((workflow) => {
    setWorkflowToDelete(workflow);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteConfirm = useCallback(async () => {
    if (!workflowToDelete) return;
    const success = await deleteWorkflow(workflowToDelete.id);
    if (success) {
      notifyInfo('Workflow deleted');
    } else {
      notifyError('Failed to delete workflow');
    }
    setWorkflowToDelete(null);
    setDeleteDialogOpen(false);
  }, [workflowToDelete, deleteWorkflow, notifyInfo, notifyError]);

  const handleDeleteCancel = useCallback(() => {
    setWorkflowToDelete(null);
    setDeleteDialogOpen(false);
  }, []);

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
    <div className="space-y-6 animate-fade-in pb-10">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 space-y-6">
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-foreground tracking-tight">Workflows</h1>
            <p className="text-sm text-muted-foreground mt-1">Manage your AI automation workflows</p>
          </div>
          <div className="flex flex-col sm:flex-row gap-3 w-full sm:w-auto">
            <WorkflowFilters filter={filter} setFilter={setFilter} />
            <Link
              to="/workflows/new"
              className="flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors shadow-sm whitespace-nowrap"
            >
              <Plus className="w-4 h-4" />
              New Workflow
            </Link>
          </div>
        </div>

        {/* Status Filter Tabs */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-border/50 pb-4">
          <div className="flex items-center gap-2 overflow-x-auto no-scrollbar max-w-full pb-1 -mb-1 mask-linear-fade">
            <StatusFilterTabs status={statusFilter} setStatus={setStatusFilter} />
          </div>
          {statusFilter !== 'all' && (
            <div className="hidden sm:block text-xs text-muted-foreground whitespace-nowrap px-1">
              Showing {filteredWorkflows.length} {statusFilter} workflow{filteredWorkflows.length !== 1 ? 's' : ''}
            </div>
          )}
        </div>
      </div>

      {loading && workflows.length === 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="h-32 rounded-xl bg-muted animate-pulse" />
          ))}
        </div>
      ) : filteredWorkflows.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredWorkflows.map((workflow) => (
            <WorkflowCard
              key={workflow.id}
              workflow={workflow}
              onClick={() => navigate(`/workflows/${workflow.id}`)}
              onDelete={handleDeleteClick}
            />
          ))}
        </div>
      ) : (
        <div className="text-center py-16">
          <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-muted flex items-center justify-center">
            <GitBranch className="w-8 h-8 text-muted-foreground" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-2">No workflows found</h3>
          <p className="text-sm text-muted-foreground mb-4">
            {filter ? 'Try adjusting your search terms' : 'Create your first workflow to get started'}
          </p>
          {!filter && (
            <Link
              to="/workflows/new"
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
            >
              <Plus className="w-4 h-4" />
              Create Workflow
            </Link>
          )}
        </div>
      )}
      
      {/* Mobile FAB */}
      <FAB onClick={() => navigate('/workflows/new')} icon={Plus} label="New Workflow" />

      {/* Delete Confirmation Dialog for list view */}
      <ConfirmDialog
        isOpen={deleteDialogOpen}
        onClose={handleDeleteCancel}
        onConfirm={handleDeleteConfirm}
        title="Delete Workflow?"
        message={workflowToDelete ? `This will permanently delete "${deriveWorkflowTitle(workflowToDelete)}" and all its associated data. This action cannot be undone.` : ''}
        confirmText="Delete"
        variant="danger"
      />
      </div>
    </div>
  );
}
