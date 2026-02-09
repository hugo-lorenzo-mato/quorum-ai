export default function FlowConnector({ completed = false, nextRunning = false }) {
  return (
    <div className="flex items-center px-0.5">
      <svg width="24" height="12" viewBox="0 0 24 12" aria-hidden="true" className="flex-shrink-0">
        <defs>
          <linearGradient id="fc-grad-done" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="var(--status-success)" stopOpacity="0.6" />
            <stop offset="100%" stopColor="var(--status-success)" />
          </linearGradient>
        </defs>
        {/* Line */}
        <line
          x1="0" y1="6" x2="18" y2="6"
          stroke={completed ? 'url(#fc-grad-done)' : 'var(--muted-fg)'}
          strokeWidth="1.5"
          strokeOpacity={completed ? 1 : 0.3}
          strokeDasharray={nextRunning ? '4 3' : 'none'}
          className={nextRunning ? 'animate-dash-flow' : ''}
        />
        {/* Arrow head */}
        <polygon
          points="17,3 22,6 17,9"
          fill={completed ? 'var(--status-success)' : 'var(--muted-fg)'}
          fillOpacity={completed ? 1 : 0.3}
        />
      </svg>
    </div>
  );
}
