import React from 'react';
import { Link } from 'react-router-dom';
import { Zap, FileText } from 'lucide-react';
import Logo from './Logo';

// Animated SVG flow diagram showing the Quorum AI multi-agent workflow pipeline
function FlowDiagram() {
  const gradId = React.useId();

  // Node definitions for the workflow pipeline
  const nodes = [
    { id: 'prompt',    label: 'Prompt',    x: 60,  y: 70, icon: '💬' },
    { id: 'analyze',   label: 'Analyze',   x: 200, y: 70, icon: '🔍' },
    { id: 'plan',      label: 'Plan',      x: 340, y: 70, icon: '📋' },
    { id: 'execute',   label: 'Execute',   x: 480, y: 70, icon: '⚡' },
    { id: 'review',    label: 'Review',    x: 620, y: 70, icon: '✅' },
    { id: 'consensus', label: 'Consensus', x: 760, y: 70, icon: '🤝' },
  ];

  // Edges between consecutive nodes
  const edges = [];
  for (let i = 0; i < nodes.length - 1; i++) {
    edges.push({ from: nodes[i], to: nodes[i + 1], id: `e${i}` });
  }

  const nodeW = 90;
  const nodeH = 50;
  const nodeR = 12;

  return (
    <svg
      viewBox="0 0 840 140"
      className="w-full h-auto max-w-3xl"
      aria-label="Quorum AI workflow: Prompt → Analyze → Plan → Execute → Review → Consensus"
      role="img"
    >
      <defs>
        <linearGradient id={`${gradId}-edge`} x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#6366f1" />
          <stop offset="50%" stopColor="#8b5cf6" />
          <stop offset="100%" stopColor="#06b6d4" />
        </linearGradient>
        <linearGradient id={`${gradId}-node`} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#6366f1" stopOpacity="0.15" />
          <stop offset="100%" stopColor="#06b6d4" stopOpacity="0.08" />
        </linearGradient>
        {/* Glow filter for active-feeling nodes */}
        <filter id={`${gradId}-glow`} x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="3" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      {/* Animated edges */}
      {edges.map((edge, i) => {
        const x1 = edge.from.x + nodeW / 2;
        const x2 = edge.to.x - nodeW / 2;
        const y = edge.from.y;
        return (
          <g key={edge.id}>
            {/* Background track */}
            <line
              x1={x1} y1={y} x2={x2} y2={y}
              stroke="currentColor"
              strokeOpacity="0.08"
              strokeWidth="2"
            />
            {/* Animated flowing line */}
            <line
              x1={x1} y1={y} x2={x2} y2={y}
              stroke={`url(#${gradId}-edge)`}
              strokeWidth="2"
              strokeDasharray="8 6"
              strokeLinecap="round"
            >
              <animate
                attributeName="stroke-dashoffset"
                from="28"
                to="0"
                dur={`${1.2 + i * 0.15}s`}
                repeatCount="indefinite"
              />
            </line>
            {/* Arrow head */}
            <polygon
              points={`${x2 - 6},${y - 4} ${x2},${y} ${x2 - 6},${y + 4}`}
              fill={`url(#${gradId}-edge)`}
              opacity="0.7"
            />
          </g>
        );
      })}

      {/* Feedback loop arrow (Review → Analyze) */}
      <path
        d={`M ${nodes[4].x + nodeW / 2} ${nodes[4].y + nodeH / 2 + 2}
            Q ${(nodes[4].x + nodes[1].x) / 2 + 40} ${nodes[4].y + nodeH / 2 + 36}
              ${nodes[1].x + nodeW / 2} ${nodes[1].y + nodeH / 2 + 2}`}
        fill="none"
        stroke={`url(#${gradId}-edge)`}
        strokeWidth="1.5"
        strokeDasharray="6 4"
        strokeOpacity="0.4"
        markerEnd=""
      >
        <animate
          attributeName="stroke-dashoffset"
          from="0"
          to="-20"
          dur="2s"
          repeatCount="indefinite"
        />
      </path>
      <text
        x={(nodes[4].x + nodes[1].x) / 2 + 70}
        y={nodes[4].y + nodeH / 2 + 34}
        textAnchor="middle"
        className="fill-muted-foreground"
        fontSize="9"
        fontFamily="Inter Variable, sans-serif"
        opacity="0.5"
      >
        iterate until consensus
      </text>

      {/* Nodes */}
      {nodes.map((node, i) => {
        const nx = node.x - nodeW / 2;
        const ny = node.y - nodeH / 2;
        return (
          <g key={node.id}>
            {/* Node background */}
            <rect
              x={nx}
              y={ny}
              width={nodeW}
              height={nodeH}
              rx={nodeR}
              fill={`url(#${gradId}-node)`}
              stroke="currentColor"
              strokeOpacity="0.12"
              strokeWidth="1"
            />
            {/* Subtle animated border highlight that cascades across nodes */}
            <rect
              x={nx}
              y={ny}
              width={nodeW}
              height={nodeH}
              rx={nodeR}
              fill="none"
              stroke={`url(#${gradId}-edge)`}
              strokeWidth="1.5"
              strokeOpacity="0"
            >
              <animate
                attributeName="stroke-opacity"
                values="0;0.6;0"
                dur="3s"
                begin={`${i * 0.5}s`}
                repeatCount="indefinite"
              />
            </rect>
            {/* Icon */}
            <text
              x={node.x}
              y={ny + 19}
              textAnchor="middle"
              fontSize="14"
              dominantBaseline="central"
            >
              {node.icon}
            </text>
            {/* Label */}
            <text
              x={node.x}
              y={ny + 38}
              textAnchor="middle"
              fontSize="11"
              fontWeight="600"
              fontFamily="Inter Variable, sans-serif"
              className="fill-foreground"
            >
              {node.label}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

export default function HeroSection() {
  return (
    <div className="relative overflow-hidden rounded-2xl border border-border bg-gradient-to-br from-card via-card to-primary/[0.03] mb-6 animate-fade-up">
      {/* Background dot pattern */}
      <div className="absolute inset-0 bg-dot-pattern opacity-30 pointer-events-none" />

      <div className="relative px-6 py-8 md:px-10 md:py-10">
        {/* Top section: branding + tagline */}
        <div className="flex flex-col items-center text-center mb-8">
          <div className="relative mb-4">
            <div className="absolute inset-0 bg-primary/20 blur-2xl rounded-full scale-150" />
            <div className="relative p-4 rounded-2xl bg-card/80 border border-border/60 shadow-sm backdrop-blur-sm">
              <Logo className="w-12 h-12" />
            </div>
          </div>
          <h2 className="text-xl md:text-2xl font-bold text-foreground tracking-tight">
            Multi-Agent Consensus Engine
          </h2>
          <p className="text-sm text-muted-foreground mt-2 max-w-md leading-relaxed">
            Orchestrate AI agents through iterative refinement rounds until consensus is reached — reducing hallucinations and increasing reliability.
          </p>
        </div>

        {/* Animated flow diagram */}
        <div className="flex justify-center overflow-x-auto scrollbar-none -mx-6 px-6 md:mx-0 md:px-0">
          <FlowDiagram />
        </div>

        {/* CTA buttons */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-3 mt-8">
          <Link
            to="/workflows/new"
            className="w-full sm:w-auto px-5 py-2.5 rounded-xl bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-all flex items-center justify-center gap-2 shadow-lg shadow-primary/20"
          >
            <Zap className="w-4 h-4" />
            Start Workflow
          </Link>
          <Link
            to="/prompts"
            className="w-full sm:w-auto px-5 py-2.5 rounded-xl border border-border text-foreground text-sm font-medium hover:bg-accent transition-all flex items-center justify-center gap-2"
          >
            <FileText className="w-4 h-4" />
            Browse Prompts
          </Link>
        </div>
      </div>
    </div>
  );
}
