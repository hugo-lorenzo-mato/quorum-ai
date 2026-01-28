import { useConfigStore } from '../../../stores/configStore';
import { AnalyzePhaseCard } from '../AnalyzePhaseCard';
import { SettingSection, TextInputSetting } from '../index';
import {
  Sparkles,
  MessageSquare,
  ListTodo,
  Play,
  ChevronRight,
  Users,
  GitMerge,
  CheckSquare,
} from 'lucide-react';

/**
 * Connector arrow between pipeline stages
 */
function PipelineConnector({ className = '' }) {
  return (
    <div className={`flex items-center justify-center px-1 ${className}`}>
      <ChevronRight className="w-5 h-5 text-muted-foreground" />
    </div>
  );
}

/**
 * Individual stage node in the pipeline
 */
function StageNode({ icon: Icon, name, color, children, className = '' }) {
  const colorClasses = {
    purple: 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20',
    blue: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20',
    amber: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20',
    green: 'bg-green-500/10 text-green-600 dark:text-green-400 border-green-500/20',
  };

  return (
    <div className={`flex flex-col items-center ${className}`}>
      <div className={`px-3 py-2 rounded-lg border ${colorClasses[color]} min-w-[80px] text-center`}>
        <div className="flex items-center justify-center gap-1.5">
          <Icon className="w-4 h-4" />
          <span className="text-sm font-medium">{name}</span>
        </div>
        {children}
      </div>
    </div>
  );
}

/**
 * Visual representation of the workflow pipeline
 */
function WorkflowPipelineDiagram() {
  return (
    <div className="p-4 rounded-xl border border-border bg-muted/30 overflow-x-auto">
      {/* Main pipeline flow */}
      <div className="flex items-center justify-center gap-1 min-w-fit">

        {/* REFINE Stage */}
        <StageNode icon={Sparkles} name="Refine" color="purple">
          <p className="text-[10px] text-muted-foreground mt-1">Prompt polish</p>
        </StageNode>

        <PipelineConnector />

        {/* ANALYZE Stage - Expanded with internal flow */}
        <div className="flex flex-col items-center">
          <div className="px-4 py-3 rounded-lg border bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20">
            <div className="flex items-center justify-center gap-1.5 mb-2">
              <MessageSquare className="w-4 h-4" />
              <span className="text-sm font-medium">Analyze</span>
            </div>

            {/* Thesis-Antithesis-Synthesis cycle */}
            <div className="flex items-center justify-center gap-1 text-[10px] mb-2">
              <span className="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-700 dark:text-blue-300">
                Thesis
              </span>
              <span className="text-muted-foreground">⇄</span>
              <span className="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-700 dark:text-blue-300">
                Antithesis
              </span>
              <span className="text-muted-foreground">⇄</span>
              <span className="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-700 dark:text-blue-300">
                Synthesis
              </span>
            </div>

            {/* Moderator */}
            <div className="flex items-center justify-center gap-1 text-[10px] text-muted-foreground">
              <Users className="w-3 h-3" />
              <span>Moderator (quorum)</span>
            </div>
          </div>
        </div>

        <PipelineConnector />

        {/* PLAN Stage */}
        <div className="flex flex-col items-center">
          <div className="px-4 py-3 rounded-lg border bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20">
            <div className="flex items-center justify-center gap-1.5 mb-2">
              <ListTodo className="w-4 h-4" />
              <span className="text-sm font-medium">Plan</span>
            </div>

            {/* Multiple plans consolidation */}
            <div className="flex flex-col items-center gap-1 text-[10px]">
              <div className="flex items-center gap-1">
                <span className="px-1.5 py-0.5 rounded bg-amber-500/20">Plan 1</span>
                <span className="px-1.5 py-0.5 rounded bg-amber-500/20">Plan 2</span>
                <span className="text-muted-foreground">...</span>
              </div>
              <div className="flex items-center gap-1 text-muted-foreground">
                <GitMerge className="w-3 h-3" />
                <span>Consolidate</span>
              </div>
            </div>
          </div>
        </div>

        <PipelineConnector />

        {/* EXECUTE Stage */}
        <div className="flex flex-col items-center">
          <div className="px-4 py-3 rounded-lg border bg-green-500/10 text-green-600 dark:text-green-400 border-green-500/20">
            <div className="flex items-center justify-center gap-1.5 mb-2">
              <Play className="w-4 h-4" />
              <span className="text-sm font-medium">Execute</span>
            </div>

            {/* Tasks */}
            <div className="flex flex-col items-center gap-0.5 text-[10px]">
              <div className="flex items-center gap-1 text-muted-foreground">
                <CheckSquare className="w-3 h-3" />
                <span>Task 1</span>
              </div>
              <div className="flex items-center gap-1 text-muted-foreground">
                <CheckSquare className="w-3 h-3" />
                <span>Task 2</span>
              </div>
              <span className="text-muted-foreground">...</span>
              <div className="flex items-center gap-1 text-muted-foreground">
                <CheckSquare className="w-3 h-3" />
                <span>Task N</span>
              </div>
            </div>
          </div>
        </div>

      </div>
    </div>
  );
}

export function PhasesTab() {
  const config = useConfigStore((state) => state.config);
  const setField = useConfigStore((state) => state.setField);

  const getFieldValue = (path) => {
    if (!config) return undefined;
    return path.split('.').reduce((obj, key) => obj?.[key], config);
  };

  return (
    <div className="space-y-6">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-foreground">Workflow Phases</h2>
        <p className="text-sm text-muted-foreground">
          Configure how each phase of the workflow is executed.
          <span className="block mt-1 text-xs">
            Note: phase-specific model and reasoning settings live under Settings → Agents (Per-phase overrides).
          </span>
        </p>
      </div>

      {/* Workflow Pipeline Diagram */}
      <WorkflowPipelineDiagram />

      {/* Analyze Phase - Full configuration */}
      <AnalyzePhaseCard />

      {/* Plan Phase - Simple timeout only */}
      <SettingSection
        title="Plan Phase"
        description="Task breakdown into executable steps"
      >
        <TextInputSetting
          label="Timeout"
          tooltip="Maximum time allowed for the plan phase"
          value={getFieldValue('phases.plan.timeout') || '1h'}
          onChange={(val) => setField('phases.plan.timeout', val)}
          placeholder="1h"
        />
      </SettingSection>

      {/* Execute Phase - Simple timeout only */}
      <SettingSection
        title="Execute Phase"
        description="Implementation of planned tasks"
      >
        <TextInputSetting
          label="Timeout"
          tooltip="Maximum time allowed for the execute phase"
          value={getFieldValue('phases.execute.timeout') || '2h'}
          onChange={(val) => setField('phases.execute.timeout', val)}
          placeholder="2h"
        />
      </SettingSection>
    </div>
  );
}

export default PhasesTab;
