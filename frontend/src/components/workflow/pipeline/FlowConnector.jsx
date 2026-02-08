export default function FlowConnector({ completed = false }) {
  return (
    <div className="flex items-center px-0.5">
      <div
        className={`w-4 h-0.5 ${completed ? 'bg-status-success' : 'bg-muted-foreground/30'}`}
      />
      <div
        className={`w-0 h-0 border-t-[3px] border-t-transparent border-b-[3px] border-b-transparent
          ${completed ? 'border-l-[4px] border-l-status-success' : 'border-l-[4px] border-l-muted-foreground/30'}
        `}
      />
    </div>
  );
}
