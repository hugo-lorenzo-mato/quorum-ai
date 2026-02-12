import { Link } from 'react-router-dom';
import Logo from '../components/Logo';
import {
  LayoutDashboard,
  GitBranch,
  MessageSquare,
  KanbanSquare,
  Zap,
  ArrowRight,
} from 'lucide-react';

export default function Landing() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[calc(100vh-10rem)] px-4">
      <div className="max-w-2xl w-full text-center space-y-8">
        {/* Hero */}
        <div className="space-y-4">
          <div className="relative inline-block mb-2">
            <div className="absolute inset-0 bg-primary/20 blur-xl rounded-full" />
            <div className="relative p-6 rounded-3xl bg-card border border-border shadow-sm">
              <Logo className="w-16 h-16" />
            </div>
          </div>
          <h1 className="text-3xl sm:text-4xl font-bold text-foreground tracking-tight">
            Quorum AI
          </h1>
          <p className="text-base sm:text-lg text-muted-foreground max-w-md mx-auto leading-relaxed">
            Your intelligent consensus engine. Orchestrate AI agents to collaborate, debate, and deliver high-quality results.
          </p>
        </div>

        {/* CTA */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
          <Link
            to="/dashboard"
            className="w-full sm:w-auto px-6 py-3 rounded-xl bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-all flex items-center justify-center gap-2 shadow-lg shadow-primary/20"
          >
            <LayoutDashboard className="w-4 h-4" />
            Go to Dashboard
            <ArrowRight className="w-4 h-4" />
          </Link>
          <Link
            to="/workflows/new"
            className="w-full sm:w-auto px-6 py-3 rounded-xl border border-border text-foreground text-sm font-medium hover:bg-accent transition-all flex items-center justify-center gap-2"
          >
            <Zap className="w-4 h-4" />
            Start a Workflow
          </Link>
        </div>

        {/* Quick Links */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-4">
          {[
            { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
            { to: '/workflows', label: 'Workflows', icon: GitBranch },
            { to: '/kanban', label: 'Kanban', icon: KanbanSquare },
            { to: '/chat', label: 'Chat', icon: MessageSquare },
          ].map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className="flex flex-col items-center gap-2 p-4 rounded-xl border border-border bg-card/50 hover:bg-accent/50 transition-all group"
            >
              <Icon className="w-5 h-5 text-muted-foreground group-hover:text-primary transition-colors" />
              <span className="text-xs font-medium text-muted-foreground group-hover:text-foreground transition-colors">
                {label}
              </span>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
