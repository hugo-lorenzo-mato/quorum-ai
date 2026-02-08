export default function LoopArrow({ width = 120, active = false, label = '< threshold' }) {
  const startX = width * 0.15;
  const endX = width * 0.85;
  const midX = width * 0.5;
  const topY = 4;
  const botY = 40;

  return (
    <svg width={width} height="50" aria-hidden="true">
      <defs>
        <linearGradient id="loop-grad" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="#6366f1" />
          <stop offset="50%" stopColor="#8b5cf6" />
          <stop offset="100%" stopColor="#06b6d4" />
        </linearGradient>
        <marker id="loop-arrow-head" viewBox="0 0 8 8" refX="6" refY="4" markerWidth="6" markerHeight="6" orient="auto">
          <polygon points="0 0, 8 4, 0 8" fill="url(#loop-grad)" />
        </marker>
      </defs>
      <path
        d={`M ${startX} ${topY} C ${startX} ${botY}, ${endX} ${botY}, ${endX} ${topY}`}
        fill="none"
        stroke="url(#loop-grad)"
        strokeWidth="1.5"
        strokeDasharray={active ? '6 4' : 'none'}
        className={active ? 'animate-dash-flow' : ''}
        markerEnd="url(#loop-arrow-head)"
      />
      <text x={midX} y={botY + 8} textAnchor="middle" className="text-[9px] fill-muted-foreground">
        <tspan className="px-1 rounded-full">{label}</tspan>
      </text>
    </svg>
  );
}
