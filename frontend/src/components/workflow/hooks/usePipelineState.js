import { useMemo, useState } from 'react';
import { useExecutionStore } from '../../../stores';

const PHASE_ORDER = { refine: -1, analyze: 0, plan: 1, execute: 2 };

function derivePhaseStatus(phaseId, currentPhase, workflowStatus, enabled) {
  if (enabled === false) return 'disabled';

  const thisOrder = PHASE_ORDER[phaseId];
  const currentOrder = PHASE_ORDER[currentPhase] ?? -2;

  if (workflowStatus === 'failed' && currentPhase === phaseId) return 'failed';
  if (workflowStatus === 'running' && currentPhase === phaseId) return 'running';
  if (currentOrder > thisOrder) return 'completed';
  if (workflowStatus === 'completed' && (!currentPhase || currentPhase === '' || currentPhase === 'done')) return 'completed';
  if (workflowStatus === 'completed' && currentPhase === phaseId) return 'completed';
  return 'pending';
}

function extractRoundsFromTimeline(timeline) {
  if (!timeline || timeline.length === 0) return { rounds: [], currentRound: 0, lastScore: null, singleAgent: null };

  const chronological = [...timeline]
    .filter((e) => e.kind === 'agent')
    .sort((a, b) => new Date(a.ts).getTime() - new Date(b.ts).getTime());

  const roundsMap = {};
  let currentRound = 0;
  let lastScore = null;
  let singleAgent = null;

  for (const e of chronological) {
    const phase = e.phase || e.data?.phase || '';
    const round = e.data?.round;

    // Single-agent mode detection
    if (phase === 'analyze_single') {
      const agent = e.agent || e.data?.agent || '';
      const model = e.data?.model || '';
      if (!singleAgent) {
        singleAgent = { agent, model, status: 'pending' };
      }
      if (e.event === 'started') {
        singleAgent = { agent: agent || singleAgent.agent, model: model || singleAgent.model, status: 'running' };
      }
      if (e.event === 'completed') {
        singleAgent = { agent: agent || singleAgent.agent, model: model || singleAgent.model, status: 'completed' };
      }
      continue;
    }

    // Moderator completed events carry round + consensus_score
    if (phase === 'moderator' && e.event === 'completed' && round != null) {
      currentRound = round;
      lastScore = e.data?.consensus_score ?? lastScore;
      if (!roundsMap[round]) roundsMap[round] = { round, agents: [], score: null, status: 'completed' };
      roundsMap[round].score = e.data?.consensus_score ?? null;
      roundsMap[round].status = 'completed';
    }

    // Analyze version agents (analyze_v1, analyze_v2, etc.)
    const vMatch = phase.match?.(/^analyze_v(\d+)$/);
    if (vMatch) {
      const r = parseInt(vMatch[1], 10);
      if (!roundsMap[r]) roundsMap[r] = { round: r, agents: [], score: null, status: 'pending' };
      const agentName = e.agent || '';
      if (agentName && !roundsMap[r].agents.includes(agentName)) {
        roundsMap[r].agents.push(agentName);
      }
      if (e.event === 'started' && roundsMap[r].status === 'pending') {
        roundsMap[r].status = 'running';
      }
      if (e.event === 'completed') {
        // Only mark completed if moderator already ran for this round
        // otherwise keep running
      }
    }

    // Moderator running
    if (phase === 'moderator' && e.event === 'started' && round != null) {
      if (!roundsMap[round]) roundsMap[round] = { round, agents: [], score: null, status: 'running' };
    }
  }

  const rounds = Object.values(roundsMap).sort((a, b) => a.round - b.round);
  return { rounds, currentRound, lastScore, singleAgent };
}

function extractSynthesisStatus(timeline) {
  if (!timeline || timeline.length === 0) return 'pending';
  const synthEvents = timeline.filter(
    (e) => e.kind === 'agent' && (e.phase === 'synthesize' || e.data?.phase === 'synthesize')
  );
  if (synthEvents.length === 0) return 'pending';
  const hasCompleted = synthEvents.some((e) => e.event === 'completed');
  if (hasCompleted) return 'completed';
  const hasStarted = synthEvents.some((e) => e.event === 'started');
  if (hasStarted) return 'running';
  return 'pending';
}

export default function usePipelineState(workflow, workflowKey) {
  const [expandedPhase, setExpandedPhase] = useState(null);
  const timeline = useExecutionStore((s) => s.timelineByWorkflow[workflowKey] || []);

  const phases = useMemo(() => {
    if (!workflow) {
      return {
        refine: { status: 'pending', enabled: false, agentName: '' },
        analyze: {
          status: 'pending', currentRound: 0, maxRounds: 0, minRounds: 0,
          consensusScore: null, threshold: 0, warningThreshold: 0, stagnationThreshold: 0,
          consensusEnabled: false, moderatorAgent: '', rounds: [], synthesisStatus: 'pending',
        },
        plan: { status: 'pending', multiPlanEnabled: false, planSynthAgent: '' },
        execute: { status: 'pending', taskCount: 0 },
      };
    }

    const bp = workflow.blueprint || {};
    const consensus = bp.consensus || {};
    const refiner = bp.refiner || {};
    const planSynth = bp.plan_synthesizer || {};
    const currentPhase = workflow.current_phase || '';
    const wfStatus = workflow.status || 'pending';
    const metrics = workflow.metrics || {};

    // Refine
    const refineEnabled = refiner.enabled === true;
    const refineStatus = derivePhaseStatus('refine', currentPhase, wfStatus, refineEnabled);

    // Analyze
    const analyzeStatus = derivePhaseStatus('analyze', currentPhase, wfStatus, true);
    const { rounds, currentRound, lastScore, singleAgent } = extractRoundsFromTimeline(timeline);
    const consensusScore = lastScore ?? metrics.consensus_score ?? null;
    const synthesisStatus = extractSynthesisStatus(timeline);

    // Plan
    const planStatus = derivePhaseStatus('plan', currentPhase, wfStatus, true);
    const multiPlanEnabled = planSynth.enabled === true;

    // Execute
    const executeStatus = derivePhaseStatus('execute', currentPhase, wfStatus, true);
    const taskCount = workflow.tasks?.length || metrics.task_count || 0;

    return {
      refine: {
        status: refineStatus,
        enabled: refineEnabled,
        agentName: refiner.agent || '',
      },
      analyze: {
        status: analyzeStatus,
        currentRound,
        maxRounds: consensus.max_rounds || 0,
        minRounds: consensus.min_rounds || 0,
        consensusScore,
        threshold: consensus.threshold || bp.consensus_threshold || 0,
        warningThreshold: consensus.warning_threshold || 0,
        stagnationThreshold: consensus.stagnation_threshold || 0,
        consensusEnabled: consensus.enabled === true,
        moderatorAgent: consensus.agent || '',
        rounds,
        synthesisStatus,
        singleAgent,
      },
      plan: {
        status: planStatus,
        multiPlanEnabled,
        planSynthAgent: planSynth.agent || '',
      },
      execute: {
        status: executeStatus,
        taskCount,
      },
    };
  }, [workflow, timeline]);

  return { phases, expandedPhase, setExpandedPhase };
}
