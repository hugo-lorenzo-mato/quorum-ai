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
  Settings,
  ArrowUpRight
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
  const { bg, text } = getStatusColor(status);
  const iconMap = { pending: Clock, running: Activity, completed: CheckCircle2, failed: XCircle, paused: Pause };
  const StatusIcon = iconMap[status] || Clock;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${bg} ${text} border border-current opacity-80`}>
      <StatusIcon className="w-3 h-3" /> {status}
    </span>
  );
}

// Workflow Card - Refined
function WorkflowCard({ workflow, onClick, onDelete }) {
  const canDelete = workflow.status !== 'running';
  const statusColor = getStatusColor(workflow.status);

  return (
    <div
      onClick={onClick}
      className={`group flex flex-col h-full rounded-2xl border border-border/40 bg-card/40 backdrop-blur-md transition-all duration-500 hover:shadow-[0_8px_30px_rgb(0,0,0,0.04)] hover:border-primary/20 hover:-translate-y-1 overflow-hidden border-l-[2px]`}
      style={statusColor.borderStrip ? { borderLeftColor: `var(--${statusColor.borderStrip.split('-')[1]})` } : {}}
    >
      <div className="flex-1 p-6 space-y-5 cursor-pointer">
        <div className="flex items-start justify-between gap-3">
          <div className={`p-2.5 rounded-xl bg-background border border-border/60 shadow-sm group-hover:border-primary/30 transition-colors duration-500`}>
            <GitBranch className="h-5 w-5 text-primary/70" />
          </div>
          <div className="flex items-center gap-2">
             <StatusBadge status={workflow.status} />
             {canDelete && (
                <button
                  onClick={(e) => { e.stopPropagation(); onDelete(workflow); }}
                  className="p-1.5 rounded-lg opacity-0 group-hover:opacity-100 hover:bg-destructive/5 text-muted-foreground/40 hover:text-destructive transition-all duration-300"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              )}
          </div>
        </div>

        <div className="space-y-2">
          <h3 className="font-bold text-lg text-foreground line-clamp-2 leading-snug group-hover:text-primary transition-colors duration-300">
            {deriveWorkflowTitle(workflow)}
          </h3>
          <p className="text-[10px] text-muted-foreground/40 font-mono tracking-tight uppercase">
            ID: {workflow.id.substring(0, 12)}...
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-5 pt-2">
           <div className="flex items-center gap-2">
              <Layers className="w-3.5 h-3.5 text-muted-foreground/40" />
              <span className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/60">
                Phase: {workflow.current_phase || 'INITIAL'}
              </span>
           </div>
           <div className="flex items-center gap-2">
              <LayoutList className="w-3.5 h-3.5 text-muted-foreground/40" />
              <span className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/60">
                {workflow.task_count || 0} Sub-tasks
              </span>
           </div>
        </div>
      </div>

      <div className="px-6 py-4 mt-auto flex items-center justify-between border-t border-border/30 bg-primary/[0.01]">
         <ExecutionModeBadge config={workflow.config} variant="inline" />
         <ChevronRight className="w-4 h-4 text-muted-foreground/20 group-hover:text-primary group-hover:translate-x-0.5 transition-all" />
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
          ? 'border-primary/30 bg-primary/5 shadow-sm scale-[1.01] z-10'
          : 'border-border/40 bg-card/40 hover:border-primary/20 hover:bg-accent/30'
      }`}
    >
      <div className={`p-2.5 rounded-xl ${bg} border ${border} shadow-sm transition-transform duration-500`}>
        <StatusIcon className={`w-4 h-4 ${text} ${isRunning ? 'animate-spin' : ''}`} />
      </div>
      <div className="flex-1 min-w-0 text-left">
        <p className="text-sm font-bold text-foreground truncate group-hover:text-primary transition-colors">{task.name || task.id}</p>
        <p className="text-[10px] text-muted-foreground font-mono mt-0.5 opacity-40">{task.id}</p>
      </div>
      <div className={`px-2 py-0.5 rounded-lg text-[9px] font-black uppercase tracking-widest ${bg} ${text} border border-current/20`}>
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

      {/* Glass Header Detail */}
      <div className="md:sticky md:top-14 z-20 -mx-3 sm:-mx-6 px-6 py-5 border-b border-border/40 bg-background/60 backdrop-blur-xl shadow-sm mb-8 transition-all duration-500">
        <div className="flex flex-col md:flex-row md:items-center gap-6">
          <div className="flex items-center gap-5 w-full md:w-auto min-w-0">
            <button onClick={onBack} className="p-2.5 rounded-xl hover:bg-primary/5 transition-all shrink-0 border border-border/40 hover:border-primary/20">
              <ArrowLeft className="w-5 h-5 text-muted-foreground/60" />
            </button>
            <div className="min-w-0 flex-1 space-y-1">
              <div className="flex items-center gap-2 group">
                <h1 className="text-xl font-bold text-foreground line-clamp-1 tracking-tight">{displayTitle}</h1>
                {canEdit && <button onClick={() => setEditModalOpen(true)} className="p-1.5 rounded-lg opacity-0 group-hover:opacity-100 hover:bg-primary/5 text-muted-foreground/40 hover:text-primary transition-all"><Pencil className="w-4 h-4" /></button>}
              </div>
              <div className="flex items-center gap-3">
                <span className="text-[10px] font-mono uppercase tracking-tight text-muted-foreground/40 font-bold">UID: {workflow.id.substring(0, 12)}...</span>
                <ExecutionModeBadge config={workflow.config} variant="inline" />
              </div>
            </div>
          </div>

          <div className="hidden md:flex flex-1 justify-center"><PhaseStepper workflow={workflow} compact /></div>

          <div className="flex items-center gap-2 flex-wrap md:justify-end w-full md:w-auto">
            {workflow.status === 'running' && <AgentActivityCompact activeAgents={activeAgents} />}
            {workflow.status === 'pending' && (
              <div className="flex gap-2 w-full md:w-auto">
                <Button onClick={() => startWorkflow(workflow.id)} disabled={loading} size="sm" className="flex-1 md:flex-none font-bold px-6 h-9 rounded-xl shadow-lg shadow-primary/10">Start Cycle</Button>
                <Button onClick={() => analyzeWorkflow(workflow.id)} disabled={loading} variant="outline" size="sm" className="flex-1 md:flex-none font-bold px-6 h-9 rounded-xl border-info/20 text-info hover:bg-info/5">Analyze</Button>
              </div>
            )}
            {workflow.status === 'completed' && workflow.current_phase === 'plan' && <Button onClick={() => planWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-6 h-9 rounded-xl">Initialize Plan</Button>}
            {workflow.status === 'completed' && (workflow.current_phase === 'plan' || workflow.current_phase === 'execute') && <Button onClick={() => setReplanModalOpen(true)} disabled={loading} variant="outline" size="sm" className="font-bold px-6 h-9 rounded-xl border-warning/20 text-warning hover:bg-warning/5">Re-Architect</Button>}
            {workflow.status === 'completed' && workflow.current_phase === 'execute' && <Button onClick={() => executeWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-6 h-9 rounded-xl">Execute Plan</Button>}
            {['execute', 'done'].includes(workflow.current_phase) && <Button onClick={() => setShowIssuesModal(true)} disabled={issuesGenerating} variant="outline" size="sm" className="font-bold px-6 h-9 rounded-xl border-primary/20 text-primary hover:bg-primary/5">Build Issues</Button>}
            {workflow.status === 'paused' && <Button onClick={() => resumeWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-6 h-9 rounded-xl">Resume Run</Button>}
            {workflow.status === 'failed' && <Button onClick={() => startWorkflow(workflow.id)} disabled={loading} size="sm" className="font-bold px-6 h-9 rounded-xl">Emergency Retry</Button>}
            {workflow.status === 'running' && <div className="inline-flex items-center gap-2 px-4 py-1.5 rounded-xl bg-primary/5 text-primary text-[10px] font-bold uppercase tracking-[0.2em] border border-primary/10 shadow-sm"><Loader2 className="w-3.5 h-3.5 animate-spin" /> {workflow.current_phase || 'RUNNING'}</div>}
            {workflow.status === 'running' && (
              <div className="flex gap-2">
                <Button onClick={() => pauseWorkflow(workflow.id)} variant="outline" size="sm" className="font-bold h-9 rounded-xl border-border/60">Pause</Button>
                <Button onClick={() => stopWorkflow(workflow.id)} variant="outline" size="sm" className="font-bold h-9 rounded-xl bg-destructive/5 border-destructive/20 text-destructive hover:bg-destructive/10">Kill Process</Button>
              </div>
            )}
            {workflow.status !== 'pending' && <Button onClick={handleDownloadArtifacts} variant="outline" size="sm" className="font-bold h-9 rounded-xl border-border/60" title="Bundle Report"><Download className="w-4 h-4 mr-2" />Bundle</Button>}
            {canDelete && <Button onClick={() => setDeleteDialogOpen(true)} variant="outline" size="sm" className="font-bold h-9 rounded-xl border-destructive/10 text-destructive/40 hover:text-destructive hover:bg-destructive/5"><Trash2 className="w-4 h-4" /></Button>}
          </div>
        </div>
      </div>

      {workflow.status === 'failed' && workflow.error && (
        <div className="mx-auto max-w-5xl p-5 rounded-2xl bg-destructive/[0.02] border border-destructive/20 animate-fade-in shadow-sm">
          <div className="flex items-start gap-4">
            <div className="p-2 rounded-xl bg-destructive/10 text-destructive shadow-inner"><XCircle className="w-5 h-5" /></div>
            <div className="flex-1 min-w-0 space-y-1">
              <p className="text-[10px] font-bold uppercase tracking-[0.2em] text-destructive/70">Terminal Error Log</p>
              <p className="text-sm font-medium text-foreground leading-relaxed break-words">{workflow.error}</p>
            </div>
          </div>
        </div>
      )}

      {/* Detailed Info Card */}
      <div className="mx-auto max-w-7xl w-full grid grid-cols-1 lg:grid-cols-3 gap-10">
        <div className="lg:col-span-3">
          <Card className="bg-card/30 backdrop-blur-xl border-border/40 overflow-hidden shadow-sm">
            <div className="p-8 flex flex-col md:flex-row md:items-start justify-between gap-10">
              <div className="flex-1 min-w-0 space-y-6">
                <div className="flex flex-wrap items-center gap-4">
                  <StatusBadge status={workflow.status} />
                  <div className="h-4 w-px bg-border/40 mx-1" />
                  <ExecutionModeBadge config={workflow.config} variant="inline" />
                </div>
                <blockquote className="text-lg font-medium text-foreground/80 leading-relaxed italic border-l-4 border-primary/20 pl-8 py-2 bg-primary/[0.01] rounded-r-2xl">
                  &ldquo;{workflow.prompt || 'No instruction data provided'}&rdquo;
                </blockquote>
              </div>
              
              <div className="grid grid-cols-2 gap-x-12 gap-y-8 min-w-[300px]">
                {[
                  { label: 'System Phase', val: workflow.current_phase || 'Ready', icon: Layers },
                  { label: 'Intelligence Nodes', val: `${tasks.length} Agents`, icon: Network },
                  { label: 'Cycle Created', val: workflow.created_at ? new Date(workflow.created_at).toLocaleDateString() : '—', icon: Clock },
                  { label: 'Last Checksum', val: workflow.updated_at ? new Date(workflow.updated_at).toLocaleDateString() : '—', icon: RefreshCw }
                ].map((item, i) => (
                  <div key={i} className="space-y-2">
                    <p className="text-[10px] font-bold uppercase tracking-[0.2em] text-muted-foreground/40">{item.label}</p>
                    <div className="flex items-center gap-2.5 text-sm font-semibold text-foreground/80">
                       <item.icon className="w-3.5 h-3.5 text-primary/40" />
                       {item.val}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </Card>
        </div>

        {/* Inspector Grid - Detailed */}
        <div className="space-y-10">
          {/* Active Agents (only when running) */}
          {(agentActivity.length > 0 || activeAgents.length > 0) && (
            <AgentActivity
              workflowId={workflow.id}
              activity={filteredActivity}
              activeAgents={activeAgents}
              expanded={activityExpanded}
              onToggle={() => setActivityExpanded(!activityExpanded)}
              workflowStartTime={workflow.status === 'running' ? workflow.updated_at : null}
            />
          )}

          {/* Attachments Card - Refined */}
          <Card className="bg-card/20 border-border/40 overflow-hidden group">
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <button type="button" onClick={() => setAttachmentsExpanded(!attachmentsExpanded)} className="flex items-center gap-3 text-[11px] font-bold uppercase tracking-[0.15em] text-muted-foreground/60 hover:text-primary transition-all">
                  <Badge variant="secondary" className="px-1.5 py-0 bg-primary/5 text-primary/60 border-primary/10 font-bold">{workflow.attachments?.length || 0}</Badge>
                  Context Files
                  {attachmentsExpanded ? <ChevronUp className="w-3.5 h-3.5 opacity-40" /> : <ChevronDown className="w-3.5 h-3.5 opacity-40" />}
                </button>
                {canModifyAttachments && (
                  <div className="flex items-center gap-2">
                    <input ref={attachmentInputRef} type="file" multiple className="hidden" disabled={attachmentUploading} onChange={handleAttachmentSelect} />
                    <Button variant="ghost" size="sm" onClick={() => attachmentInputRef.current?.click()} disabled={attachmentUploading} className="h-8 px-3 rounded-lg text-[10px] font-bold uppercase tracking-wider bg-primary/[0.03] hover:bg-primary/10 border border-primary/10">Add Context</Button>
                  </div>
                )}
              </div>

              {attachmentsExpanded && (
                <div className="space-y-2.5 animate-fade-in">
                  {(workflow.attachments || []).length > 0 ? workflow.attachments.map((a) => (
                    <div key={a.id} className="flex items-center justify-between gap-4 p-3 rounded-xl border border-border/30 bg-background/20 hover:border-primary/20 hover:bg-primary/[0.01] transition-all group/item">
                      <div className="min-w-0 flex items-center gap-3">
                        <div className="p-2 rounded-lg bg-muted/30 text-muted-foreground/40 group-hover/item:text-primary group-hover/item:bg-primary/5 transition-all"><FileText className="w-4 h-4" /></div>
                        <div className="min-w-0"><p className="text-xs font-semibold text-foreground/80 truncate">{a.name}</p><p className="text-[9px] font-mono text-muted-foreground/40 mt-0.5">{a.size >= 1024 ? `${Math.round(a.size/1024)} KB` : `${a.size} B`}</p></div>
                      </div>
                      <div className="flex items-center gap-1 opacity-0 group-hover/item:opacity-100 transition-opacity">
                        <button onClick={() => handleDownloadAttachment(a)} className="p-1.5 rounded-lg hover:bg-primary/5 transition-all"><Download className="w-3.5 h-3.5 text-muted-foreground/40 hover:text-primary" /></button>
                        {canModifyAttachments && <button onClick={() => handleDeleteAttachment(a)} className="p-1.5 rounded-lg hover:bg-destructive/5 transition-all"><Trash2 className="w-3.5 h-3.5 text-muted-foreground/40 hover:text-destructive" /></button>}
                      </div>
                    </div>
                  )) : <div className="text-center py-8 rounded-2xl border border-dashed border-border/20 bg-muted/[0.02]"><p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/20">Awaiting context</p></div>}
                </div>
              )}
            </div>
          </Card>

          {/* Tasks Card - Refined */}
          <Card className="bg-card/20 border-border/40 overflow-hidden">
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-3">
                  <h3 className="text-[11px] font-bold uppercase tracking-[0.15em] text-muted-foreground/60">Logic Stack</h3>
                  <Badge variant="secondary" className="px-1.5 py-0 font-bold bg-muted/20 text-muted-foreground/40 border-transparent">{tasks.length}</Badge>
                </div>
                <div className="flex bg-muted/20 p-0.5 rounded-xl border border-border/30 shadow-inner">
                  <button onClick={() => setTaskView('list')} className={`p-1.5 rounded-lg transition-all ${taskView === 'list' ? 'bg-background shadow-sm text-primary' : 'text-muted-foreground/40 hover:text-foreground'}`}><LayoutList className="w-3.5 h-3.5" /></button>
                  <button onClick={() => setTaskView('graph')} className={`p-1.5 rounded-lg transition-all ${taskView === 'graph' ? 'bg-background shadow-sm text-primary' : 'text-muted-foreground/40 hover:text-foreground'}`}><Network className="w-3.5 h-3.5" /></button>
                </div>
              </div>
              {taskView === 'list' ? (
                tasks.length > 0 ? <div className="space-y-3 max-h-[50vh] overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-muted/30">{tasks.map((t) => <TaskItem key={t.id} task={t} selected={selectedDoc?.key?.includes(`:${t.id}`)} onClick={() => selectTask(t)} />)}</div> 
                : <div className="text-center py-12 rounded-2xl border border-dashed border-border/20 bg-muted/[0.02]"><p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/20 italic">Queue uninitialized</p></div>
              ) : <WorkflowGraph tasks={tasks} />}
            </div>
          </Card>

          {/* Artifacts Card - Refined */}
          <Card className="bg-card/20 border-border/40 overflow-hidden">
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-[11px] font-bold uppercase tracking-[0.15em] text-muted-foreground/60">Execution Logs</h3>
                {artifactsLoading && <Loader2 className="w-4 h-4 text-primary animate-spin opacity-40" />}
              </div>
              {!artifactIndex?.reportPath ? <div className="text-center py-10 rounded-2xl border border-dashed border-border/20 bg-muted/[0.02]"><p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/20 italic">Awaiting telemetry</p></div>
              : <div className="space-y-5">
                  <div className="p-2 rounded-lg bg-muted/10 border border-border/30"><p className="text-[9px] font-bold font-mono tracking-tight text-muted-foreground/40 flex items-center gap-2 overflow-hidden"><FolderTree className="w-3 h-3 text-primary/40 shrink-0" /><span className="truncate">{artifactIndex.reportPath}</span></p></div>
                  <div className="space-y-4 max-h-[50vh] overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-muted/30"><FileTree items={docGroups.flatMap(g => g.docs.map(d => ({ ...d, treePath: `${g.label}/${d.title}` })))} onSelect={setSelectedDoc} selectedKey={selectedDoc?.key} /></div>
                </div>}
            </div>
          </Card>
        </div>

        {/* Preview Panel - Refined Hero-like style */}
        <div className="lg:col-span-2 flex flex-col min-h-[70vh] max-h-[90vh] bg-card/40 backdrop-blur-xl border border-border/40 rounded-[2.5rem] shadow-2xl overflow-hidden animate-fade-in group/preview">
          <div className="flex items-center justify-between gap-6 px-10 py-8 border-b border-border/30 bg-primary/[0.01]">
            <div className="min-w-0 flex items-center gap-5">
              <div className="p-3 rounded-2xl bg-background border border-border/60 shadow-sm text-primary/60"><Terminal className="w-5 h-5" /></div>
              <div className="min-w-0 space-y-1">
                <h3 className="text-xl font-bold text-foreground tracking-tight truncate">{selectedDoc?.title || 'System Terminal'}</h3>
                {selectedDoc?.path && <p className="text-[10px] font-mono font-bold text-muted-foreground/30 truncate uppercase tracking-widest">{selectedDoc.path}</p>}
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Button variant="ghost" size="icon" onClick={handleCopy} disabled={!docContent} className={`h-10 w-10 rounded-xl transition-all ${copied ? 'text-success bg-success/5 border-success/20' : 'text-muted-foreground/40 hover:text-primary hover:bg-primary/5'}`}>{copied ? <CheckCircle2 className="w-4 h-4" /> : <Copy className="w-4 h-4" />}</Button>
              <Button variant="ghost" size="icon" onClick={handleRefresh} disabled={!selectedDoc} className="h-10 w-10 rounded-xl text-muted-foreground/40 hover:text-primary hover:bg-primary/5"><RefreshCw className="w-4 h-4" /></Button>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto min-h-0 p-10 scrollbar-thin scrollbar-thumb-muted/30 selection:bg-primary/10">
            {docLoading ? <div className="space-y-8 animate-pulse"><div className="h-8 w-1/4 bg-muted/10 rounded-xl" /><div className="space-y-4">{[...Array(4)].map((_, i) => <div key={i} className="h-4 w-full bg-muted/5 rounded-lg" />)}</div><div className="h-64 w-full bg-muted/[0.03] rounded-3xl border border-border/20" /></div>
            : docError ? <div className="flex flex-col items-center justify-center py-32 text-center gap-6"><div className="p-6 rounded-full bg-destructive/5 text-destructive/40 border border-destructive/10"><XCircle className="w-12 h-12" /></div><div><p className="text-lg font-bold text-foreground/80 tracking-tight">Stream Interrupted</p><p className="text-sm text-muted-foreground/40 mt-2 max-w-xs">{docError}</p></div><Button onClick={handleRefresh} variant="outline" className="rounded-2xl font-bold px-8 border-border/60">Reconnect Stream</Button></div>
            : selectedDoc ? <div className="animate-fade-in h-full">{selectedDoc.path && !selectedDoc.path.endsWith('.md') ? <CodeEditor value={docContent} language={selectedDoc.path.split('.').pop()} readOnly={true} /> : <MarkdownViewer markdown={docContent} />}</div>
            : <div className="flex flex-col items-center justify-center h-full text-center gap-8 py-32 opacity-20"><div className="p-8 rounded-full border-2 border-dashed border-border/40"><Search className="w-16 h-16" /></div><div className="space-y-2"><p className="text-xl font-bold tracking-widest uppercase">System Idle</p><p className="text-sm font-medium">Initialize selection to view execution telemetry.</p></div></div>}
          </div>
        </div>
      </div>

      <EditWorkflowModal isOpen={editModalOpen} onClose={() => setEditModalOpen(false)} workflow={workflow} onSave={handleSaveWorkflow} canEditPrompt={canEditPrompt} />
      <ConfirmDialog isOpen={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)} onConfirm={handleDelete} title="Purge Workflow Lifecycle?" message={`This will permanently delete "${displayTitle}" and all associated intelligence logs. Environmental artifacts in .quorum will be decoupled.`} confirmText="Purge Execution" variant="danger" />
      <ReplanModal isOpen={isReplanModalOpen} onClose={() => setReplanModalOpen(false)} onSubmit={async (ctx) => { await replanWorkflow(workflow.id, ctx); setReplanModalOpen(false); }} loading={loading} />
      <GenerationOptionsModal isOpen={showIssuesModal} onClose={() => setShowIssuesModal(false)} onSelect={handleIssuesModeSelect} loading={issuesGenerating} />
    </div>
  );
}

const AGENT_OPTIONS = [ { value: 'claude', label: 'Claude Intelligence' }, { value: 'gemini', label: 'Gemini Neural' }, { value: 'codex', label: 'Codex Dev' } ];

function NewWorkflowForm({ onSubmit, onCancel, loading, initialData }) {
  const [title, setTitle] = useState(initialData?.name || '');
  const [prompt, setPrompt] = useState(initialData?.prompt || '');
  const [files, setFiles] = useState([]);
  const fileInputRef = useRef(null);
  const initialMode = initialData?.executionStrategy === 'single-agent' ? 'single_agent' : 'multi_agent';
  const [executionMode, setExecutionMode] = useState(initialMode);
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

  const handleSubmit = (e) => {
    e.preventDefault(); if (!prompt.trim()) return;
    const wfConfig = executionMode === 'single_agent' ? { execution_mode: 'single_agent', single_agent_name: effectiveAgent, ...(effectiveModel ? { single_agent_model: effectiveModel } : {}), ...(supportsReason && effectiveReason ? { single_agent_reasoning_effort: effectiveReason } : {}) } : undefined;
    onSubmit(prompt, files, title.trim() || undefined, wfConfig);
  };

  const handleFiles = (e) => { const s = Array.from(e.target.files || []); e.target.value = ''; if (s.length) setFiles(p => [...p, ...s]); };
  const removeFile = (i) => setFiles(p => p.filter((_, idx) => idx !== i));

  return (
    <div className="relative w-full animate-fade-in pb-20">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <div className="max-w-3xl mx-auto p-10 md:p-16 rounded-[3rem] border border-border/40 bg-card/40 backdrop-blur-2xl shadow-2xl animate-fade-up">
        <div className="flex items-center gap-5 mb-12">
           <div className="p-4 rounded-3xl bg-primary/5 border border-primary/10 text-primary/70 shadow-inner"><Plus className="w-8 h-8" /></div>
           <div><h2 className="text-3xl font-bold text-foreground tracking-tighter leading-none">Assemble Cycle</h2><p className="text-[10px] font-bold uppercase tracking-[0.3em] text-muted-foreground/40 mt-2">NEW BLUEPRINT ARCHITECTURE</p></div>
        </div>
        <form onSubmit={handleSubmit} className="space-y-12">
          <div className="space-y-8">
            <div className="space-y-3"><label className="text-[11px] font-bold uppercase tracking-widest text-muted-foreground/60 ml-1">Blueprint Identity</label><Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Define cycle name..." className="h-14 bg-background/30 rounded-2xl border-border/40 text-lg font-medium px-6" /></div>
            <div className="space-y-3"><label className="text-[11px] font-bold uppercase tracking-widest text-muted-foreground/60 ml-1">Core Logic Instruction</label><div className="relative group/prompt"><div className="absolute inset-0 bg-primary/[0.02] rounded-[2rem] opacity-0 group-hover/prompt:opacity-100 transition-opacity pointer-events-none" /><textarea value={prompt} onChange={(e) => setPrompt(e.target.value)} placeholder="Describe the neural workflow..." rows={8} className="w-full px-8 py-6 rounded-[2rem] border border-border/40 bg-background/30 text-foreground placeholder:text-muted-foreground/30 focus:outline-none focus:ring-2 focus:ring-primary/10 transition-all resize-none font-mono text-base leading-relaxed" /><VoiceInputButton onTranscript={(t) => setPrompt(p => p ? p + ' ' + t : t)} disabled={loading} className="absolute top-6 right-6" /></div></div>
          </div>
          <div className="space-y-6 pt-10 border-t border-border/30">
             <h3 className="text-[11px] font-bold uppercase tracking-widest text-muted-foreground/60 ml-1">Orchestration Strategy</h3>
             <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
              {[ { id: 'multi_agent', icon: Network, title: 'Consensus Loop', sub: 'Multi-agent iterative debate. Highest precision.' }, { id: 'single_agent', icon: Zap, title: 'Direct Pipeline', sub: 'Linear single-agent execution. Optimized latency.' } ].map(mode => (
                <button key={mode.id} type="button" onClick={() => setExecutionMode(mode.id)} className={`relative flex flex-col items-start p-6 rounded-3xl border-2 transition-all text-left duration-500 ${executionMode === mode.id ? 'border-primary/40 bg-primary/[0.03] shadow-inner' : 'border-border/40 bg-background/20 hover:border-primary/20 hover:bg-primary/[0.01]'}`}>
                  <div className="flex items-center justify-between w-full mb-4"><div className={`p-2.5 rounded-2xl border transition-colors duration-500 ${executionMode === mode.id ? 'bg-primary/10 border-primary/20 text-primary' : 'bg-muted/30 border-border/40 text-muted-foreground/40'}`}><mode.icon className="w-6 h-6" /></div>{executionMode === mode.id && <CheckCircle2 className="w-5 h-5 text-primary" />}</div>
                  <span className="font-bold text-foreground tracking-tight">{mode.title}</span>
                  <span className="text-[11px] text-muted-foreground/50 mt-1 leading-relaxed font-medium">{mode.sub}</span>
                </button>
              ))}
             </div>
            <div className={`transition-all duration-700 ease-in-out overflow-hidden ${executionMode === 'single_agent' ? 'max-h-[600px] opacity-100' : 'max-h-0 opacity-0 pointer-events-none'}`}>
              <div className="mt-6 p-8 rounded-[2rem] border border-border/30 bg-primary/[0.01] space-y-8 shadow-inner">
                <div className="flex items-center gap-3"><Settings className="w-4 h-4 text-primary/40" /><h4 className="text-[10px] font-bold uppercase tracking-[0.25em] text-muted-foreground/60">Node Configuration</h4></div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-8">
                  <div className="space-y-2"><label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40 ml-1">Provider Core</label><select value={effectiveAgent} onChange={(e) => { setSingleAgentName(e.target.value); setSingleAgentModel(''); setSingleAgentReasoningEffort(''); }} className="w-full h-12 px-4 rounded-xl border border-border/40 bg-background/50 text-sm font-bold focus:ring-2 focus:ring-primary/10 outline-none transition-all">{enabledAgents.map(a => <option key={a.value} value={a.value}>{a.label}</option>)}</select></div>
                  <div className="space-y-2"><label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40 ml-1">Intelligence Layer</label><select value={effectiveModel} onChange={(e) => setSingleAgentModel(e.target.value)} className="w-full h-12 px-4 rounded-xl border border-border/40 bg-background/50 text-sm font-bold focus:ring-2 focus:ring-primary/10 outline-none transition-all">{modelOptions.map(m => <option key={`${effectiveAgent}-${m.value || 'default'}`} value={m.value}>{m.label}</option>)}</select></div>
                  {supportsReason && <div className="sm:col-span-2 space-y-2"><label className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40 ml-1">Reasoning Intensity</label><select value={effectiveReason} onChange={(e) => setSingleAgentReasoningEffort(e.target.value)} className="w-full h-12 px-4 rounded-xl border border-border/40 bg-background/50 text-sm font-bold focus:ring-2 focus:ring-primary/10 outline-none transition-all"><option value="">Standard Balance</option>{reasoningLevels.map(r => <option key={r.value} value={r.value}>{r.label}</option>)}</select>{selectedReasoning?.description && <p className="text-[10px] text-muted-foreground/40 font-medium px-1">{selectedReasoning.description}</p>}</div>}
                </div>
              </div>
            </div>
          </div>
          <div className="pt-10 border-t border-border/30">
            <label className="text-[11px] font-bold uppercase tracking-widest text-muted-foreground/60 ml-1 mb-4 block">Knowledge Base Attachments</label>
            <div className="flex flex-wrap items-center gap-6"><input ref={fileInputRef} type="file" multiple onChange={handleFiles} className="hidden" disabled={loading} /><Button type="button" variant="outline" onClick={() => fileInputRef.current?.click()} disabled={loading} className="rounded-2xl border-dashed border-2 border-border/60 hover:border-primary/30 h-14 px-8 text-xs font-bold bg-background/20"><Upload className="w-4 h-4 mr-3" />Source Context</Button><p className="text-[10px] font-medium text-muted-foreground/40 italic max-w-[200px]">Inject codebase segments or technical documentation into the cycle memory.</p></div>
            {files.length > 0 && <div className="mt-6 flex flex-wrap gap-2.5">{files.map((f, i) => <div key={i} className="flex items-center gap-2 pl-4 pr-2 py-1.5 rounded-full bg-primary/5 text-primary/70 text-[10px] font-bold border border-primary/10 shadow-sm animate-fade-in"><span className="truncate max-w-[150px]">{f.name}</span><button type="button" onClick={() => removeFile(i)} className="p-1 hover:bg-primary/10 rounded-full transition-colors"><X className="w-3.5 h-3.5" /></button></div>)}</div>}
          </div>
          <div className="flex flex-col sm:flex-row gap-4 pt-10">
            <Button type="submit" disabled={loading || !prompt.trim()} className="flex-1 rounded-2xl h-14 text-sm font-bold uppercase tracking-[0.2em] shadow-2xl shadow-primary/10">{loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Zap className="w-5 h-5 mr-3" />}Initialize Cycle</Button>
            <Button type="button" variant="ghost" onClick={onCancel} className="rounded-2xl h-14 px-10 text-xs font-bold text-muted-foreground/40 hover:text-foreground">Cancel</Button>
          </div>
        </form>
      </div>
    </div>
  );
}

const STATUS_FLT = [ { value: 'all', label: 'All Cycles', icon: List }, { value: 'running', label: 'Active Pulse', icon: Activity }, { value: 'completed', label: 'Resolved', icon: CheckCircle2 }, { value: 'failed', label: 'Halted', icon: XCircle } ];

function StatusFilterTabs({ status, setStatus }) {
  return (
    <div className="flex items-center gap-2 p-1.5 rounded-2xl bg-card/20 border border-border/30 shadow-inner backdrop-blur-sm">
      {STATUS_FLT.map(({ value, label, icon: Icon }) => (
        <button key={value} onClick={() => setStatus(value)} className={`flex items-center gap-2.5 px-5 py-2.5 rounded-xl text-[10px] font-bold uppercase tracking-widest transition-all duration-500 ${status === value ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/10 scale-105 z-10' : 'text-muted-foreground/60 hover:bg-background/50 hover:text-foreground'}`}>
          <Icon className="w-3.5 h-3.5" /> {label}
        </button>
      ))}
    </div>
  );
}

function WorkflowFilters({ filter, setFilter }) {
  return (
    <div className="relative w-full sm:w-80 group">
      <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/40 group-focus-within:text-primary/60 transition-colors" />
      <Input placeholder="Filter execution stream..." value={filter} onChange={(e) => setFilter(e.target.value)} className="h-12 pl-12 pr-6 rounded-2xl border-border/40 bg-card/30 backdrop-blur-md shadow-sm focus-visible:ring-primary/10 transition-all" />
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
  const setStatusFilter = useCallback((s) => { if (s === 'all') searchParams.delete('status'); else searchParams.set('status', s); setSearchParams(searchParams); }, [searchParams, setSearchParams]);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [workflowToDelete, setWorkflowToDelete] = useState(null);

  useEffect(() => { fetchWorkflows(); }, [fetchWorkflows]);
  useEffect(() => { if (id && id !== 'new') fetchWorkflow(id); else if (id === 'new' || templateData) setShowNewForm(true); }, [fetchWorkflow, id, templateData]);

  const filteredWorkflows = useMemo(() => {
    let res = workflows;
    if (statusFilter && statusFilter !== 'all') res = res.filter(w => w.status === statusFilter);
    if (filter) { const f = filter.toLowerCase(); res = res.filter(w => (deriveWorkflowTitle(w).toLowerCase().includes(f)) || (w.id && w.id.toLowerCase().includes(f)) || (w.prompt && w.prompt.toLowerCase().includes(f))); }
    return res;
  }, [workflows, filter, statusFilter]);

  const fetchTasks = useCallback(async (wfId) => { try { const tList = await workflowApi.getTasks(wfId); setTasks(wfId, tList); } catch (e) { console.error(e); } }, [setTasks]);
  useEffect(() => { if (id && id !== 'new') fetchTasks(id); }, [fetchTasks, id]);

  const selectedWorkflow = workflows.find(w => w.id === id);
  const workflowTasks = id ? getTasksForWorkflow(id) : [];

  const handleCreate = async (p, f = [], t, c) => {
    const wf = await createWorkflow(p, { title: t, config: c });
    if (!wf) { notifyError(useWorkflowStore.getState().error || 'Initialization failed'); clearError(); return; }
    if (f.length > 0) { try { await workflowApi.uploadAttachments(wf.id, f); await fetchWorkflow(wf.id); notifyInfo(`Attached ${f.length} assets`); } catch (err) { notifyError(err.message || 'Asset ingestion failed'); } }
    setShowNewForm(false); navigate(`/workflows/${wf.id}`);
  };

  const handleDeleteClick = useCallback((wf) => { setWorkflowToDelete(wf); setDeleteDialogOpen(true); }, []);
  const handleDeleteConfirm = useCallback(async () => {
    if (!workflowToDelete) return;
    if (await deleteWorkflow(workflowToDelete.id)) notifyInfo('Cycle purged'); else notifyError('Purge failed');
    setWorkflowToDelete(null); setDeleteDialogOpen(false);
  }, [workflowToDelete, deleteWorkflow, notifyInfo, notifyError]);

  if (id === 'new' || showNewForm) return <NewWorkflowForm onSubmit={handleCreate} onCancel={() => { setShowNewForm(false); if (id === 'new') navigate('/workflows'); navigate(location.pathname, { replace: true, state: {} }); }} loading={loading} initialData={templateData} />;
  if (selectedWorkflow) return <WorkflowDetail workflow={selectedWorkflow} tasks={workflowTasks} onBack={() => navigate('/workflows')} />;

  return (
    <div className="relative min-h-full space-y-10 animate-fade-in pb-12">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header Workflows */}
      <div className="relative overflow-hidden rounded-[2.5rem] border border-border/40 bg-card/20 backdrop-blur-xl p-8 md:p-12 shadow-[0_8px_30px_rgb(0,0,0,0.02)]">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/[0.03] rounded-full blur-[120px] -translate-y-1/2 translate-x-1/4 pointer-events-none" />
        <div className="relative z-10 flex flex-col lg:flex-row lg:items-center justify-between gap-10">
          <div className="max-w-2xl space-y-6">
            <div className="inline-flex items-center gap-2.5 px-4 py-1.5 rounded-full bg-primary/5 border border-primary/10 text-primary text-[10px] font-bold uppercase tracking-[0.2em]"><Network className="h-3 w-3 opacity-70" /> Automation Fabric</div>
            <h1 className="text-4xl md:text-5xl font-bold text-foreground tracking-tight leading-[1.1]">Workflow <span className="text-primary/80">Blueprint Stream</span></h1>
            <p className="text-base md:text-lg text-muted-foreground font-medium leading-relaxed max-w-lg">Manage and monitor the entire lifecycle of your AI-driven execution cycles.</p>
          </div>
          <div className="flex shrink-0"><Link to="/workflows/new" className="flex items-center justify-center gap-2.5 px-8 py-3.5 rounded-2xl bg-primary text-primary-foreground text-sm font-bold hover:bg-primary/90 transition-all shadow-xl shadow-primary/10"><Plus className="w-5 h-5" /> New Cycle</Link></div>
        </div>
      </div>

      {/* Control Bar Sticky */}
      <div className="sticky top-14 z-30 flex flex-col gap-6 bg-background/80 backdrop-blur-xl py-6 border-b border-border/30 transition-all duration-500">
        <div className="flex flex-col md:flex-row gap-6 md:items-center justify-between">
          <WorkflowFilters filter={filter} setFilter={setFilter} />
          <div className="hidden sm:flex items-center gap-3 text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 bg-muted/10 px-4 py-2 rounded-xl border border-border/30 backdrop-blur-sm"><Info className="h-3.5 w-3.5" /><span>{filteredWorkflows.length} Records Active</span></div>
        </div>
        <div className="flex items-center gap-2 overflow-x-auto no-scrollbar mask-fade-right"><StatusFilterTabs status={statusFilter} setStatus={setStatusFilter} /></div>
      </div>

      {/* Grid Content */}
      {loading && workflows.length === 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-8">
          {[...Array(6)].map((_, i) => <div key={i} className="h-56 rounded-3xl bg-muted/5 animate-pulse border border-border/20" />)}
        </div>
      ) : filteredWorkflows.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-8">
          {filteredWorkflows.map((w) => <WorkflowCard key={w.id} workflow={w} onClick={() => navigate(`/workflows/${w.id}`)} onDelete={handleDeleteClick} />)}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-32 text-center space-y-8 rounded-[3rem] border border-dashed border-border/30 bg-muted/[0.02] animate-fade-in">
          <div className="p-8 rounded-[2rem] bg-muted/10 border border-border/20 text-muted-foreground/20"><GitBranch className="w-16 h-16" /></div>
          <div className="space-y-3"><h3 className="text-2xl font-bold text-foreground/80 tracking-tight">Stream Vacant</h3><p className="text-muted-foreground/40 max-w-sm mx-auto font-medium leading-relaxed">{filter ? 'No blueprints match current telemetry parameters.' : 'Initialize your first autonomous cycle to begin building your automation history.'}</p></div>
          {!filter ? <Link to="/workflows/new" className="inline-flex items-center gap-3 px-10 py-4 rounded-[2rem] bg-primary text-primary-foreground text-sm font-bold uppercase tracking-widest hover:bg-primary/90 transition-all shadow-2xl shadow-primary/10"><Plus className="w-5 h-5" /> Assemble Cycle</Link>
          : <Button variant="outline" onClick={() => { setFilter(''); setStatusFilter('all'); }} className="rounded-2xl font-bold px-8 h-12 border-border/60">Reset Environment</Button>}
        </div>
      )}
      
      <FAB onClick={() => navigate('/workflows/new')} icon={Plus} label="New Workflow" />
      <ConfirmDialog isOpen={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)} onConfirm={handleDeleteConfirm} title="Purge Cycle Lifecycle?" message={workflowToDelete ? `Confirming the permanent erasure of "${deriveWorkflowTitle(workflowToDelete)}". Historical logs and telemetry will be decoupled from the core UI.` : ''} confirmText="Confirm Purge" variant="danger" />
    </div>
  );
}
