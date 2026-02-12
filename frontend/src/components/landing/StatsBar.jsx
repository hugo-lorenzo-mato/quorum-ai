import { Bot, Layers, Palette, Code2 } from 'lucide-react';

const stats = [
  { value: '5', label: 'AI Agents', icon: Bot, color: 'bg-primary/10 text-primary' },
  { value: '15+', label: 'Models', icon: Layers, color: 'bg-info/10 text-info' },
  { value: '8', label: 'Themes', icon: Palette, color: 'bg-warning/10 text-warning' },
  { value: '100%', label: 'Open Source', icon: Code2, color: 'bg-success/10 text-success' },
];

export default function StatsBar() {
  return (
    <section className="w-full max-w-5xl mx-auto px-6 py-12" aria-label="Quorum AI stats">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {stats.map((stat, index) => {
          const Icon = stat.icon;
          return (
            <div
              key={stat.label}
              className="group relative rounded-xl border border-border bg-card p-4 transition-all hover:border-muted-foreground/30 hover:shadow-lg animate-fade-up"
              style={{ animationDelay: `${index * 100}ms`, animationFillMode: 'both' }}
            >
              <div className="flex items-start gap-3">
                <div className={`p-2 rounded-lg ${stat.color}`}>
                  <Icon className="w-4 h-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-2xl font-mono font-semibold text-foreground">{stat.value}</p>
                  <p className="text-xs font-medium text-muted-foreground">{stat.label}</p>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
