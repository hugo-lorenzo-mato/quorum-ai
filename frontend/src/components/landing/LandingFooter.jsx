import { Github } from 'lucide-react';

export default function LandingFooter() {
  return (
    <footer className="w-full max-w-5xl mx-auto px-6 py-8 border-t border-border mt-8">
      <div className="flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-muted-foreground">
        <p>&copy; {new Date().getFullYear()} Quorum AI &mdash; Open-source multi-agent consensus engine</p>
        <a
          href="https://github.com/hugo-lorenzo-mato/quorum-ai"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 hover:text-foreground transition-colors"
        >
          <Github className="w-4 h-4" />
          GitHub
        </a>
      </div>
    </footer>
  );
}
