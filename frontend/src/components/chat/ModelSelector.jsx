import { Cpu, ChevronDown } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import { getModelsForAgent, getModelByValue, useEnums } from '../../lib/agents';

export default function ModelSelector({ value, onChange, agent, disabled, direction = 'down' }) {
  const [isOpen, setIsOpen] = useState(false);
  const ref = useRef(null);

  // Subscribe to enums updates for re-render when API data loads
  useEnums();

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (ref.current && !ref.current.contains(event.target)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const models = getModelsForAgent(agent);
  const selected = getModelByValue(agent, value);

  const dropdownClasses = direction === 'up' 
    ? 'bottom-full mb-1' 
    : 'top-full mt-1';

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
        <div 
          className={`absolute left-0 z-50 min-w-[200px] rounded-lg border border-border bg-popover shadow-lg animate-fade-in ${dropdownClasses}`}
          onClick={(e) => e.stopPropagation()}
        >
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
