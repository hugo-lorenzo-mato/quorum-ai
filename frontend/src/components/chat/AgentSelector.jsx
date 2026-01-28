import { Bot, ChevronDown } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';

const AGENTS = [
  { value: 'claude', label: 'Claude', description: 'Anthropic AI' },
  { value: 'gemini', label: 'Gemini', description: 'Google AI' },
  { value: 'codex', label: 'Codex', description: 'OpenAI Codex' },
  { value: 'opencode', label: 'OpenCode', description: 'Open Source' },
];

export default function AgentSelector({ value, onChange, disabled }) {
  const [isOpen, setIsOpen] = useState(false);
  const ref = useRef(null);

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (ref.current && !ref.current.contains(event.target)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const selected = AGENTS.find(a => a.value === value) || AGENTS[0];

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-border bg-background hover:bg-accent text-sm transition-colors disabled:opacity-50"
      >
        <Bot className="w-4 h-4 text-muted-foreground" />
        <span className="font-medium">{selected.label}</span>
        <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-1 z-50 min-w-[180px] rounded-lg border border-border bg-popover shadow-lg animate-fade-in">
          <div className="p-1">
            {AGENTS.map((agent) => (
              <button
                key={agent.value}
                type="button"
                onClick={() => {
                  onChange(agent.value);
                  setIsOpen(false);
                }}
                className={`w-full flex items-start gap-3 p-2 rounded-md text-left transition-colors ${
                  value === agent.value
                    ? 'bg-accent text-accent-foreground'
                    : 'hover:bg-accent/50'
                }`}
              >
                <Bot className={`w-4 h-4 mt-0.5 ${value === agent.value ? 'text-primary' : 'text-muted-foreground'}`} />
                <div>
                  <p className="text-sm font-medium">{agent.label}</p>
                  <p className="text-xs text-muted-foreground">{agent.description}</p>
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
