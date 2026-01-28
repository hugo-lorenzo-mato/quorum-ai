import { Brain, ChevronDown } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';

const REASONING_LEVELS = [
  { value: 'minimal', label: 'Minimal', description: 'Quick responses' },
  { value: 'low', label: 'Low', description: 'Light reasoning' },
  { value: 'medium', label: 'Medium', description: 'Balanced (default)' },
  { value: 'high', label: 'High', description: 'Deep analysis' },
  { value: 'xhigh', label: 'Max', description: 'Maximum reasoning' },
];

// Agents that support reasoning effort
const REASONING_AGENTS = ['codex'];

export default function ReasoningSelector({ value, onChange, agent, disabled }) {
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

  // Only show for agents that support reasoning effort
  if (!REASONING_AGENTS.includes(agent)) {
    return null;
  }

  const selected = REASONING_LEVELS.find(r => r.value === value) || REASONING_LEVELS[2];

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-border bg-background hover:bg-accent text-sm transition-colors disabled:opacity-50"
      >
        <Brain className="w-4 h-4 text-muted-foreground" />
        <span className="font-medium">{selected.label}</span>
        <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-1 z-50 min-w-[180px] rounded-lg border border-border bg-popover shadow-lg animate-fade-in">
          <div className="p-1">
            {REASONING_LEVELS.map((level) => (
              <button
                key={level.value}
                type="button"
                onClick={() => {
                  onChange(level.value);
                  setIsOpen(false);
                }}
                className={`w-full flex items-start gap-3 p-2 rounded-md text-left transition-colors ${
                  value === level.value
                    ? 'bg-accent text-accent-foreground'
                    : 'hover:bg-accent/50'
                }`}
              >
                <Brain className={`w-4 h-4 mt-0.5 ${value === level.value ? 'text-primary' : 'text-muted-foreground'}`} />
                <div>
                  <p className="text-sm font-medium">{level.label}</p>
                  <p className="text-xs text-muted-foreground">{level.description}</p>
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
