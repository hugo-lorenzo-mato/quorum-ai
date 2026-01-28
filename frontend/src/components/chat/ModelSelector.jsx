import { Cpu, ChevronDown } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';

// Models grouped by agent
const AGENT_MODELS = {
  claude: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'claude-sonnet-4-5-20250929', label: 'Sonnet 4.5', description: 'Fast & capable' },
    { value: 'claude-opus-4-5-20251101', label: 'Opus 4.5', description: 'Most powerful' },
    { value: 'claude-haiku-3-5-20241022', label: 'Haiku 3.5', description: 'Quick & efficient' },
  ],
  gemini: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'gemini-2.5-pro-preview-05-06', label: 'Gemini 2.5 Pro', description: 'Advanced reasoning' },
    { value: 'gemini-2.5-flash-preview-05-20', label: 'Gemini 2.5 Flash', description: 'Fast responses' },
    { value: 'gemini-2.0-flash', label: 'Gemini 2.0 Flash', description: 'Quick & capable' },
  ],
  codex: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'gpt-5.2', label: 'GPT-5.2', description: 'Latest model' },
    { value: 'gpt-5.1-codex', label: 'GPT-5.1 Codex', description: 'Code optimized' },
    { value: 'o3', label: 'o3', description: 'Reasoning model' },
    { value: 'o4-mini', label: 'o4-mini', description: 'Fast reasoning' },
  ],
  opencode: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'anthropic/claude-sonnet-4-5-20250929', label: 'Sonnet 4.5', description: 'Via OpenCode' },
    { value: 'openai/gpt-4o', label: 'GPT-4o', description: 'OpenAI via OpenCode' },
  ],
};

export default function ModelSelector({ value, onChange, agent, disabled }) {
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

  const models = AGENT_MODELS[agent] || AGENT_MODELS.claude;
  const selected = models.find(m => m.value === value) || models[0];

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-border bg-background hover:bg-accent text-sm transition-colors disabled:opacity-50"
      >
        <Cpu className="w-4 h-4 text-muted-foreground" />
        <span className="font-medium">{selected.label}</span>
        <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-1 z-50 min-w-[200px] rounded-lg border border-border bg-popover shadow-lg animate-fade-in">
          <div className="p-1 max-h-64 overflow-y-auto">
            {models.map((model) => (
              <button
                key={model.value}
                type="button"
                onClick={() => {
                  onChange(model.value);
                  setIsOpen(false);
                }}
                className={`w-full flex items-start gap-3 p-2 rounded-md text-left transition-colors ${
                  value === model.value
                    ? 'bg-accent text-accent-foreground'
                    : 'hover:bg-accent/50'
                }`}
              >
                <Cpu className={`w-4 h-4 mt-0.5 ${value === model.value ? 'text-primary' : 'text-muted-foreground'}`} />
                <div>
                  <p className="text-sm font-medium">{model.label}</p>
                  <p className="text-xs text-muted-foreground">{model.description}</p>
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
