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
  Info,
  X,
  Terminal,
  Settings
} from 'lucide-react';
import { ConfirmDialog } from '../components/config/ConfirmDialog';
import { ExecutionModeBadge, PhaseStepper, ReplanModal } from '../components/workflow';
import { GenerationOptionsModal } from '../components/issues';
import useIssuesStore from '../stores/issuesStore';
import FileTree from '../components/FileTree';
import CodeEditor from '../components/CodeEditor';
import WorkflowGraph from '../components/WorkflowGraph';
import { Card, CardHeader, CardTitle, CardDescription } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';

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
  
  const iconMap = {
    pending: Clock,
    running: Activity,
    completed: CheckCircle2,
    failed: XCircle,
    paused: Pause,
  };
  
  const StatusIcon = iconMap[status] || Clock;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wide ${bg} ${text}`}>
      <StatusIcon className="w-3 h-3" />
      {status}
    </span>
  );
}

function WorkflowCard({ workflow, onClick, onDelete }) {
  const canDelete = workflow.status !== 'running';
  const statusColor = getStatusColor(workflow.status);

  const handleDeleteClick = (e) => {
    e.stopPropagation();
    if (canDelete && onDelete) {
      onDelete(workflow);
    }
  };

  return (
    <div
      onClick={onClick}
      className={`group flex flex-col h-full rounded-xl border border-border bg-card/50 backdrop-blur-sm transition-all duration-300 hover:shadow-xl hover:-translate-y-1 overflow-hidden border-l-[4px] ${statusColor.borderStrip ? '' : 'border-l-muted'}`}
      style={statusColor.borderStrip ? { borderLeftColor: `var(--${statusColor.borderStrip.split('-')[1]})` } : {}}
    >
      <div className="flex-1 p-5 space-y-4 cursor-pointer">
        <div className="flex items-start justify-between gap-3">
          <div className={`p-2.5 rounded-lg bg-background border border-border shadow-sm group-hover:border-primary/30 transition-colors`}>
            <GitBranch className="h-5 w-5 text-primary" />
          </div>
          <div className="flex items-center gap-2">
             <StatusBadge status={workflow.status} />
             {canDelete && (
                <button
                  onClick={handleDeleteClick}
                  className="p-1.5 rounded-lg opacity-0 group-hover:opacity-100 hover:bg-destructive/10 text-muted-foreground hover:text-destructive transition-all"
                  title="Delete workflow"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              )}
          </div>
        </div>

        <div className="space-y-2">
          <h3 className="font-bold text-lg text-foreground line-clamp-2 leading-tight group-hover:text-primary transition-colors">
            {deriveWorkflowTitle(workflow)}
          </h3>
          <p className="text-xs text-muted-foreground font-mono opacity-60 group-hover:opacity-100 transition-opacity">
            {workflow.id}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-4 pt-2">
           <div className="flex items-center gap-1.5">
              <Layers className="w-3.5 h-3.5 text-muted-foreground" />
              <span className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/70">
                Phase: {workflow.current_phase || 'PENDING'}
              </span>
           </div>
           <div className="flex items-center gap-1.5">
              <LayoutList className="w-3.5 h-3.5 text-muted-foreground" />
              <span className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/70">
                {workflow.task_count || 0} Tasks
              </span>
           </div>
        </div>
      </div>

      <div className="p-4 pt-0 mt-auto flex gap-2">
         <div className="flex-1">
            <ExecutionModeBadge config={workflow.config} variant="badge" />
         </div>
         <Button 
          variant="ghost" 
          size="sm"
          className="rounded-lg text-xs font-bold h-8 group-hover:bg-primary/10 group-hover:text-primary transition-all"
        >
          View Details
          <ChevronRight className="ml-1 h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}

function TaskItem({ task, selected, onClick }) {
  const { bg, text, border } = getStatusColor(task.status);
  
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
      className={`w-full flex items-center gap-3 p-3 rounded-xl border transition-all duration-200 ${
        selected
          ? 'border-primary bg-primary/5 shadow-sm scale-[1.02] z-10'
          : 'border-border bg-card hover:border-primary/30 hover:bg-accent/30'
      }`}
    >
      <div className={`p-2 rounded-lg ${bg} border ${border} shadow-sm`}>
        <StatusIcon className={`w-4 h-4 ${text} ${isRunning ? 'animate-spin' : ''}`} />
      </div>
      <div className="flex-1 min-w-0 text-left">
        <p className="text-sm font-bold text-foreground truncate">{task.name || task.id}</p>
        <p className="text-[10px] text-muted-foreground font-mono mt-0.5 opacity-60">{task.id}</p>
      </div>
      <div className={`px-2 py-0.5 rounded text-[9px] uppercase font-black tracking-widest ${bg} ${text}`}>
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

  // Issues store hooks
  const {
    setWorkflow: setIssuesWorkflow,
    loadIssues,
    startGeneration,
    updateGenerationProgress,
    cancelGeneration,
  } = useIssuesStore();

  const handleIssuesModeSelect = useCallback(async (mode) => {
    setShowIssuesModal(false);
    setIssuesWorkflow(workflow.id, workflow.title || workflow.id);

    if (mode === 'fast') {
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
      startGeneration('ai', 10);
      navigate(`/workflows/${workflow.id}/issues`);

      try {
        const response = await workflowApi.previewIssues(workflow.id, false);
        const issues = response.preview_issues || [];

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

  const [activityExpanded, setActivityExpanded] = useState(true);
  const [attachmentsExpanded, setAttachmentsExpanded] = useState(false);
  
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
  }, [taskPlanById, workflow.id]);

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

  const [activeMobileTab, setActiveMobileTab] = useState('tasks');
  const [taskView, setTaskView] = useState('list');

  const selectedTaskId = useMemo(() => {
    if (!selectedDoc?.key) return null;
    const match = selectedDoc.key.match(/^task-(?:plan|output):(.+)$/);
    return match ? match[1] : null;
  }, [selectedDoc]);

  const filteredActivity = useMemo(() => {
    if (!selectedTaskId) return agentActivity;
    return agentActivity.filter(entry => {
      if (entry.data?.task_id === selectedTaskId) return true;
      if (entry.message && entry.message.includes(selectedTaskId)) return true;
      const taskName = tasks.find(t => t.id === selectedTaskId)?.name;
      if (taskName && entry.message && entry.message.includes(taskName)) return true;
      return false;
    });
  }, [agentActivity, selectedTaskId, tasks]);

  return (
    <div className="relative min-h-full space-y-6 animate-fade-in pb-10">
      {/* Background Pattern - Detail View */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Detail Header - Glass Header style */}
      <div className="md:sticky md:top-14 z-20 -mx-3 sm:-mx-6 px-3 sm:px-6 py-4 border-b border-border bg-background/80 backdrop-blur-md shadow-sm mb-6">
        <div className="flex flex-col md:flex-row md:items-center gap-4">
          <div className="flex items-center gap-4 w-full md:w-auto">
            <button
              onClick={onBack}
              className="p-2 rounded-xl hover:bg-accent transition-colors shrink-0 border border-transparent hover:border-border"
            >
              <ArrowLeft className="w-5 h-5 text-muted-foreground" />
            </button>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 group">
                <h1 className="text-xl font-bold text-foreground line-clamp-1 tracking-tight">{displayTitle}</h1>
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
              <div className="flex items-center gap-3 mt-0.5">
                <p className="text-[10px] text-muted-foreground font-mono bg-muted/50 px-1.5 py-0.5 rounded border border-border/30">{workflow.id}</p>
                <ExecutionModeBadge config={workflow.config} variant="inline" />
              </div>
            </div>
          </div>

          <div className="hidden md:flex flex-1 justify-center">
            <PhaseStepper workflow={workflow} compact />
          </div>

          <div className="flex items-center gap-2 flex-wrap md:justify-end w-full md:w-auto">
            {workflow.status === 'running' && (
              <AgentActivityCompact activeAgents={activeAgents} />
            )}

            {workflow.status === 'pending' && (
              <>
                <Button
                  onClick={() => startWorkflow(workflow.id)}
                  disabled={loading}
                  size="sm"
                  className="flex-1 md:flex-none font-bold"
                >
                  {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <FastForward className="w-4 h-4 mr-1.5" />}
                  Run All
                </Button>
                <Button
                  onClick={() => analyzeWorkflow(workflow.id)}
                  disabled={loading}
                  variant="outline"
                  size="sm"
                  className="flex-1 md:flex-none font-bold bg-info/5 border-info/20 text-info hover:bg-info/10"
                >
                  {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4 mr-1.5" />}
                  Analyze
                </Button>
              </>
            )}

            {workflow.status === 'completed' && workflow.current_phase === 'plan' && (
              <Button
                onClick={() => planWorkflow(workflow.id)}
                disabled={loading}
                size="sm"
                className="flex-1 md:flex-none font-bold"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4 mr-1.5" />}
                Plan
              </Button>
            )}

            {workflow.status === 'completed' && (workflow.current_phase === 'plan' || workflow.current_phase === 'execute') && (
              <Button
                onClick={() => setReplanModalOpen(true)}
                disabled={loading}
                variant="outline"
                size="sm"
                className="flex-1 md:flex-none font-bold bg-warning/5 border-warning/20 text-warning hover:bg-warning/10"
              >
                <RefreshCw className="w-4 h-4 mr-1.5" />
                Replan
              </Button>
            )}

            {workflow.status === 'completed' && workflow.current_phase === 'execute' && (
              <Button
                onClick={() => executeWorkflow(workflow.id)}
                disabled={loading}
                size="sm"
                className="flex-1 md:flex-none font-bold"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4 mr-1.5" />}
                Execute
              </Button>
            )}

            {['execute', 'done'].includes(workflow.current_phase) && (
              <Button
                onClick={() => setShowIssuesModal(true)}
                disabled={issuesGenerating}
                variant="outline"
                size="sm"
                className="flex-1 md:flex-none font-bold bg-primary/5 border-primary/20 text-primary hover:bg-primary/10"
              >
                {issuesGenerating ? <Loader2 className="w-4 h-4 animate-spin" /> : <FileText className="w-4 h-4 mr-1.5" />}
                Create Issues
              </Button>
            )}

            {workflow.status === 'paused' && (
              <Button
                onClick={() => resumeWorkflow(workflow.id)}
                disabled={loading}
                size="sm"
                className="flex-1 md:flex-none font-bold"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4 mr-1.5" />}
                Resume
              </Button>
            )}

            {workflow.status === 'failed' && (
              <Button
                onClick={() => startWorkflow(workflow.id)}
                disabled={loading}
                size="sm"
                className="flex-1 md:flex-none font-bold"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RotateCcw className="w-4 h-4 mr-1.5" />}
                Retry
              </Button>
            )}

            {workflow.status === 'running' && (
              <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-xl bg-info/10 text-info text-xs font-bold uppercase tracking-widest border border-info/20 shadow-sm">
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                {workflow.current_phase || 'RUNNING'}
              </div>
            )}

            {workflow.status === 'running' && (
              <>
                <Button
                  onClick={() => pauseWorkflow(workflow.id)}
                  variant="outline"
                  size="sm"
                  className="flex-1 md:flex-none font-bold h-9"
                >
                  <Pause className="w-4 h-4 mr-1.5" />
                  Pause
                </Button>
                <Button
                  onClick={() => stopWorkflow(workflow.id)}
                  variant="outline"
                  size="sm"
                  className="flex-1 md:flex-none font-bold h-9 bg-destructive/5 border-destructive/20 text-destructive hover:bg-destructive/10"
                >
                  <StopCircle className="w-4 h-4 mr-1.5" />
                  Stop
                </Button>
              </>
            )}

            {workflow.status === 'paused' && (
              <Button
                onClick={() => stopWorkflow(workflow.id)}
                variant="outline"
                size="sm"
                className="flex-1 md:flex-none font-bold h-9 bg-destructive/5 border-destructive/20 text-destructive hover:bg-destructive/10"
              >
                <StopCircle className="w-4 h-4 mr-1.5" />
                Stop
              </Button>
            )}
  
            {workflow.status !== 'pending' && (
              <Button
                onClick={handleDownloadArtifacts}
                variant="outline"
                size="sm"
                className="flex-1 md:flex-none font-bold h-9"
                title="Download artifacts"
              >
                <Download className="w-4 h-4 mr-1.5" />
                Bundle
              </Button>
            )}
  
            {canDelete && (
              <Button
                onClick={() => setDeleteDialogOpen(true)}
                variant="outline"
                size="sm"
                className="flex-1 md:flex-none font-bold h-9 border-destructive/20 text-destructive/60 hover:text-destructive hover:bg-destructive/10"
                title="Delete workflow"
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Workflow Error Banner */}
      {workflow.status === 'failed' && workflow.error && (
        <div className="p-4 rounded-2xl bg-destructive/5 border border-destructive/20 animate-fade-in shadow-sm">
          <div className="flex items-start gap-3">
            <XCircle className="w-5 h-5 text-destructive flex-shrink-0 mt-0.5" />
            <div className="flex-1 min-w-0">
              <p className="text-xs font-black uppercase tracking-widest text-destructive mb-1">Execution Failure</p>
              <p className="text-sm text-destructive/80 font-medium break-words leading-relaxed">{workflow.error}</p>
            </div>
          </div>
        </div>
      )}

      {/* Info Card */}
      <Card className="bg-card/40 backdrop-blur-sm border-border overflow-hidden">
        <CardHeader className="p-5">
          <div className="flex items-start justify-between gap-6">
            <div className="flex-1 min-w-0 space-y-3">
              <div className="flex flex-wrap items-center gap-3">
                <StatusBadge status={workflow.status} />
                <ExecutionModeBadge config={workflow.config} variant="badge" />
              </div>
              <p className="text-sm text-muted-foreground leading-relaxed italic border-l-2 border-border/50 pl-4 py-1">
                &ldquo;{workflow.prompt || 'No prompt content'}&rdquo;
              </p>
            </div>
            {canEdit && (
              <Button
                onClick={() => setEditModalOpen(true)}
                variant="outline"
                size="sm"
                className="rounded-xl text-xs font-bold shrink-0 shadow-sm"
              >
                <Pencil className="w-3.5 h-3.5 mr-1.5" />
                Edit
              </Button>
            )}
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-6 mt-6 pt-5 border-t border-border/50">
            <div className="space-y-1">
              <p className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground/60">Phase</p>
              <p className="text-sm font-bold text-foreground flex items-center gap-2">
                 <Layers className="w-3.5 h-3.5 text-primary" />
                 {workflow.current_phase || 'N/A'}
              </p>
            </div>
            <div className="space-y-1">
              <p className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground/60">Tasks</p>
              <p className="text-sm font-bold text-foreground flex items-center gap-2">
                 <LayoutList className="w-3.5 h-3.5 text-primary" />
                 {tasks.length} Assigned
              </p>
            </div>
            <div className="space-y-1">
              <p className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground/60">Created</p>
              <p className="text-sm font-bold text-foreground">
                {workflow.created_at ? new Date(workflow.created_at).toLocaleDateString() : '—'}
              </p>
            </div>
            <div className="space-y-1">
              <p className="text-[10px] font-black uppercase tracking-[0.2em] text-muted-foreground/60">Updated</p>
              <p className="text-sm font-bold text-foreground">
                {workflow.updated_at ? new Date(workflow.updated_at).toLocaleDateString() : '—'}
              </p>
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Mobile Tab Bar */}
      <div className="md:hidden flex border-b border-border bg-card/20 backdrop-blur-sm rounded-xl mb-4 overflow-hidden">
        {[
          { id: 'tasks', label: 'Tasks', icon: List },
          { id: 'preview', label: 'Preview', icon: FileText },
          { id: 'activity', label: 'Activity', icon: Activity },
        ].map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveMobileTab(tab.id)}
            className={`flex-1 py-3 text-[10px] font-black uppercase tracking-widest text-center transition-all ${
              activeMobileTab === tab.id
                ? 'bg-primary/10 text-primary border-b-2 border-primary'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            <div className="flex flex-col items-center gap-1">
              <tab.icon className="w-4 h-4" />
              {tab.label}
            </div>
          </button>
        ))}
      </div>

      {/* Agent Activity Panel */}
      <div className={`${activeMobileTab === 'activity' ? 'block' : 'hidden'} md:block animate-fade-up`}>
        {(agentActivity.length > 0 || activeAgents.length > 0) && (
          <>
            {selectedTaskId && filteredActivity.length !== agentActivity.length && (
              <div className="mb-3 px-4 py-2 bg-accent/50 rounded-xl border border-border/50 flex items-center justify-between text-[10px] font-bold uppercase tracking-widest text-muted-foreground shadow-sm">
                <span className="flex items-center gap-2">
                  <Filter className="w-3.5 h-3.5 text-primary" />
                  Logs for <span className="font-mono text-foreground">{selectedTaskId}</span> ({filteredActivity.length}/{agentActivity.length} events)
                </span>
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

      {/* Inspector Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className={`space-y-8 ${activeMobileTab === 'tasks' ? 'block' : 'hidden'} md:block`}>
          {/* Attachments */}
          <Card className="bg-card/40 backdrop-blur-sm border-border overflow-hidden group">
            <CardHeader className="p-5">
              <div className="flex items-center justify-between mb-4">
                <button
                  type="button"
                  onClick={() => setAttachmentsExpanded(!attachmentsExpanded)}
                  className="flex items-center gap-2.5 text-sm font-bold uppercase tracking-widest text-muted-foreground hover:text-primary transition-colors"
                >
                  <Badge variant="secondary" className="px-1.5 py-0 bg-primary/10 text-primary border-transparent">
                    {workflow.attachments?.length || 0}
                  </Badge>
                  Attachments
                  {attachmentsExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
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
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => attachmentInputRef.current?.click()}
                      disabled={attachmentUploading}
                      className="h-8 rounded-lg text-[10px] font-black uppercase tracking-widest"
                    >
                      {attachmentUploading ? <Loader2 className="w-3 h-3 animate-spin" /> : <Upload className="w-3 h-3 mr-1.5" />}
                      Add
                    </Button>
                  </div>
                )}
              </div>

              {attachmentsExpanded && (
                <div className="animate-fade-in">
                  {workflow.attachments && workflow.attachments.length > 0 ? (
                    <div className="space-y-2">
                      {workflow.attachments.map((a) => (
                        <div
                          key={a.id}
                          className="flex items-center justify-between gap-3 p-3 rounded-xl border border-border bg-background/50 hover:border-primary/30 transition-all group/item"
                        >
                          <div className="min-w-0 flex items-center gap-3">
                            <div className="p-2 rounded-lg bg-muted text-muted-foreground group-hover/item:text-primary group-hover/item:bg-primary/10 transition-colors">
                               <FileText className="w-4 h-4" />
                            </div>
                            <div className="min-w-0">
                              <p className="text-xs font-bold text-foreground truncate">{a.name}</p>
                              <p className="text-[10px] text-muted-foreground font-mono opacity-60">
                                {a.size >= 1024 ? `${Math.round(a.size / 1024)} KB` : `${a.size} B`}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center gap-1 shrink-0 opacity-0 group-hover/item:opacity-100 transition-opacity">
                            <button
                              type="button"
                              onClick={() => handleDownloadAttachment(a)}
                              className="p-1.5 rounded-lg hover:bg-accent transition-colors"
                              title="Download"
                            >
                              <Download className="w-3.5 h-3.5 text-muted-foreground hover:text-primary" />
                            </button>
                            {canModifyAttachments && (
                              <button
                                type="button"
                                onClick={() => handleDeleteAttachment(a)}
                                className="p-1.5 rounded-lg hover:bg-destructive/10 transition-colors"
                                title="Delete"
                              >
                                <Trash2 className="w-3.5 h-3.5 text-destructive" />
                              </button>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-center py-10 rounded-xl border border-dashed border-border/50 bg-muted/5">
                      <p className="text-xs font-bold uppercase tracking-widest text-muted-foreground/40">No context files</p>
                    </div>
                  )}
                </div>
              )}
            </CardHeader>
          </Card>

          {/* Tasks */}
          <Card className="bg-card/40 backdrop-blur-sm border-border overflow-hidden">
            <CardHeader className="p-5">
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2">
                  <h3 className="text-sm font-bold uppercase tracking-widest text-muted-foreground">Execution Queue</h3>
                  <Badge variant="secondary" className="px-1.5 py-0 font-bold bg-muted/50 text-muted-foreground border-transparent">
                    {tasks.length}
                  </Badge>
                </div>
                <div className="flex bg-muted/50 p-0.5 rounded-lg border border-border/50 shadow-inner">
                  <button
                    onClick={() => setTaskView('list')}
                    className={`p-1.5 rounded-md transition-all ${taskView === 'list' ? 'bg-background shadow-sm text-primary' : 'text-muted-foreground hover:text-foreground'}`}
                    title="List View"
                  >
                    <LayoutList className="w-3.5 h-3.5" />
                  </button>
                  <button
                    onClick={() => setTaskView('graph')}
                    className={`p-1.5 rounded-md transition-all ${taskView === 'graph' ? 'bg-background shadow-sm text-primary' : 'text-muted-foreground hover:text-foreground'}`}
                    title="Graph View"
                  >
                    <Network className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
              
              {taskView === 'list' ? (
                tasks.length > 0 ? (
                  <div className="space-y-3 max-h-[50vh] overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-muted">
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
                  <div className="text-center py-12 rounded-2xl border border-dashed border-border bg-muted/5">
                    <p className="text-xs font-bold uppercase tracking-widest text-muted-foreground/40 italic">Queue is empty</p>
                  </div>
                )
              ) : (
                <WorkflowGraph tasks={tasks} />
              )}
            </CardHeader>
          </Card>

          {/* Artifacts */}
          <Card className="bg-card/40 backdrop-blur-sm border-border overflow-hidden">
            <CardHeader className="p-5">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-bold uppercase tracking-widest text-muted-foreground">Generated Reports</h3>
                {artifactsLoading && <Loader2 className="w-4 h-4 text-primary animate-spin" />}
              </div>

              {artifactsError && (
                <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-xs text-destructive font-medium mb-4">
                  {artifactsError}
                </div>
              )}

              {!artifactIndex?.reportPath ? (
                <div className="text-center py-10 rounded-2xl border border-dashed border-border bg-muted/5">
                   <p className="text-xs font-bold uppercase tracking-widest text-muted-foreground/40 italic">No reports yet</p>
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="p-2 rounded-lg bg-muted/50 border border-border/50">
                    <p className="text-[9px] font-black uppercase tracking-widest text-muted-foreground/60 flex items-center gap-1.5 overflow-hidden">
                      <FolderTree className="w-3 h-3 text-primary shrink-0" />
                      <span className="truncate">{artifactIndex.reportPath}</span>
                    </p>
                  </div>
                  <div className="space-y-4 max-h-[50vh] overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-muted">
                    <FileTree
                      items={docGroups.flatMap(g => g.docs.map(d => ({
                        ...d,
                        treePath: `${g.label}/${d.title}`
                      })))}
                      onSelect={setSelectedDoc}
                      selectedKey={selectedDoc?.key}
                    />
                  </div>
                </div>
              )}
            </CardHeader>
          </Card>
        </div>

        {/* Preview Panel */}
        <div className={`lg:col-span-2 p-0 rounded-2xl border border-border bg-card/60 backdrop-blur-sm flex flex-col max-h-[85vh] shadow-xl overflow-hidden group/preview ${activeMobileTab === 'preview' ? 'block' : 'hidden'} md:flex`}>
          <div className="flex items-center justify-between gap-4 p-5 border-b border-border bg-muted/20 flex-none">
            <div className="min-w-0 flex items-center gap-3">
              <div className="p-2 rounded-xl bg-background border border-border shadow-sm text-primary">
                 <Terminal className="w-4 h-4" />
              </div>
              <div className="min-w-0">
                <h3 className="text-base font-bold text-foreground truncate tracking-tight">
                  {selectedDoc?.title || 'System Output'}
                </h3>
                {selectedDoc?.path && (
                  <p className="text-[10px] text-muted-foreground font-mono truncate opacity-60">{selectedDoc.path}</p>
                )}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="icon"
                onClick={handleCopy}
                disabled={!docContent}
                className={`h-9 w-9 rounded-lg transition-all ${copied ? 'text-green-500 bg-green-500/10' : 'text-muted-foreground hover:bg-accent'}`}
                title="Copy to clipboard"
              >
                {copied ? <CheckCircle2 className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
              </Button>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleRefresh}
                disabled={!selectedDoc}
                className="h-9 w-9 rounded-lg text-muted-foreground hover:bg-accent"
                title="Reload content"
              >
                <RefreshCw className="w-4 h-4" />
              </Button>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto min-h-0 p-6 scrollbar-thin scrollbar-thumb-muted">
            {docLoading ? (
              <div className="space-y-6 animate-pulse">
                <div className="h-8 w-1/3 bg-muted rounded-lg" />
                <div className="space-y-3">
                   <div className="h-4 w-full bg-muted/50 rounded" />
                   <div className="h-4 w-5/6 bg-muted/50 rounded" />
                   <div className="h-4 w-full bg-muted/50 rounded" />
                   <div className="h-4 w-4/6 bg-muted/50 rounded" />
                </div>
                <div className="h-48 w-full bg-muted/20 rounded-xl border border-border/50" />
              </div>
            ) : docError ? (
              <div className="flex flex-col items-center justify-center py-20 text-center gap-4">
                 <div className="p-4 rounded-full bg-destructive/10 text-destructive border border-destructive/20">
                    <XCircle className="w-8 h-8" />
                 </div>
                 <div>
                    <p className="font-bold text-foreground">Content Unavailable</p>
                    <p className="text-sm text-muted-foreground max-w-xs mx-auto mt-1">{docError}</p>
                 </div>
                 <Button onClick={handleRefresh} variant="outline" size="sm" className="rounded-xl">Try again</Button>
              </div>
            ) : selectedDoc ? (
              <div className="animate-fade-in h-full">
                {selectedDoc.path && !selectedDoc.path.endsWith('.md') ? (
                  <CodeEditor 
                    value={docContent} 
                    language={selectedDoc.path.split('.').pop()} 
                    readOnly={true} 
                  />
                ) : (
                  <MarkdownViewer markdown={docContent} />
                )}
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center h-full text-center gap-4 py-20">
                <div className="p-6 rounded-full bg-muted/30 border border-border/50 text-muted-foreground/30">
                   <Search className="w-12 h-12" />
                </div>
                <div className="space-y-1">
                   <p className="font-bold text-muted-foreground">No Selection</p>
                   <p className="text-sm text-muted-foreground/60">Select a task or report artifact to preview here.</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      <EditWorkflowModal
        isOpen={editModalOpen}
        onClose={() => setEditModalOpen(false)}
        workflow={workflow}
        onSave={handleSaveWorkflow}
        canEditPrompt={canEditPrompt}
      />

      <ConfirmDialog
        isOpen={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        onConfirm={handleDelete}
        title="Delete Workflow?"
        message={`This will permanently delete "${displayTitle}" and all its associated data. This action cannot be undone.`}
        confirmText="Delete"
        variant="danger"
      />

      <ReplanModal
        isOpen={isReplanModalOpen}
        onClose={() => setReplanModalOpen(false)}
        onSubmit={async (context) => {
          await replanWorkflow(workflow.id, context);
          setReplanModalOpen(false);
        }}
        loading={loading}
      />

      <GenerationOptionsModal
        isOpen={showIssuesModal}
        onClose={() => setShowIssuesModal(false)}
        onSelect={handleIssuesModeSelect}
        loading={issuesGenerating}
      />
    </div>
  );
}

const AGENT_OPTIONS = [
  { value: 'claude', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'codex', label: 'Codex' },
];

function NewWorkflowForm({ onSubmit, onCancel, loading, initialData }) {
  const [title, setTitle] = useState(initialData?.name || '');
  const [prompt, setPrompt] = useState(initialData?.prompt || '');
  const [files, setFiles] = useState([]);
  const fileInputRef = useRef(null);

  const initialExecutionMode = initialData?.executionStrategy === 'single-agent' ? 'single_agent' : 'multi_agent';
  const [executionMode, setExecutionMode] = useState(initialExecutionMode);
  const [singleAgentName, setSingleAgentName] = useState('claude');
  const [singleAgentModel, setSingleAgentModel] = useState('');
  const [singleAgentReasoningEffort, setSingleAgentReasoningEffort] = useState('');

  useEnums();

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
    <div className="relative w-full animate-fade-in pb-10">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      
      <div className="max-w-2xl mx-auto p-8 rounded-3xl border border-border bg-card/60 backdrop-blur-md shadow-2xl animate-fade-up">
        <div className="flex items-center gap-3 mb-8">
           <div className="p-3 rounded-2xl bg-primary/10 border border-primary/20 text-primary">
              <Plus className="w-6 h-6" />
           </div>
           <div>
              <h2 className="text-2xl font-black text-foreground tracking-tight">Create Workflow</h2>
              <p className="text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground/60">BLUEPRINT DEFINITION</p>
           </div>
        </div>

      <form onSubmit={handleSubmit} className="space-y-8">
        <div className="space-y-6">
          <div className="space-y-2">
            <label className="text-[10px] font-black uppercase tracking-widest text-muted-foreground ml-1">
              Workflow Title <span className="opacity-40">(optional)</span>
            </label>
            <Input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Name your execution cycle..."
              className="h-11 bg-background/50 rounded-xl"
            />
          </div>
          <div className="space-y-2">
            <label className="text-[10px] font-black uppercase tracking-widest text-muted-foreground ml-1">
              System Instruction / Prompt
            </label>
            <div className="relative group">
              <div className="absolute inset-0 bg-primary/5 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder="Describe the desired outcome in natural language..."
                rows={6}
                spellCheck={false}
                className="w-full px-4 py-3 pr-12 border border-input rounded-xl bg-background/50 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 transition-all resize-none font-mono text-sm leading-relaxed"
              />
              <VoiceInputButton
                onTranscript={(text) => setPrompt((prev) => (prev ? prev + ' ' + text : text))}
                disabled={loading}
                className="absolute top-3 right-3"
              />
            </div>
          </div>
        </div>

        <div className="space-y-4 pt-6 border-t border-border/50">
           <h3 className="text-[10px] font-black uppercase tracking-widest text-muted-foreground ml-1">Execution Strategy</h3>
           
           <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <button
              type="button"
              onClick={() => setExecutionMode('multi_agent')}
              className={`relative flex flex-col items-start p-5 rounded-2xl border-2 transition-all text-left group ${
                executionMode === 'multi_agent'
                  ? 'border-primary bg-primary/5 shadow-inner'
                  : 'border-border bg-background/30 hover:border-primary/30 hover:bg-accent/50'
              }`}
            >
              <div className="flex items-center justify-between w-full mb-3">
                <div className={`p-2 rounded-xl border transition-colors ${executionMode === 'multi_agent' ? 'bg-primary/10 border-primary/20 text-primary' : 'bg-muted border-border text-muted-foreground'}`}>
                   <Network className="w-5 h-5" />
                </div>
                {executionMode === 'multi_agent' && <CheckCircle2 className="w-4 h-4 text-primary" />}
              </div>
              <span className="font-bold text-foreground text-sm">Multi-Agent Consensus</span>
              <span className="text-[10px] text-muted-foreground mt-1 leading-relaxed">Iterative refinement and debate between multiple models. High accuracy.</span>
            </button>

            <button
              type="button"
              onClick={() => setExecutionMode('single_agent')}
              className={`relative flex flex-col items-start p-5 rounded-2xl border-2 transition-all text-left group ${
                executionMode === 'single_agent'
                  ? 'border-primary bg-primary/5 shadow-inner'
                  : 'border-border bg-background/30 hover:border-primary/30 hover:bg-accent/50'
              }`}
            >
              <div className="flex items-center justify-between w-full mb-3">
                <div className={`p-2 rounded-xl border transition-colors ${executionMode === 'single_agent' ? 'bg-primary/10 border-primary/20 text-primary' : 'bg-muted border-border text-muted-foreground'}`}>
                   <Zap className="w-5 h-5" />
                </div>
                {executionMode === 'single_agent' && <CheckCircle2 className="w-4 h-4 text-primary" />}
              </div>
              <span className="font-bold text-foreground text-sm">Direct Agent</span>
              <span className="text-[10px] text-muted-foreground mt-1 leading-relaxed">Fast, streamlined execution by a single specialized provider. Low latency.</span>
            </button>
           </div>

          <div className={`overflow-hidden transition-all duration-500 ease-in-out ${executionMode === 'single_agent' ? 'max-h-[500px] opacity-100' : 'max-h-0 opacity-0 pointer-events-none'}`}>
            <div className="mt-4 p-5 rounded-2xl border border-border/50 bg-muted/30 space-y-5 animate-fade-in shadow-inner">
              <div className="flex items-center gap-2">
                <Settings className="w-3.5 h-3.5 text-primary" />
                <h4 className="text-[10px] font-black uppercase tracking-[0.2em] text-foreground">Advanced Provider Config</h4>
              </div>
              
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
                <div className="space-y-1.5">
                  <label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground ml-1">Provider</label>
                  <select
                    value={effectiveSingleAgentName}
                    onChange={(e) => {
                      setSingleAgentName(e.target.value);
                      setSingleAgentModel('');
                      setSingleAgentReasoningEffort('');
                    }}
                    className="w-full h-10 px-3 rounded-xl border border-input bg-background text-sm font-bold focus:ring-2 focus:ring-primary/20 outline-none transition-all"
                  >
                    {enabledAgents.map(agent => (
                      <option key={agent.value} value={agent.value}>{agent.label}</option>
                    ))}
                  </select>
                </div>

                <div className="space-y-1.5">
                  <label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground ml-1">Intelligence Model</label>
                  <select
                    value={effectiveSingleAgentModel}
                    onChange={(e) => setSingleAgentModel(e.target.value)}
                    className="w-full h-10 px-3 rounded-xl border border-input bg-background text-sm font-bold focus:ring-2 focus:ring-primary/20 outline-none transition-all"
                  >
                    {modelOptions.map((model) => (
                      <option key={`${effectiveSingleAgentName}-${model.value || 'default'}`} value={model.value}>{model.label}</option>
                    ))}
                  </select>
                </div>

                {agentSupportsReasoning && (
                  <div className="sm:col-span-2 space-y-1.5">
                    <label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground ml-1">Reasoning Intensity</label>
                    <select
                      value={effectiveSingleAgentReasoningEffort}
                      onChange={(e) => setSingleAgentReasoningEffort(e.target.value)}
                      className="w-full h-10 px-3 rounded-xl border border-input bg-background text-sm font-bold focus:ring-2 focus:ring-primary/20 outline-none transition-all"
                    >
                      <option value="">Balance (Default)</option>
                      {reasoningLevels.map((level) => (
                        <option key={level.value} value={level.value}>{level.label}</option>
                      ))}
                    </select>
                    {selectedReasoning?.description && (
                      <p className="text-[10px] text-muted-foreground leading-relaxed px-1">
                        {selectedReasoning.description}
                      </p>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="pt-6 border-t border-border/50">
          <label className="text-[10px] font-black uppercase tracking-widest text-muted-foreground ml-1 mb-2 block">Context Attachments</label>
          <div className="flex flex-wrap items-center gap-4">
            <input ref={fileInputRef} type="file" multiple onChange={handleFilesSelected} className="hidden" disabled={loading} />
            <Button
              type="button"
              variant="outline"
              onClick={() => fileInputRef.current?.click()}
              disabled={loading}
              className="rounded-xl border-dashed border-2 hover:border-primary/50 text-xs font-bold"
            >
              <Upload className="w-4 h-4 mr-2" />
              Upload Files
            </Button>
            <p className="text-[10px] text-muted-foreground italic">Supports codebase, text files and technical specs.</p>
          </div>
          {files.length > 0 && (
            <div className="mt-4 flex flex-wrap gap-2">
              {files.map((f, idx) => (
                <div key={idx} className="flex items-center gap-2 pl-3 pr-1 py-1 rounded-full bg-primary/10 text-primary text-[10px] font-bold border border-primary/20 shadow-sm animate-fade-in">
                  <span className="truncate max-w-[120px]">{f.name}</span>
                  <button type="button" onClick={() => removeFile(idx)} className="p-1 hover:bg-primary/20 rounded-full transition-colors"><X className="w-3 h-3" /></button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="flex flex-col sm:flex-row gap-3 pt-6">
          <Button
            type="submit"
            disabled={loading || !prompt.trim()}
            className="flex-1 rounded-xl h-12 text-sm font-black uppercase tracking-widest shadow-xl shadow-primary/20"
          >
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Zap className="w-5 h-5 mr-2" />}
            Initialize Workflow
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={onCancel}
            className="rounded-xl h-12 text-xs font-bold text-muted-foreground hover:text-foreground"
          >
            Abort
          </Button>
        </div>
      </form>
    </div>
  </div>
);
}

const STATUS_FILTERS = [
  { value: 'all', label: 'History', icon: List },
  { value: 'running', label: 'Processing', icon: Activity },
  { value: 'completed', label: 'Resolved', icon: CheckCircle2 },
  { value: 'failed', label: 'Halted', icon: XCircle },
];

function StatusFilterTabs({ status, setStatus }) {
  return (
    <div className="flex items-center gap-1.5 p-1.5 rounded-xl bg-muted/50 border border-border/50 shadow-inner">
      {STATUS_FILTERS.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          onClick={() => setStatus(value)}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-bold uppercase tracking-widest transition-all ${
            status === value
              ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/20 scale-105 z-10'
              : 'text-muted-foreground hover:bg-card hover:text-foreground'
          }`}
        >
          <Icon className="w-3.5 h-3.5" />
          {label}
        </button>
      ))}
    </div>
  );
}

function WorkflowFilters({ filter, setFilter }) {
  return (
    <div className="relative w-full sm:w-72">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
      <Input
        type="text"
        placeholder="Search logs and blueprints..."
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className="h-11 pl-10 pr-4 rounded-xl border border-border bg-background shadow-sm focus-visible:ring-primary/20"
      />
    </div>
  );
}

export default function Workflows() {
  const { id } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { workflows, loading, error, fetchWorkflows, fetchWorkflow, createWorkflow, deleteWorkflow, clearError } = useWorkflowStore();
  const { getTasksForWorkflow, setTasks } = useTaskStore();
  const notifyInfo = useUIStore((s) => s.notifyInfo);
  const notifyError = useUIStore((s) => s.notifyError);
  const [showNewForm, setShowNewForm] = useState(false);
  const [filter, setFilter] = useState('');

  const templateData = location.state?.template;

  const statusFilter = searchParams.get('status') || 'all';
  const setStatusFilter = useCallback((status) => {
    if (status === 'all') {
      searchParams.delete('status');
    } else {
      searchParams.set('status', status);
    }
    setSearchParams(searchParams);
  }, [searchParams, setSearchParams]);

  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [workflowToDelete, setWorkflowToDelete] = useState(null);

  useEffect(() => {
    fetchWorkflows();
  }, [fetchWorkflows]);

  useEffect(() => {
    if (id && id !== 'new') {
      fetchWorkflow(id);
    } else if (id === 'new' || templateData) {
      setShowNewForm(true);
    }
  }, [fetchWorkflow, id, templateData]);

  const filteredWorkflows = useMemo(() => {
    let result = workflows;
    if (statusFilter && statusFilter !== 'all') {
      result = result.filter(w => w.status === statusFilter);
    }
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
      const storeError = useWorkflowStore.getState().error;
      notifyError(storeError || 'Failed to create workflow');
      clearError();
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

  if (id === 'new' || showNewForm) {
    return (
      <NewWorkflowForm
        onSubmit={handleCreate}
        onCancel={() => {
          setShowNewForm(false);
          if (id === 'new') navigate('/workflows');
          navigate(location.pathname, { replace: true, state: {} });
        }}
        loading={loading}
        initialData={templateData}
      />
    );
  }

  if (selectedWorkflow) {
    return (
      <WorkflowDetail
        workflow={selectedWorkflow}
        tasks={workflowTasks}
        onBack={() => navigate('/workflows')}
      />
    );
  }

  return (
    <div className="relative min-h-full space-y-8 animate-fade-in pb-12">
      {/* Background Pattern - Consistent Style */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header */}
      <div className="relative overflow-hidden rounded-3xl border border-border bg-card/30 backdrop-blur-md p-8 sm:p-12 shadow-sm">
        <div className="absolute top-0 right-0 -translate-y-1/2 translate-x-1/3 w-96 h-96 bg-primary/5 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute bottom-0 left-0 translate-y-1/2 -translate-x-1/3 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none" />
        
        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-8">
          <div className="max-w-2xl space-y-4">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest">
              <Network className="h-3 w-3" />
              Automated Cycles
            </div>
            <h1 className="text-4xl sm:text-5xl font-black text-foreground tracking-tight leading-tight">
              Agent <span className="text-primary">Workflows</span>
            </h1>
            <p className="text-lg text-muted-foreground leading-relaxed max-w-xl">
              Monitor and manage your multi-agent execution threads. From initial analysis to final issue generation and verification.
            </p>
          </div>

          <div className="flex shrink-0">
             <Link
                to="/workflows/new"
                className="flex items-center justify-center gap-2 px-6 py-3 rounded-2xl bg-primary text-primary-foreground text-sm font-bold hover:bg-primary/90 transition-all shadow-xl shadow-primary/20"
              >
                <Plus className="w-5 h-5" />
                New Blueprint
              </Link>
          </div>
        </div>
      </div>

      {/* Control Bar - Sticky */}
      <div className="sticky top-14 z-30 flex flex-col gap-4 bg-background/80 backdrop-blur-md py-4 border-b border-border/50">
        <div className="flex flex-col md:flex-row gap-4 md:items-center justify-between">
          <WorkflowFilters filter={filter} setFilter={setFilter} />
          <div className="hidden sm:flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-muted-foreground bg-muted/50 px-3 py-2 rounded-lg border border-border/50">
            <Info className="h-3.5 w-3.5" />
            <span>{filteredWorkflows.length} Workflows Found</span>
          </div>
        </div>

        <div className="flex items-center gap-2 overflow-x-auto no-scrollbar mask-fade-right">
          <StatusFilterTabs status={statusFilter} setStatus={setStatusFilter} />
        </div>
      </div>

      {/* Grid Content */}
      {loading && workflows.length === 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="h-48 rounded-2xl bg-muted/20 animate-pulse border border-border/50" />
          ))}
        </div>
      ) : filteredWorkflows.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
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
        <div className="flex flex-col items-center justify-center py-24 text-center space-y-6 rounded-3xl border border-dashed border-border bg-muted/5 animate-fade-in">
          <div className="p-6 rounded-full bg-muted/30 border border-border text-muted-foreground/30">
            <GitBranch className="w-12 h-12" />
          </div>
          <div className="space-y-2">
            <h3 className="text-xl font-bold text-foreground">No matching workflows</h3>
            <p className="text-muted-foreground max-w-sm mx-auto leading-relaxed">
              {filter ? 'Refine your search criteria or clear the filters to see all historical executions.' : 'Your history is empty. Start your first agent workflow to begin orchestrating intelligence.'}
            </p>
          </div>
          {!filter ? (
            <Link
              to="/workflows/new"
              className="inline-flex items-center gap-2 px-8 py-3 rounded-2xl bg-primary text-primary-foreground text-sm font-black uppercase tracking-widest hover:bg-primary/90 transition-all shadow-xl shadow-primary/20"
            >
              <Plus className="w-5 h-5" />
              Create First Workflow
            </Link>
          ) : (
            <Button variant="outline" onClick={() => { setFilter(''); setStatusFilter('all'); }} className="rounded-xl font-bold px-6">
              Reset Filters
            </Button>
          )}
        </div>
      )}
      
      {/* Mobile FAB */}
      <FAB onClick={() => navigate('/workflows/new')} icon={Plus} label="New Workflow" />

      {/* Delete Confirmation */}
      <ConfirmDialog
        isOpen={deleteDialogOpen}
        onClose={handleDeleteCancel}
        onConfirm={handleDeleteConfirm}
        title="Delete Workflow Execution?"
        message={workflowToDelete ? `This will permanently purge "${deriveWorkflowTitle(workflowToDelete)}" and its entire execution history. Artifacts stored in the .quorum directory will remain but won't be accessible from the UI.` : ''}
        confirmText="Confirm Purge"
        variant="danger"
      />
    </div>
  );
}