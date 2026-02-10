import { Bot, ChevronDown, Loader2 } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import { getAgentByValue, useEnabledAgents } from '../../lib/agents';

export default function AgentSelector({ value, onChange, disabled, direction = 'down' }) {
  const [isOpen, setIsOpen] = useState(false);
  const ref = useRef(null);

  // Get only enabled agents (filters by config.agents.{name}.enabled)
  const agents = useEnabledAgents();
  const isLoading = agents.length === 0;

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (ref.current && !ref.current.contains(event.target)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const selected = getAgentByValue(value);

  // If current agent is disabled, switch to first enabled agent
  useEffect(() => {
    if (agents.length > 0 && !agents.find((a) => a.value === value)) {
      onChange(agents[0].value);
    }
  }, [agents, value, onChange]);

  const dropdownClasses = direction === 'up' 
    ? 'bottom-full mb-1' 
    : 'top-full mt-1';

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => !disabled && !isLoading && setIsOpen(!isOpen)}
        disabled={disabled || isLoading}
        className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-border bg-background hover:bg-accent text-sm transition-colors disabled:opacity-50"
      >
        {isLoading ? (
          <Loader2 className="w-4 h-4 text-muted-foreground animate-spin" />
        ) : (
          <Bot className="w-4 h-4 text-muted-foreground" />
        )}
        <span className="font-medium">{isLoading ? 'Loading...' : selected.label}</span>
        <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div 
          className={`absolute left-0 z-50 min-w-[180px] rounded-lg border border-border bg-popover shadow-lg animate-fade-in ${dropdownClasses}`}
        >
          <div className="p-1">
            {agents.map((agent) => (
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
