export default function LoopArrow({ width = 120, active = false, label = '< threshold' }) {
  const startX = width * 0.15;
  const endX = width * 0.85;
  const midX = width * 0.5;
  const topY = 4;
  const botY = 40;

  return (
    <svg width={width} height="50" className="text-muted-foreground" aria-hidden="true">
      <defs>
        <marker id="loop-arrow-head" viewBox="0 0 8 8" refX="6" refY="4" markerWidth="6" markerHeight="6" orient="auto">
          <polygon points="0 0, 8 4, 0 8" fill="currentColor" />
        </marker>
      </defs>
      <path
        d={`M ${startX} ${topY} C ${startX} ${botY}, ${endX} ${botY}, ${endX} ${topY}`}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeDasharray={active ? '6 4' : 'none'}
        className={active ? 'animate-dash-flow' : ''}
        markerEnd="url(#loop-arrow-head)"
      />
      <text x={midX} y={botY + 8} textAnchor="middle" className="text-[9px] fill-muted-foreground">
        {label}
      </text>
    </svg>
  );
}
