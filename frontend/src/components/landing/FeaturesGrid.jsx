import {
  GitBranch,
  GitMerge,
  MessageSquare,
  Monitor,
  Palette,
  Users,
} from 'lucide-react';

const features = [
  {
    icon: Users,
    title: 'Multi-Agent Orchestration',
    description:
      'Run Claude, Gemini, Codex, Copilot, and OpenCode simultaneously on complex tasks',
  },
  {
    icon: GitBranch,
    title: 'DAG Planning & Execution',
    description:
      'Automatic task decomposition with dependency-aware parallel execution',
  },
  {
    icon: Palette,
    title: '8 Beautiful Themes',
    description:
      'From Light to Midnight OLED, Dracula, Nord, Ocean - pick your vibe',
  },
  {
    icon: Monitor,
    title: 'Real-time WebUI',
    description:
      'Live dashboard with workflow monitoring, metrics, and interactive chat',
  },
  {
    icon: GitMerge,
    title: 'Git-Aware Workflows',
    description:
      'Worktree isolation, branch management, and artifact reconciliation built-in',
  },
  {
    icon: MessageSquare,
    title: 'Interactive Chat',
    description:
      'Conversational interface to any configured AI agent with streaming responses',
  },
];

export default function FeaturesGrid() {
  return (
    <section className="w-full max-w-5xl mx-auto px-6 py-16">
      <div className="text-center mb-10">
        <h2 className="text-3xl font-bold text-foreground">Features</h2>
        <p className="text-muted-foreground mt-2">
          Everything you need to orchestrate AI coding at scale
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {features.map((feature, index) => {
          const Icon = feature.icon;

          return (
            <div
              key={feature.title}
              className="group relative rounded-xl border border-border bg-card p-6 transition-all hover:border-muted-foreground/30 hover:shadow-lg animate-fade-up"
              style={{ animationDelay: `${index * 80}ms`, animationFillMode: 'both' }}
            >
              <div className="p-2 rounded-lg bg-primary/10 text-primary w-fit mb-4">
                <Icon className="w-5 h-5" />
              </div>
              <h3 className="text-lg font-semibold text-foreground mb-2">
                {feature.title}
              </h3>
              <p className="text-sm text-muted-foreground leading-relaxed">
                {feature.description}
              </p>
            </div>
          );
        })}
      </div>
    </section>
  );
}
