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
  const lines = cleaned.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
  if (lines.length === 0) return '';
  const genericPrefixes = [/^analiza\b/i, /^analyze\b/i, /^implementa\b/i, /^implement\b/i, /^crea\b/i, /^create\b/i, /^eres\b/i, /^you are\b/i];
  const bestLine = lines.find((l) => !genericPrefixes.some((re) => re.test(l))) || lines[0];
  const title = normalizeWhitespace(bestLine);
  if (title.length <= 110) return title;
  const snippet = title.slice(0, 110);
  const lastSentence = Math.max(snippet.lastIndexOf('.'), snippet.lastIndexOf('!'), snippet.lastIndexOf('?'));
  return lastSentence > 50 ? snippet.slice(0, lastSentence + 1).trim() : snippet.trim();
}

function deriveWorkflowTitle(workflow, tasks = []) {
  if (workflow?.title && String(workflow.title).trim().length > 0) return String(workflow.title).trim();
  const namedTasks = (tasks || []).filter((t) => t?.name && String(t.name).trim().length > 0);
  if (namedTasks.length > 0) {
    const first = String(namedTasks[0].name).trim();
    const extra = Math.max(0, (tasks || []).length - 1);
    return extra > 0 ? `${first} +${extra}` : first;
  }
  const promptTitle = deriveTitleFromPrompt(workflow?.prompt);
  return promptTitle || workflow?.id || 'Untitled workflow';
}

function StatusBadge({ status }) {
  const { text } = getStatusColor(status);
  const iconMap = { pending: Clock, running: Activity, completed: CheckCircle2, failed: XCircle, paused: Pause };
  const StatusIcon = iconMap[status] || Clock;
  return (
    <span className={`inline-flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider ${text} opacity-80`}>
      <StatusIcon className="w-3 h-3" /> {status}
    </span>
  );
}

function WorkflowCard({ workflow, onClick, onDelete }) {
  const canDelete = workflow.status !== 'running';
  const statusColor = getStatusColor(workflow.status);

  return (
    <div
      onClick={onClick}
      className={`group flex flex-col h-full rounded-2xl border border-border/30 bg-card/10 backdrop-blur-sm transition-all duration-500 hover:shadow-soft hover:border-primary/20 hover:-translate-y-0.5 overflow-hidden`}
    >
      <div className="flex-1 p-6 space-y-4 cursor-pointer">
        <div className="flex items-start justify-between">
          <div className={`p-2 rounded-xl bg-primary/[0.03] border border-primary/5 transition-colors group-hover:border-primary/20`}>
            <GitBranch className="h-4 w-4 text-primary/60" />
          </div>
          <div className="flex items-center gap-3">
             <StatusBadge status={workflow.status} />
             {canDelete && (
                <button
                  onClick={(e) => { e.stopPropagation(); onDelete(workflow); }}
                  className="p-1 rounded-md opacity-0 group-hover:opacity-100 hover:bg-destructive/5 text-muted-foreground/30 hover:text-destructive transition-all"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              )}
          </div>
        </div>

        <div className="space-y-1">
          <h3 className="font-bold text-base text-foreground/90 line-clamp-2 leading-snug group-hover:text-primary transition-colors">
            {deriveWorkflowTitle(workflow)}
          </h3>
          <p className="text-[10px] text-muted-foreground/30 font-mono tracking-tight uppercase">
            {workflow.id.substring(0, 12)}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-4 pt-1">
           <div className="flex items-center gap-1.5">
              <Layers className="w-3 h-3 text-muted-foreground/30" />
              <span className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/50">
                {workflow.current_phase || 'INITIAL'}
              </span>
           </div>
           <div className="flex items-center gap-1.5">
              <LayoutList className="w-3 h-3 text-muted-foreground/30" />
              <span className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/50">
                {workflow.task_count || 0} Sub-tasks
              </span>
           </div>
        </div>
      </div>

      <div className="px-6 py-3 mt-auto flex items-center justify-between border-t border-border/20 bg-background/20">
         <ExecutionModeBadge config={workflow.config} variant="inline" />
         <ChevronRight className="w-3.5 h-3.5 text-muted-foreground/20 group-hover:text-primary transition-all" />
      </div>
    </div>
  );
}

function TaskItem({ task, selected, onClick }) {
  const { bg, text, border } = getStatusColor(task.status);
  const iconMap = { pending: Clock, running: Loader2, completed: CheckCircle2, failed: XCircle, paused: Pause };
  const StatusIcon = iconMap[task.status] || Clock;
  const isRunning = task.status === 'running';

  return (
    <button
      type="button"
      onClick={onClick}
      className={`w-full flex items-center gap-4 p-4 rounded-2xl border transition-all duration-300 ${
        selected
          ? 'border-primary/20 bg-primary/[0.02] shadow-sm'
          : 'border-border/30 bg-card/5 hover:border-primary/10 hover:bg-accent/20'
      }`}
    >
      <div className={`p-2 rounded-xl ${bg} border ${border} shadow-sm`}>
        <StatusIcon className={`w-3.5 h-3.5 ${text} ${isRunning ? 'animate-spin' : ''}`} />
      </div>
      <div className="flex-1 min-w-0 text-left">
        <p className="text-sm font-semibold text-foreground/80 truncate transition-colors">{task.name || task.id}</p>
        <p className="text-[9px] text-muted-foreground/30 font-mono mt-0.5">{task.id}</p>
      </div>
      <div className={`px-2 py-0.5 rounded-lg text-[9px] font-bold uppercase tracking-wider ${bg} ${text} opacity-80`}>
        {task.status}
      </div>
    </button>
  );
}

