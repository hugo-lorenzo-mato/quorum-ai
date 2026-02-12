import { useMemo } from 'react';

// Agent-specific colors for SVG (hex values matching Tailwind classes in AgentActivity)
const AGENT_SVG_COLORS = {
  claude:   { fill: '#f97316', glow: '#fb923c' },  // orange-500
  gemini:   { fill: '#3b82f6', glow: '#60a5fa' },  // blue-500
  codex:    { fill: '#22c55e', glow: '#4ade80' },  // green-500
  copilot:  { fill: '#a855f7', glow: '#c084fc' },  // purple-500
  opencode: { fill: '#06b6d4', glow: '#22d3ee' },  // cyan-500
};

const DEFAULT_COLOR = { fill: '#94a3b8', glow: '#cbd5e1' }; // slate-400

function getColor(name) {
  const key = (name || '').toLowerCase();
  for (const [agent, color] of Object.entries(AGENT_SVG_COLORS)) {
    if (key.includes(agent)) return color;
  }
  return DEFAULT_COLOR;
}

function isActive(status) {
  return ['started', 'thinking', 'tool_use', 'progress'].includes(status);
}

/**
 * AgentConstellation – SVG visualization of agents arranged in a radial pattern
 * with connection lines showing consensus interactions.
 *
 * Props:
 *  - agents: Array of { name, status, message? } — agents participating
 *  - moderator: string|null — name of the moderator agent (rendered at center)
 *  - consensusScore: number|null — current consensus score (0-1)
 *  - round: number — current consensus round
 *  - width/height: dimensions (default 280x220)
 */
export default function AgentConstellation({
  agents = [],
  moderator = null,
  consensusScore = null,
  round = 0,
  width = 280,
  height = 220,
}) {
  const cx = width / 2;
  const cy = height / 2;
  const radius = Math.min(cx, cy) - 36;
  const nodeRadius = 18;

  // Separate moderator from participant agents
  const { modAgent, participants } = useMemo(() => {
    const mod = moderator ? agents.find(a => a.name === moderator) : null;
    const parts = moderator
      ? agents.filter(a => a.name !== moderator)
      : agents;
    return { modAgent: mod, participants: parts };
  }, [agents, moderator]);

  // Position participants around the circle
  const positioned = useMemo(() => {
    const count = participants.length;
    if (count === 0) return [];
    const angleStep = (2 * Math.PI) / count;
    const startAngle = -Math.PI / 2; // start from top
    return participants.map((agent, i) => {
      const angle = startAngle + i * angleStep;
      return {
        ...agent,
        x: cx + radius * Math.cos(angle),
        y: cy + radius * Math.sin(angle),
      };
    });
  }, [participants, cx, cy, radius]);

  if (agents.length === 0) return null;

  // Unique gradient IDs scoped to avoid conflicts
  const gradId = 'constellation-link';
  const glowId = 'constellation-glow';

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      aria-label="Agent constellation visualization"
      className="select-none"
    >
      <defs>
        {/* Link gradient */}
        <linearGradient id={gradId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#6366f1" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#06b6d4" stopOpacity="0.3" />
        </linearGradient>
        {/* Glow filter for active nodes */}
        <filter id={glowId} x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Connection lines: each participant to center (moderator or hub) */}
      {positioned.map((agent) => (
        <line
          key={`link-${agent.name}`}
          x1={agent.x}
          y1={agent.y}
          x2={cx}
          y2={cy}
          stroke={`url(#${gradId})`}
          strokeWidth="1.5"
          strokeDasharray={isActive(agent.status) ? '6 4' : 'none'}
          className={isActive(agent.status) ? 'animate-dash-flow' : ''}
          strokeOpacity={agent.status === 'completed' ? 0.6 : 0.4}
        />
      ))}

      {/* Inter-agent connections (between adjacent participants) */}
      {positioned.length > 1 && positioned.map((agent, i) => {
        const next = positioned[(i + 1) % positioned.length];
        return (
          <line
            key={`peer-${agent.name}-${next.name}`}
            x1={agent.x}
            y1={agent.y}
            x2={next.x}
            y2={next.y}
            stroke={`url(#${gradId})`}
            strokeWidth="1"
            strokeOpacity={0.15}
          />
        );
      })}

      {/* Center node: moderator or consensus hub */}
      <CenterNode
        cx={cx}
        cy={cy}
        r={nodeRadius + 2}
        agent={modAgent}
        consensusScore={consensusScore}
        round={round}
        glowId={glowId}
      />

      {/* Participant nodes */}
      {positioned.map((agent) => (
        <AgentNode
          key={agent.name}
          x={agent.x}
          y={agent.y}
          r={nodeRadius}
          agent={agent}
          glowId={glowId}
        />
      ))}
    </svg>
  );
}

