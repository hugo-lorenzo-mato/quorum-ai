import { useState, useEffect, useId } from 'react';
import {
  Search,
  GitBranch,
  Play,
  RefreshCw,
  CheckCircle2,
  Circle,
  Loader2,
} from 'lucide-react';

const PHASES = [
  { label: 'Analyze', Icon: Search, desc: 'Analyzing requirements and codebase' },
  { label: 'Plan', Icon: GitBranch, desc: 'Creating DAG and task assignments' },
  { label: 'Execute', Icon: Play, desc: 'Running tasks across agents in parallel' },
  { label: 'Refine', Icon: RefreshCw, desc: 'Quality checks and artifact reconciliation' },
  { label: 'Complete', Icon: CheckCircle2, desc: 'Workflow finished' },
];

const STATUS_CONFIG = {
  completed: {
    StatusIcon: CheckCircle2,
    classes:
      'bg-gradient-to-r from-status-success-bg to-status-success-bg/50 text-status-success ring-1 ring-status-success/20',
  },
  running: {
    StatusIcon: Loader2,
    classes:
      'bg-gradient-to-r from-status-running-bg to-status-running-bg/50 text-status-running ring-1 ring-status-running/20 shadow-sm shadow-status-running/10 animate-pulse-glow',
    iconClass: 'animate-spin',
  },
  pending: {
    StatusIcon: Circle,
    classes: 'bg-muted/50 text-muted-foreground ring-1 ring-border/30',
  },
};

function getPhaseStatus(phaseIndex, activePhase) {
  if (activePhase < 0) return 'pending';
  if (phaseIndex < activePhase) return 'completed';
  if (phaseIndex === activePhase) return 'running';
  return 'pending';
}

function PhaseChipDemo({ label, status, icon: PhaseIcon, reducedMotion }) {
  const config = STATUS_CONFIG[status];
  const { StatusIcon } = config;

  return (
    <span
      className={`
        inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium
        transition-all duration-300 select-none
        ${config.classes}
        ${reducedMotion ? '[animation:none]' : ''}
      `}
    >
      <PhaseIcon className="w-3 h-3" />
      <StatusIcon
        className={`w-3 h-3 ${!reducedMotion && config.iconClass ? config.iconClass : ''}`}
      />
      {label}
    </span>
  );
}

function ConnectorDemo({ completed, nextRunning, gradientId, reducedMotion }) {
  return (
    <div className="flex items-center px-0.5 shrink-0">
      <svg
        width="24"
        height="12"
        viewBox="0 0 24 12"
        aria-hidden="true"
        className="shrink-0"
      >
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="var(--status-success)" stopOpacity="0.6" />
            <stop offset="100%" stopColor="var(--status-success)" />
          </linearGradient>
        </defs>
        <line
          x1="0"
          y1="6"
          x2="18"
          y2="6"
          stroke={completed ? `url(#${gradientId})` : 'var(--muted-fg)'}
          strokeWidth="1.5"
          strokeOpacity={completed ? 1 : 0.3}
          strokeDasharray={nextRunning ? '4 3' : 'none'}
          className={nextRunning && !reducedMotion ? 'animate-dash-flow' : ''}
        />
        <polygon
          points="17,3 22,6 17,9"
          fill={completed ? 'var(--status-success)' : 'var(--muted-fg)'}
          fillOpacity={completed ? 1 : 0.3}
        />
      </svg>
    </div>
  );
}

export default function PipelineDemo() {
  // -1 = all pending, 0..4 = that phase is running, 5 = all completed (pause)
  const [activePhase, setActivePhase] = useState(-1);
  const [reducedMotion, setReducedMotion] = useState(false);
  const uid = useId();

  useEffect(() => {
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    setReducedMotion(mq.matches);
    const handler = (e) => setReducedMotion(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  useEffect(() => {
    if (reducedMotion) return;

    const delay = activePhase >= PHASES.length ? 2500 : 1500;
    const timer = setTimeout(() => {
      setActivePhase((prev) => {
        if (prev >= PHASES.length) return -1; // reset
        return prev + 1;
      });
    }, delay);
    return () => clearTimeout(timer);
  }, [activePhase, reducedMotion]);

  return (
    <section className="w-full py-16 px-4">
      <div className="max-w-3xl mx-auto text-center mb-10">
        <h2 className="text-2xl font-semibold tracking-tight text-foreground">
          Automated Pipeline Execution
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Every workflow flows through five phases — fully automated, from analysis to completion.
        </p>
      </div>

      <div className="flex flex-wrap items-center justify-center gap-y-3">
        {PHASES.map((phase, i) => {
          const status = getPhaseStatus(i, activePhase);
          const isLast = i === PHASES.length - 1;

          return (
            <div key={phase.label} className="flex items-center">
              <PhaseChipDemo
                label={phase.label}
                status={status}
                icon={phase.Icon}
                reducedMotion={reducedMotion}
              />
              {!isLast && (
                <ConnectorDemo
                  completed={status === 'completed'}
                  nextRunning={
                    status === 'completed' &&
                    getPhaseStatus(i + 1, activePhase) === 'running'
                  }
                  gradientId={`${uid}-conn-${i}`}
                  reducedMotion={reducedMotion}
                />
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}