function WorkflowDetail({ workflow, tasks, onBack }) {
  const { startWorkflow, pauseWorkflow, resumeWorkflow, stopWorkflow, deleteWorkflow, updateWorkflow, fetchWorkflow, analyzeWorkflow, planWorkflow, replanWorkflow, executeWorkflow, loading, error, clearError } = useWorkflowStore();
  const notifyInfo = useUIStore((s) => s.notifyInfo);
  const notifyError = useUIStore((s) => s.notifyError);
  const navigate = useNavigate();
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [isReplanModalOpen, setReplanModalOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const canDelete = workflow.status !== 'running';
  const canModifyAttachments = workflow.status !== 'running';
  const attachmentInputRef = useRef(null);
  const [attachmentUploading, setAttachmentUploading] = useState(false);

  const handleUploadAttachments = useCallback(async (fileList) => {
    if (!fileList || fileList.length === 0 || !canModifyAttachments) return;
    setAttachmentUploading(true);
    try {
      await workflowApi.uploadAttachments(workflow.id, fileList);
      await fetchWorkflow(workflow.id);
      notifyInfo(`Uploaded ${fileList.length} attachment(s)`);
    } catch (err) { notifyError(err.message || 'Failed to upload attachments'); } finally { setAttachmentUploading(false); }
  }, [workflow.id, canModifyAttachments, fetchWorkflow, notifyInfo, notifyError]);

  const handleAttachmentSelect = (e) => { const selected = Array.from(e.target.files || []); e.target.value = ''; handleUploadAttachments(selected); };

  const handleDeleteAttachment = useCallback(async (attachment) => {
    if (!canModifyAttachments || !window.confirm(`Delete "${attachment.name}"?`)) return;
    try {
      await workflowApi.deleteAttachment(workflow.id, attachment.id);
      await fetchWorkflow(workflow.id);
      notifyInfo('Attachment deleted');
    } catch (err) { notifyError(err.message || 'Failed to delete attachment'); }
  }, [workflow.id, canModifyAttachments, fetchWorkflow, notifyInfo, notifyError]);

  const handleDownloadAttachment = (attachment) => { window.open(`/api/v1/workflows/${workflow.id}/attachments/${attachment.id}/download`, '_blank'); };
  const handleDownloadArtifacts = () => { window.open(`/api/v1/workflows/${workflow.id}/download`, '_blank'); };

  const { setWorkflow: setIssuesWorkflow, loadIssues, startGeneration, updateGenerationProgress, cancelGeneration } = useIssuesStore();

  const handleIssuesModeSelect = useCallback(async (mode) => {
    setShowIssuesModal(false);
    setIssuesWorkflow(workflow.id, workflow.title || workflow.id);
    if (mode === 'fast') {
      setIssuesGenerating(true);
      try {
        const response = await workflowApi.previewIssues(workflow.id, true);
        const issues = response.preview_issues || [];
        if (issues.length > 0) {
          loadIssues(issues, { ai_used: response.ai_used, ai_errors: response.ai_errors });
          navigate(`/workflows/${workflow.id}/issues`);
        } else { notifyError('No issues generated from workflow artifacts'); }
      } catch (err) { notifyError(err.message || 'Failed to generate issues'); } finally { setIssuesGenerating(false); }
    } else {
      startGeneration('ai', 10); navigate(`/workflows/${workflow.id}/issues`);
      try {
        const response = await workflowApi.previewIssues(workflow.id, false);
        const issues = response.preview_issues || [];
        for (let i = 0; i < issues.length; i++) { await new Promise(resolve => setTimeout(resolve, 200)); updateGenerationProgress(i + 1, issues[i]); }
        loadIssues(issues, { ai_used: response.ai_used, ai_errors: response.ai_errors });
      } catch (err) { cancelGeneration(); notifyError(err.message || 'Failed to generate issues'); navigate(`/workflows/${workflow.id}`); }
    }
  }, [workflow.id, workflow.title, setIssuesWorkflow, loadIssues, startGeneration, updateGenerationProgress, cancelGeneration, navigate, notifyError]);

  const handleDelete = useCallback(async () => {
    if (await deleteWorkflow(workflow.id)) { notifyInfo('Workflow deleted'); navigate('/workflows'); } else { notifyError('Failed to delete workflow'); }
  }, [workflow.id, deleteWorkflow, notifyInfo, notifyError, navigate]);

  const canEdit = workflow.status !== 'running';
  const canEditPrompt = workflow.status === 'pending';
  const displayTitle = workflow.title || deriveWorkflowTitle(workflow, tasks);

  const handleSaveWorkflow = useCallback(async (updates) => {
    try { await updateWorkflow(workflow.id, updates); notifyInfo('Workflow updated'); } catch (err) { notifyError(err.message || 'Failed to update workflow'); throw err; }
  }, [workflow.id, updateWorkflow, notifyInfo, notifyError]);

  const [activityExpanded, setActivityExpanded] = useState(true);
  const [attachmentsExpanded, setAttachmentsExpanded] = useState(false);
  const [showIssuesModal, setShowIssuesModal] = useState(false);
  const [issuesGenerating, setIssuesGenerating] = useState(false);
  const agentActivityMap = useAgentStore((s) => s.agentActivity);
  const currentAgentsMap = useAgentStore((s) => s.currentAgents);
  const agentActivity = useMemo(() => agentActivityMap[workflow?.id] || [], [agentActivityMap, workflow?.id]);
  const activeAgents = useMemo(() => {
    const agents = currentAgentsMap[workflow?.id] || {};
    return Object.entries(agents).filter(([, info]) => ['started', 'thinking', 'tool_use', 'progress'].includes(info.status)).map(([name, info]) => ({ name, ...info }));
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

  const inferReportPath = useCallback(async (id) => { try { const e = await fileApi.list('.quorum/runs'); return e.find((en) => en.is_dir && en.name === id)?.path || null; } catch { return null; } }, []);

  const buildArtifactIndex = useCallback(async () => {
    setArtifactsLoading(true); setArtifactsError(null);
    try {
      const reportPath = workflow.report_path || (await inferReportPath(workflow.id));
      if (!reportPath) { setArtifactIndex(null); return; }
      const safeList = async (p) => { try { return await fileApi.list(p); } catch { return null; } };
      const stripMd = (n) => (n || '').replace(/\.md$/i, '');
      const parseNumber = (s) => { const m = String(s || '').match(/\d+/); return m ? Number(m[0]) : Number.NaN; };
      const docs = { prompts: [], analyses: [], comparisons: [], plan: [] };
      const planTaskFiles = [];
      const analyzePath = `${reportPath}/analyze-phase`;
      const analyzeEntries = (await safeList(analyzePath)) || [];
      const analyzeFiles = new Set(analyzeEntries.filter((e) => !e.is_dir).map((e) => e.name));
      if (analyzeFiles.has('00-original-prompt.md')) docs.prompts.push({ key: 'prompt:original', title: 'Original System Instruction', path: `${analyzePath}/00-original-prompt.md` });
      else if (workflow.prompt) docs.prompts.push({ key: 'prompt:original:state', title: 'Original System Instruction', getContent: async () => `# Original Prompt\n\n\`\`\`\n${workflow.prompt}\n\`\`\`\n` });
      if (analyzeFiles.has('01-refined-prompt.md')) docs.prompts.push({ key: 'prompt:optimized', title: 'Optimized Logic', path: `${analyzePath}/01-refined-prompt.md` });
      if (analyzeFiles.has('consolidated.md')) docs.analyses.push({ key: 'analysis:consolidated', title: 'Consolidated Analysis', path: `${analyzePath}/consolidated.md` });
      const versionDirs = analyzeEntries.filter((e) => e.is_dir && /^v\d+$/i.test(e.name)).sort((a, b) => (parseNumber(a.name) || 0) - (parseNumber(b.name) || 0));
      for (const dir of versionDirs) {
        const vEntries = (await safeList(dir.path)) || [];
        vEntries.filter((e) => !e.is_dir && /\.md$/i.test(e.name)).forEach((f) => docs.analyses.push({ key: `analysis:${dir.name}:${f.name}`, title: `${dir.name.toUpperCase()} · ${stripMd(f.name)}`, path: f.path }));
      }
      const planPath = `${reportPath}/plan-phase`;
      const planEntries = (await safeList(planPath)) || [];
      const planFiles = new Set(planEntries.filter((e) => !e.is_dir).map((e) => e.name));
      if (planFiles.has('final-plan.md')) docs.plan.push({ key: 'plan:final', title: 'Final Execution Plan', path: `${planPath}/final-plan.md` });
      const tasksDir = planEntries.find((e) => e.is_dir && e.name === 'tasks');
      if (tasksDir) { const tEntries = (await safeList(tasksDir.path)) || []; planTaskFiles.push(...tEntries.filter((e) => !e.is_dir && /\.md$/i.test(e.name))); }
      setArtifactIndex({ reportPath, docs, planTaskFiles });
    } catch (err) { setArtifactIndex(null); setArtifactsError(err?.message || 'Failed to load artifacts'); } finally { setArtifactsLoading(false); }
  }, [inferReportPath, workflow]);

  useEffect(() => { buildArtifactIndex(); }, [buildArtifactIndex]);

  const taskPlanById = useMemo(() => {
    const map = {}; const files = artifactIndex?.planTaskFiles || []; if (files.length === 0 || tasks.length === 0) return map;
    for (const task of tasks) { const match = files.find((f) => f.name === `${task.id}.md` || f.name?.startsWith(`${task.id}-`)); if (match?.path) map[task.id] = match.path; }
    return map;
  }, [artifactIndex?.planTaskFiles, tasks]);

  const loadDoc = useCallback(async (doc, { force = false } = {}) => {
    if (!doc) return; const cacheKey = doc.path || doc.key;
    if (!force && cacheRef.current.has(cacheKey)) { setDocError(null); setDocContent(cacheRef.current.get(cacheKey)); return; }
    setDocLoading(true); setDocError(null);
    try {
      let markdown = '';
      if (doc.getContent) markdown = await doc.getContent();
      else if (doc.path) { const file = await fileApi.getContent(doc.path); if (file.binary) throw new Error('File is binary'); markdown = file.content || ''; }
      cacheRef.current.set(cacheKey, markdown); setDocContent(markdown);
    } catch (err) { setDocContent(''); setDocError(err?.message || 'Failed to load document'); } finally { setDocLoading(false); }
  }, []);

  useEffect(() => { if (!selectedDoc && artifactIndex) { const first = artifactIndex.docs.prompts[0] || artifactIndex.docs.analyses[0] || artifactIndex.docs.plan[0]; if (first) setSelectedDoc(first); } }, [artifactIndex, selectedDoc]);
  useEffect(() => { loadDoc(selectedDoc); }, [loadDoc, selectedDoc]);

  const selectTask = useCallback((task) => {
    setActivityExpanded(false); const planPath = taskPlanById[task.id];
    if (planPath) { setSelectedDoc({ key: `task-plan:${task.id}`, title: `${task.name || task.id} · Plan`, path: planPath }); return; }
    setSelectedDoc({
      key: `task-output:${task.id}`, title: `${task.name || task.id} · Output`,
      getContent: async () => {
        const tData = await workflowApi.getTask(workflow.id, task.id); let out = tData.output || '';
        if (tData.output_file) { try { const f = await fileApi.getContent(tData.output_file); if (!f.binary && f.content) out = f.content; } catch {} }
        return out ? `# Output\n\n\`\`\`\n${out}\n\`\`\`\n` : '_No output captured for this task._\n';
      },
    });
  }, [taskPlanById, workflow.id]);

  const handleCopy = useCallback(async () => { if (!docContent) return; try { await navigator.clipboard.writeText(docContent); setCopied(true); notifyInfo('Copied to clipboard'); setTimeout(() => setCopied(false), 2000); } catch { notifyError('Failed to copy'); } }, [docContent, notifyError, notifyInfo]);
  const handleRefresh = useCallback(() => { loadDoc(selectedDoc, { force: true }); }, [loadDoc, selectedDoc]);

  const docGroups = useMemo(() => ([ { id: 'prompts', label: 'Blueprints', docs: artifactIndex?.docs.prompts || [] }, { id: 'analyses', label: 'Intelligence', docs: artifactIndex?.docs.analyses || [] }, { id: 'plan', label: 'Operations', docs: artifactIndex?.docs.plan || [] } ]), [artifactIndex]);
  const [activeMobileTab, setActiveMobileTab] = useState('tasks');
  const [taskView, setTaskView] = useState('list');
  const selectedTaskId = useMemo(() => { if (!selectedDoc?.key) return null; const match = selectedDoc.key.match(/^task-(?:plan|output):(.+)$/); return match ? match[1] : null; }, [selectedDoc]);
  const filteredActivity = useMemo(() => {
    if (!selectedTaskId) return agentActivity;
    return agentActivity.filter(entry => entry.data?.task_id === selectedTaskId || (entry.message && entry.message.includes(selectedTaskId)) || (tasks.find(t => t.id === selectedTaskId)?.name && entry.message && entry.message.includes(tasks.find(t => t.id === selectedTaskId).name)));
  }, [agentActivity, selectedTaskId, tasks]);

  return (
    <div className="relative min-h-full space-y-8 animate-fade-in pb-12">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Detail Header - Minimalist */}
      <div className="md:sticky md:top-14 z-20 -mx-3 sm:-mx-6 px-6 py-4 border-b border-border/20 bg-background/60 backdrop-blur-xl mb-8 transition-all">
        <div className="flex flex-col md:flex-row md:items-center gap-6">
          <div className="flex items-center gap-4 w-full md:w-auto min-w-0">
            <button onClick={onBack} className="p-2 rounded-xl hover:bg-accent transition-all shrink-0 border border-transparent">
              <ArrowLeft className="w-5 h-5 text-muted-foreground/60" />
            </button>
            <div className="min-w-0 flex-1 space-y-0.5">
              <div className="flex items-center gap-2 group">
                <h1 className="text-lg font-bold text-foreground line-clamp-1 tracking-tight">{displayTitle}</h1>
                {canEdit && <button onClick={() => setEditModalOpen(true)} className="p-1 rounded-lg opacity-0 group-hover:opacity-100 hover:bg-accent text-muted-foreground/40 transition-all"><Pencil className="w-3.5 h-3.5" /></button>}
              </div>
              <div className="flex items-center gap-3">
                <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground/40 font-bold">{workflow.id.substring(0, 12)}</span>
                <ExecutionModeBadge config={workflow.config} variant="inline" />
              </div>
            </div>
          </div>

          <div className="hidden md:flex flex-1 justify-center"><PhaseStepper workflow={workflow} compact /></div>

          <div className="flex items-center gap-2 flex-wrap md:justify-end w-full md:w-auto">
            {workflow.status === 'running' && <AgentActivityCompact activeAgents={activeAgents} />}
            {workflow.status === 'pending' && (
              <div className="flex gap-2 w-full md:w-auto">
                <Button onClick={() => startWorkflow(workflow.id)} disabled={loading} size="sm" className="flex-1 md:flex-none font-bold px-5 h-9 rounded-xl shadow-sm">Start Cycle</Button>
                <Button onClick={() => analyzeWorkflow(workflow.id)} disabled={loading} variant="outline" size="sm" className="flex-1 md:flex-none font-bold px-5 h-9 rounded-xl">Analyze</Button>
              </div>
            )}
            {workflow.status === 'completed' && workflow.current_phase === 'plan' && <Button onClick={() => planWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-5 h-9 rounded-xl">Initialize Plan</Button>}
            {workflow.status === 'completed' && (workflow.current_phase === 'plan' || workflow.current_phase === 'execute') && <Button onClick={() => setReplanModalOpen(true)} disabled={loading} variant="outline" size="sm" className="font-bold px-5 h-9 rounded-xl">Re-Architect</Button>}
            {workflow.status === 'completed' && workflow.current_phase === 'execute' && <Button onClick={() => executeWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-5 h-9 rounded-xl">Execute Plan</Button>}
            {['execute', 'done'].includes(workflow.current_phase) && <Button onClick={() => setShowIssuesModal(true)} disabled={issuesGenerating} variant="outline" size="sm" className="font-bold px-5 h-9 rounded-xl">Build Issues</Button>}
            {workflow.status === 'paused' && <Button onClick={() => resumeWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-5 h-9 rounded-xl">Resume Run</Button>}
            {workflow.status === 'failed' && <Button onClick={() => startWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-5 h-9 rounded-xl">Retry</Button>}
            {workflow.status === 'running' && <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-xl bg-primary/5 text-primary text-[9px] font-bold uppercase tracking-widest border border-primary/10 shadow-sm"><Loader2 className="w-3.5 h-3.5 animate-spin" /> {workflow.current_phase || 'RUNNING'}</div>}
            {workflow.status === 'running' && (
              <div className="flex gap-2">
                <Button onClick={() => pauseWorkflow(workflow.id)} variant="outline" size="sm" className="font-bold h-9 rounded-xl">Pause</Button>
                <Button onClick={() => stopWorkflow(workflow.id)} variant="outline" size="sm" className="font-bold h-9 rounded-xl border-destructive/20 text-destructive hover:bg-destructive/5">Kill</Button>
              </div>
            )}
            {workflow.status !== 'pending' && <Button onClick={handleDownloadArtifacts} variant="outline" size="sm" className="font-bold h-9 rounded-xl" title="Bundle Report"><Download className="w-4 h-4 mr-2" />Bundle</Button>}
            {canDelete && <Button onClick={() => setDeleteDialogOpen(true)} variant="outline" size="sm" className="font-bold h-9 rounded-xl border-destructive/10 text-destructive/40 hover:text-destructive"><Trash2 className="w-4 h-4" /></Button>}
          </div>
        </div>
      </div>

      {/* Main Grid View */}
      <div className="mx-auto max-w-7xl w-full grid grid-cols-1 lg:grid-cols-3 gap-10">
        <div className="lg:col-span-3">
          <div className="bg-card/10 backdrop-blur-sm border border-border/30 rounded-3xl p-8 flex flex-col md:flex-row md:items-start justify-between gap-10">
              <div className="flex-1 min-w-0 space-y-6">
                <div className="flex flex-wrap items-center gap-4">
                  <StatusBadge status={workflow.status} />
                  <div className="h-3 w-px bg-border/20 mx-1" />
                  <ExecutionModeBadge config={workflow.config} variant="inline" />
                </div>
                <blockquote className="text-base font-medium text-foreground/70 leading-relaxed italic border-l-2 border-primary/20 pl-6 py-1">
                  &ldquo;{workflow.prompt || 'No instruction data'}&rdquo;
                </blockquote>
              </div>
              
              <div className="grid grid-cols-2 gap-x-10 gap-y-6 min-w-[280px]">
                {[
                  { label: 'System Phase', val: workflow.current_phase || 'Ready', icon: Layers },
                  { label: 'Agent Nodes', val: `${tasks.length} Nodes`, icon: Network },
                  { label: 'Established', val: workflow.created_at ? new Date(workflow.created_at).toLocaleDateString() : '—', icon: Clock },
                  { label: 'Last Pulse', val: workflow.updated_at ? new Date(workflow.updated_at).toLocaleDateString() : '—', icon: RefreshCw }
                ].map((item, i) => (
                  <div key={i} className="space-y-1">
                    <p className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40">{item.label}</p>
                    <div className="flex items-center gap-2 text-xs font-semibold text-foreground/80">
                       <item.icon className="w-3 h-3 text-primary/40" />
                       {item.val}
                    </div>
                  </div>
                ))}
              </div>
          </div>
        </div>

        <div className="space-y-8">
          {(agentActivity.length > 0 || activeAgents.length > 0) && (
            <AgentActivity workflowId={workflow.id} activity={filteredActivity} activeAgents={activeAgents} expanded={activityExpanded} onToggle={() => setActivityExpanded(!activityExpanded)} workflowStartTime={workflow.status === 'running' ? workflow.updated_at : null} />
          )}

          <div className="bg-card/10 border border-border/20 rounded-3xl p-6 group">
              <div className="flex items-center justify-between mb-6">
                <button type="button" onClick={() => setAttachmentsExpanded(!attachmentsExpanded)} className="flex items-center gap-3 text-[10px] font-bold uppercase tracking-widest text-muted-foreground/60 hover:text-primary transition-all">
                  <Badge variant="secondary" className="px-1.5 py-0 bg-primary/5 text-primary/60 border-primary/10">{workflow.attachments?.length || 0}</Badge>
                  Assets
                  {attachmentsExpanded ? <ChevronUp className="w-3.5 h-3.5 opacity-40" /> : <ChevronDown className="w-3.5 h-3.5 opacity-40" />}
                </button>
                {canModifyAttachments && (
                  <Button variant="ghost" size="sm" onClick={() => attachmentInputRef.current?.click()} disabled={attachmentUploading} className="h-7 px-3 rounded-lg text-[9px] font-bold uppercase tracking-widest bg-primary/[0.02] border border-primary/5">Ingest</Button>
                )}
              </div>
              <input ref={attachmentInputRef} type="file" multiple className="hidden" disabled={attachmentUploading} onChange={handleAttachmentSelect} />

              {attachmentsExpanded && (
                <div className="space-y-2 animate-fade-in">
                  {(workflow.attachments || []).length > 0 ? workflow.attachments.map((a) => (
                    <div key={a.id} className="flex items-center justify-between gap-4 p-2.5 rounded-xl border border-border/20 bg-background/20 hover:border-primary/10 transition-all group/item">
                      <div className="min-w-0 flex items-center gap-3">
                        <FileText className="w-3.5 h-3.5 text-muted-foreground/40 group-hover/item:text-primary transition-all" />
                        <div className="min-w-0"><p className="text-[11px] font-semibold text-foreground/70 truncate">{a.name}</p></div>
                      </div>
                      <div className="flex items-center gap-1 opacity-0 group-hover/item:opacity-100 transition-opacity">
                        <button onClick={() => handleDownloadAttachment(a)} className="p-1 rounded hover:bg-accent transition-all"><Download className="w-3.5 h-3.5 text-muted-foreground/40" /></button>
                        {canModifyAttachments && <button onClick={() => handleDeleteAttachment(a)} className="p-1 rounded hover:bg-destructive/5 transition-all"><Trash2 className="w-3.5 h-3.5 text-muted-foreground/40 hover:text-destructive" /></button>}
                      </div>
                    </div>
                  )) : <p className="text-[10px] text-center py-4 text-muted-foreground/30 font-medium italic">Empty context pool</p>}
                </div>
              )}
          </div>

          <div className="bg-card/10 border border-border/20 rounded-3xl p-6">
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/60">Execution Queue</h3>
                <div className="flex bg-muted/20 p-0.5 rounded-xl border border-border/20 shadow-inner">
                  <button onClick={() => setTaskView('list')} className={`p-1.5 rounded-lg transition-all ${taskView === 'list' ? 'bg-background shadow-sm text-primary' : 'text-muted-foreground/40'}`}><LayoutList className="w-3.5 h-3.5" /></button>
                  <button onClick={() => setTaskView('graph')} className={`p-1.5 rounded-lg transition-all ${taskView === 'graph' ? 'bg-background shadow-sm text-primary' : 'text-muted-foreground/40'}`}><Network className="w-3.5 h-3.5" /></button>
                </div>
              </div>
              {taskView === 'list' ? (
                tasks.length > 0 ? <div className="space-y-2.5 max-h-[50vh] overflow-y-auto pr-1 scrollbar-none">{tasks.map((t) => <TaskItem key={t.id} task={t} selected={selectedDoc?.key?.includes(`:${t.id}`)} onClick={() => selectTask(t)} />)}</div> 
                : <p className="text-[10px] text-center py-10 text-muted-foreground/20 italic font-bold tracking-widest uppercase">No active nodes</p>
              ) : <WorkflowGraph tasks={tasks} />}
          </div>
        </div>

        <div className="lg:col-span-2 flex flex-col min-h-[60vh] max-h-[85vh] bg-card/10 backdrop-blur-xl border border-border/30 rounded-[2rem] shadow-soft overflow-hidden group/preview">
          <div className="flex items-center justify-between gap-6 px-8 py-5 border-b border-border/20 bg-background/20">
            <div className="min-w-0 flex items-center gap-4">
              <div className="p-2 rounded-xl bg-background border border-border/40 shadow-sm text-primary/50"><Terminal className="w-4 h-4" /></div>
              <h3 className="text-base font-bold text-foreground truncate tracking-tight">{selectedDoc?.title || 'System Output'}</h3>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="icon" onClick={handleCopy} disabled={!docContent} className={`h-9 w-9 rounded-lg transition-all ${copied ? 'text-success bg-success/5' : 'text-muted-foreground/30 hover:text-primary'}`}>{copied ? <CheckCircle2 className="w-4 h-4" /> : <Copy className="w-4 h-4" />}</Button>
              <Button variant="ghost" size="icon" onClick={handleRefresh} disabled={!selectedDoc} className="h-9 w-9 rounded-lg text-muted-foreground/30 hover:text-primary"><RefreshCw className="w-4 h-4" /></Button>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto min-h-0 p-8 scrollbar-thin scrollbar-thumb-muted/20 selection:bg-primary/5">
            {docLoading ? <div className="space-y-6 animate-pulse"><div className="h-6 w-1/4 bg-muted/10 rounded-xl" /><div className="space-y-3">{[...Array(4)].map((_, i) => <div key={i} className="h-3 w-full bg-muted/5 rounded-lg" />)}</div></div>
            : docError ? <div className="text-center py-20"><p className="text-sm text-destructive font-bold">{docError}</p></div>
            : selectedDoc ? <div className="animate-fade-in h-full">{selectedDoc.path && !selectedDoc.path.endsWith('.md') ? <CodeEditor value={docContent} language={selectedDoc.path.split('.').pop()} readOnly={true} /> : <MarkdownViewer markdown={docContent} />}</div>
            : <div className="flex flex-col items-center justify-center h-full text-center gap-4 opacity-20"><Search className="w-12 h-12" /><p className="text-xs font-bold uppercase tracking-widest leading-none">Awaiting telemetry selection</p></div>}
          </div>
        </div>
      </div>

      <EditWorkflowModal isOpen={editModalOpen} onClose={() => setEditModalOpen(false)} workflow={workflow} onSave={handleSaveWorkflow} canEditPrompt={canEditPrompt} />
      <ConfirmDialog isOpen={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)} onConfirm={handleDelete} title="Purge Execution?" message={`Confirming the irreversible erasure of the lifecycle data for "${displayTitle}".`} confirmText="Confirm Purge" variant="danger" />
      <ReplanModal isOpen={isReplanModalOpen} onClose={() => setReplanModalOpen(false)} onSubmit={async (ctx) => { await replanWorkflow(workflow.id, ctx); setReplanModalOpen(false); }} loading={loading} />
      <GenerationOptionsModal isOpen={showIssuesModal} onClose={() => setShowIssuesModal(false)} onSelect={handleIssuesModeSelect} loading={issuesGenerating} />
    </div>
  );
}

const AGENT_OPTIONS = [ { value: 'claude', label: 'Claude' }, { value: 'gemini', label: 'Gemini' }, { value: 'codex', label: 'Codex' } ];

function NewWorkflowForm({ onSubmit, onCancel, loading, initialData }) {
  const [title, setTitle] = useState(initialData?.name || '');
  const [prompt, setPrompt] = useState(initialData?.prompt || '');
  const [files, setFiles] = useState([]);
  const fileInputRef = useRef(null);
  const [executionMode, setExecutionMode] = useState(initialData?.executionStrategy === 'single-agent' ? 'single_agent' : 'multi_agent');
  const [singleAgentName, setSingleAgentName] = useState('claude');
  const [singleAgentModel, setSingleAgentModel] = useState('');
  const [singleAgentReasoningEffort, setSingleAgentReasoningEffort] = useState('');
  useEnums();
  const { config } = useConfigStore();
  const enabledAgents = useMemo(() => { if (!config?.agents) return AGENT_OPTIONS; return AGENT_OPTIONS.filter(o => config.agents[o.value]?.enabled !== false); }, [config]);
  const effectiveAgent = enabledAgents.some(a => a.value === singleAgentName) ? singleAgentName : (enabledAgents[0]?.value || singleAgentName);
  const modelOptions = getModelsForAgent(effectiveAgent);
  const reasoningLevels = getReasoningLevels();
  const supportsReason = supportsReasoning(effectiveAgent);
  const effectiveModel = modelOptions.some(m => m.value === singleAgentModel) ? singleAgentModel : '';
  const effectiveReason = supportsReason && reasoningLevels.some(r => r.value === singleAgentReasoningEffort) ? singleAgentReasoningEffort : '';
  const selectedReasoning = reasoningLevels.find(r => r.value === effectiveReason);

  const handleSubmit = (e) => { e.preventDefault(); if (!prompt.trim()) return; const wfConfig = executionMode === 'single_agent' ? { execution_mode: 'single_agent', single_agent_name: effectiveAgent, ...(effectiveModel ? { single_agent_model: effectiveModel } : {}), ...(supportsReason && effectiveReason ? { single_agent_reasoning_effort: effectiveReason } : {}) } : undefined; onSubmit(prompt, files, title.trim() || undefined, wfConfig); };
  const removeFile = (i) => setFiles(p => p.filter((_, idx) => idx !== i));

  return (
    <div className="relative w-full animate-fade-in pb-20">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <div className="max-w-3xl mx-auto p-10 rounded-[2.5rem] border border-border/30 bg-card/20 backdrop-blur-2xl shadow-soft animate-fade-up">
        <div className="flex items-center gap-4 mb-10">
           <div className="p-3 rounded-2xl bg-primary/[0.02] border border-primary/5 text-primary/60"><Plus className="w-6 h-6" /></div>
           <h2 className="text-2xl font-bold text-foreground tracking-tight">New Cycle</h2>
        </div>
        <form onSubmit={handleSubmit} className="space-y-10">
          <div className="space-y-6">
            <div className="space-y-2"><label className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 ml-1">Title</label><Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Cycle identifier..." className="h-12 bg-background/30 rounded-xl border-border/30" /></div>
            <div className="space-y-2"><label className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 ml-1">Instruction</label><div className="relative group/prompt"><textarea value={prompt} onChange={(e) => setPrompt(e.target.value)} placeholder="Describe neural workflow..." rows={6} className="w-full px-6 py-4 rounded-2xl border border-border/30 bg-background/30 text-foreground placeholder:text-muted-foreground/20 focus:outline-none focus:ring-1 focus:ring-primary/10 transition-all resize-none font-mono text-sm leading-relaxed" /><VoiceInputButton onTranscript={(t) => setPrompt(p => p ? p + ' ' + t : t)} disabled={loading} className="absolute top-4 right-4" /></div></div>
          </div>
          <div className="space-y-6 pt-8 border-t border-border/20">
             <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {[ { id: 'multi_agent', icon: Network, title: 'Consensus', sub: 'High precision iterative loop.' }, { id: 'single_agent', icon: Zap, title: 'Direct', sub: 'Linear streamlined pipeline.' } ].map(mode => (
                <button key={mode.id} type="button" onClick={() => setExecutionMode(mode.id)} className={`relative flex flex-col items-start p-5 rounded-2xl border transition-all duration-500 ${executionMode === mode.id ? 'border-primary/30 bg-primary/[0.02] shadow-inner' : 'border-border/30 bg-background/10 hover:border-primary/10'}`}>
                  <div className="flex items-center justify-between w-full mb-3"><div className={`p-2 rounded-xl transition-colors ${executionMode === mode.id ? 'text-primary' : 'text-muted-foreground/30'}`}><mode.icon className="w-5 h-5" /></div>{executionMode === mode.id && <CheckCircle2 className="w-4 h-4 text-primary/60" />}</div>
                  <span className="font-bold text-sm text-foreground/80">{mode.title}</span>
                  <span className="text-[10px] text-muted-foreground/40 mt-1 leading-relaxed font-medium">{mode.sub}</span>
                </button>
              ))}
             </div>
            {executionMode === 'single_agent' && (
              <div className="mt-4 p-6 rounded-2xl border border-border/20 bg-muted/5 space-y-6">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
                  <div className="space-y-1.5"><label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40 ml-1">Provider</label><select value={effectiveAgent} onChange={(e) => { setSingleAgentName(e.target.value); setSingleAgentModel(''); setSingleAgentReasoningEffort(''); }} className="w-full h-10 px-3 rounded-xl border border-border/30 bg-background/50 text-xs font-bold focus:ring-1 focus:ring-primary/10 outline-none transition-all">{enabledAgents.map(a => <option key={a.value} value={a.value}>{a.label}</option>)}</select></div>
                  <div className="space-y-1.5"><label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40 ml-1">Model</label><select value={effectiveModel} onChange={(e) => setSingleAgentModel(e.target.value)} className="w-full h-10 px-3 rounded-xl border border-border/30 bg-background/50 text-xs font-bold focus:ring-1 focus:ring-primary/10 outline-none transition-all">{modelOptions.map(m => <option key={m.value} value={m.value}>{m.label}</option>)}</select></div>
                </div>
              </div>
            )}
          </div>
          <div className="flex flex-col sm:flex-row gap-3 pt-6">
            <Button type="submit" disabled={loading || !prompt.trim()} className="flex-1 rounded-xl h-12 text-xs font-bold uppercase tracking-widest shadow-lg shadow-primary/10">{loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4 mr-2" />}Initialize</Button>
            <Button type="button" variant="ghost" onClick={onCancel} className="rounded-xl h-12 px-8 text-xs font-bold text-muted-foreground/40">Abort</Button>
          </div>
        </form>
      </div>
    </div>
  );
}

const STATUS_FLT = [ { value: 'all', label: 'History', icon: List }, { value: 'running', label: 'Active', icon: Activity }, { value: 'completed', label: 'Done', icon: CheckCircle2 }, { value: 'failed', label: 'Faults', icon: XCircle } ];

export default function Workflows() {
  const { id } = useParams(); const navigate = useNavigate(); const location = useLocation(); const [searchParams, setSearchParams] = useSearchParams();
  const { workflows, loading, error, fetchWorkflows, fetchWorkflow, createWorkflow, deleteWorkflow, clearError } = useWorkflowStore();
  const { getTasksForWorkflow, setTasks } = useTaskStore(); const notifyInfo = useUIStore((s) => s.notifyInfo); const notifyError = useUIStore((s) => s.notifyError);
  const [showNewForm, setShowNewForm] = useState(false); const [filter, setFilter] = useState('');
  const statusFilter = searchParams.get('status') || 'all';
  const setStatusFilter = useCallback((s) => { if (s === 'all') searchParams.delete('status'); else searchParams.set('status', s); setSearchParams(searchParams); }, [searchParams, setSearchParams]);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false); const [workflowToDelete, setWorkflowToDelete] = useState(null);

  useEffect(() => { fetchWorkflows(); }, [fetchWorkflows]);
  useEffect(() => { if (id && id !== 'new') fetchWorkflow(id); else if (id === 'new' || location.state?.template) setShowNewForm(true); }, [fetchWorkflow, id, location.state]);

  const filteredWorkflows = useMemo(() => {
    let res = workflows;
    if (statusFilter && statusFilter !== 'all') res = res.filter(w => w.status === statusFilter);
    if (filter) { const f = filter.toLowerCase(); res = res.filter(w => (deriveWorkflowTitle(w).toLowerCase().includes(f)) || (w.id && w.id.toLowerCase().includes(f)) || (w.prompt && w.prompt.toLowerCase().includes(f))); }
    return res;
  }, [workflows, filter, statusFilter]);

  const handleDeleteClick = useCallback((wf) => { setWorkflowToDelete(wf); setDeleteDialogOpen(true); }, []);
  const handleDeleteConfirm = useCallback(async () => { if (!workflowToDelete) return; if (await deleteWorkflow(workflowToDelete.id)) notifyInfo('Cycle purged'); else notifyError('Purge failed'); setWorkflowToDelete(null); setDeleteDialogOpen(false); }, [workflowToDelete, deleteWorkflow, notifyInfo, notifyError]);

  if (id === 'new' || showNewForm) return <NewWorkflowForm onSubmit={async (p, f, t, c) => { const wf = await createWorkflow(p, { title: t, config: c }); if (!wf) { notifyError(useWorkflowStore.getState().error || 'Failed'); clearError(); return; } if (f.length) try { await workflowApi.uploadAttachments(wf.id, f); await fetchWorkflow(wf.id); } catch {} setShowNewForm(false); navigate(`/workflows/${wf.id}`); }} onCancel={() => { setShowNewForm(false); if (id === 'new') navigate('/workflows'); navigate(location.pathname, { replace: true, state: {} }); }} loading={loading} initialData={location.state?.template} />;
  if (selectedWorkflow) return <WorkflowDetail workflow={selectedWorkflow} tasks={id ? getTasksForWorkflow(id) : []} onBack={() => navigate('/workflows')} />;

  return (
    <div className="relative min-h-full space-y-10 animate-fade-in pb-12">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 pt-4 pb-2 border-b border-border/20">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-primary"><div className="w-1 h-1 rounded-full bg-current" /><span className="text-[10px] font-bold uppercase tracking-widest opacity-70">Automation Registry</span></div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Workflow <span className="text-muted-foreground/40 font-medium">History</span></h1>
        </div>
        <div className="flex items-center gap-3">
           <Link to="/workflows/new" className="px-5 py-2 rounded-xl bg-primary text-primary-foreground text-xs font-bold hover:bg-primary/90 transition-all shadow-sm">New Blueprint</Link>
        </div>
      </div>

      <div className="sticky top-14 z-30 flex flex-col gap-6 bg-background/80 backdrop-blur-xl py-4 border-b border-border/10 transition-all duration-500">
        <div className="flex flex-col md:flex-row gap-6 md:items-center justify-between">
          <div className="relative w-full sm:w-80 group">
            <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" />
            <Input placeholder="Filter stream..." value={filter} onChange={(e) => setFilter(e.target.value)} className="h-10 pl-11 pr-6 rounded-2xl border-border/20 bg-card/20 text-xs shadow-sm focus-visible:ring-primary/10 transition-all" />
          </div>
          <div className="flex items-center gap-1.5 p-1 rounded-2xl bg-card/20 border border-border/20 shadow-inner">
            {STATUS_FLT.map(f => <button key={f.value} onClick={() => setStatusFilter(f.value)} className={`flex items-center gap-2 px-4 py-2 rounded-xl text-[9px] font-bold uppercase tracking-widest transition-all duration-500 ${statusFilter === f.value ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/5' : 'text-muted-foreground/40 hover:text-foreground'}`}>{f.label}</button>)}
          </div>
        </div>
      </div>

      {loading && workflows.length === 0 ? <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-8">{[...Array(6)].map((_, i) => <div key={i} className="h-48 rounded-3xl bg-muted/5 animate-pulse border border-border/10" />)}</div>
      : filteredWorkflows.length > 0 ? <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">{filteredWorkflows.map(w => <WorkflowCard key={w.id} workflow={w} onClick={() => navigate(`/workflows/${w.id}`)} onDelete={handleDeleteClick} />)}</div>
      : <div className="text-center py-32 opacity-20 flex flex-col items-center gap-4"><GitBranch className="w-12 h-12" /><p className="text-xs font-bold uppercase tracking-widest">Registry Empty</p></div>}
      <FAB onClick={() => navigate('/workflows/new')} icon={Plus} label="New Workflow" />
      <ConfirmDialog isOpen={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)} onConfirm={handleDeleteConfirm} title="Purge Record?" message={`IRREVERSIBLE: Permanently delete record for "${deriveWorkflowTitle(workflowToDelete)}"?`} confirmText="Confirm Purge" variant="danger" />
    </div>
  );
}