function AgentNode({ x, y, r, agent, glowId }) {
  const color = getColor(agent.name);
  const active = isActive(agent.status);
  const done = agent.status === 'completed';
  const error = agent.status === 'error';

  return (
    <g filter={active ? `url(#${glowId})` : undefined}>
      {/* Background circle */}
      <circle
        cx={x}
        cy={y}
        r={r}
        fill="var(--card)"
        stroke={error ? '#ef4444' : done ? '#22c55e' : color.fill}
        strokeWidth={active ? 2.5 : 1.5}
        strokeOpacity={active ? 1 : 0.7}
      >
        {active && (
          <animate
            attributeName="stroke-opacity"
            values="1;0.4;1"
            dur="2s"
            repeatCount="indefinite"
          />
        )}
      </circle>

      {/* Agent initial */}
      <text
        x={x}
        y={y - 2}
        textAnchor="middle"
        dominantBaseline="central"
        className="text-[11px] font-bold"
        fill={error ? '#ef4444' : done ? '#22c55e' : color.fill}
      >
        {agent.name.charAt(0).toUpperCase()}
      </text>

      {/* Agent name label */}
      <text
        x={x}
        y={y + r + 12}
        textAnchor="middle"
        className="text-[9px]"
        fill="var(--muted-fg, #94a3b8)"
      >
        {agent.name}
      </text>

      {/* Status dot */}
      <circle
        cx={x + r * 0.65}
        cy={y - r * 0.65}
        r={3.5}
        fill={error ? '#ef4444' : done ? '#22c55e' : active ? '#f59e0b' : '#64748b'}
        stroke="var(--card)"
        strokeWidth="1.5"
      >
        {active && (
          <animate
            attributeName="r"
            values="3.5;5;3.5"
            dur="1.5s"
            repeatCount="indefinite"
          />
        )}
      </circle>
    </g>
  );
}

function CenterNode({ cx, cy, r, agent, consensusScore, round, glowId }) {
  const hasModerator = !!agent;
  const color = hasModerator ? getColor(agent.name) : { fill: '#8b5cf6', glow: '#a78bfa' };
  const active = hasModerator && isActive(agent.status);
  const done = hasModerator && agent.status === 'completed';

  // Score display
  const scoreText = consensusScore != null ? `${Math.round(consensusScore * 100)}%` : '';
  const roundText = round > 0 ? `R${round}` : '';

  return (
    <g filter={active ? `url(#${glowId})` : undefined}>
      {/* Outer ring */}
      <circle
        cx={cx}
        cy={cy}
        r={r + 4}
        fill="none"
        stroke={color.fill}
        strokeWidth="1"
        strokeOpacity={0.2}
        strokeDasharray="4 3"
      >
        {active && (
          <animateTransform
            attributeName="transform"
            type="rotate"
            from={`0 ${cx} ${cy}`}
            to={`360 ${cx} ${cy}`}
            dur="8s"
            repeatCount="indefinite"
          />
        )}
      </circle>

      {/* Main circle */}
      <circle
        cx={cx}
        cy={cy}
        r={r}
        fill="var(--card)"
        stroke={done ? '#22c55e' : color.fill}
        strokeWidth={active ? 2.5 : 2}
      >
        {active && (
          <animate
            attributeName="stroke-opacity"
            values="1;0.5;1"
            dur="2s"
            repeatCount="indefinite"
          />
        )}
      </circle>

      {/* Center label */}
      {hasModerator ? (
        <>
          <text
            x={cx}
            y={cy - (scoreText ? 4 : 0)}
            textAnchor="middle"
            dominantBaseline="central"
            className="text-[10px] font-bold"
            fill={color.fill}
          >
            Mod
          </text>
          {scoreText && (
            <text
              x={cx}
              y={cy + 8}
              textAnchor="middle"
              dominantBaseline="central"
              className="text-[8px] font-medium"
              fill="var(--muted-fg, #94a3b8)"
            >
              {scoreText}
            </text>
          )}
        </>
      ) : (
        <>
          <text
            x={cx}
            y={cy - (roundText ? 4 : 0)}
            textAnchor="middle"
            dominantBaseline="central"
            className="text-[10px] font-semibold"
            fill={color.fill}
          >
            {roundText || '?'}
          </text>
          {scoreText && (
            <text
              x={cx}
              y={cy + 8}
              textAnchor="middle"
              dominantBaseline="central"
              className="text-[8px] font-medium"
              fill="var(--muted-fg, #94a3b8)"
            >
              {scoreText}
            </text>
          )}
        </>
      )}

      {/* Label below */}
      <text
        x={cx}
        y={cy + r + 14}
        textAnchor="middle"
        className="text-[9px]"
        fill="var(--muted-fg, #94a3b8)"
      >
        {hasModerator ? agent.name : 'consensus'}
      </text>
    </g>
  );
}
