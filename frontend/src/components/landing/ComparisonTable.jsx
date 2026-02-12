import { Check, X } from 'lucide-react';

const rows = [
  { feature: 'Multi-agent orchestration', quorum: true, single: false },
  { feature: 'Iterative consensus refinement', quorum: true, single: false },
  { feature: 'DAG-based task decomposition', quorum: true, single: false },
  { feature: 'Git worktree isolation', quorum: true, single: false },
  { feature: 'Real-time streaming UI', quorum: true, single: 'partial' },
  { feature: 'Theme support (8 themes)', quorum: true, single: false },
  { feature: 'Agent-agnostic (5+ providers)', quorum: true, single: false },
  { feature: '100% open source', quorum: true, single: 'partial' },
];

function Cell({ value }) {
  if (value === true)
    return <Check className="w-4 h-4 text-status-success mx-auto" />;
  if (value === 'partial')
    return <span className="text-xs text-muted-foreground">Varies</span>;
  return <X className="w-4 h-4 text-muted-foreground/40 mx-auto" />;
}

export default function ComparisonTable() {
  return (
    <section className="w-full max-w-3xl mx-auto px-6 py-16">
      <div className="text-center mb-10">
        <h2 className="text-3xl font-bold text-foreground">
          Why Quorum?
        </h2>
        <p className="text-muted-foreground mt-2">
          Multi-agent consensus vs. single-agent workflows
        </p>
      </div>

      <div className="rounded-xl border border-border overflow-hidden">
        {/* Header */}
        <div className="grid grid-cols-[1fr_100px_100px] bg-muted/30 border-b border-border text-xs font-semibold text-muted-foreground uppercase tracking-wider">
          <div className="px-4 py-3">Feature</div>
          <div className="px-4 py-3 text-center">Quorum</div>
          <div className="px-4 py-3 text-center">Single Agent</div>
        </div>

        {/* Rows */}
        {rows.map((row, i) => (
          <div
            key={row.feature}
            className={`grid grid-cols-[1fr_100px_100px] items-center text-sm ${
              i !== rows.length - 1 ? 'border-b border-border' : ''
            } ${i % 2 === 0 ? 'bg-card' : 'bg-card/50'}`}
          >
            <div className="px-4 py-3 text-foreground">{row.feature}</div>
            <div className="px-4 py-3 text-center">
              <Cell value={row.quorum} />
            </div>
            <div className="px-4 py-3 text-center">
              <Cell value={row.single} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
