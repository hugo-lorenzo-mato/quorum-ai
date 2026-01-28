import { useState, useRef, useEffect } from 'react';
import { Info } from 'lucide-react';

export function Tooltip({ content, children }) {
  const [isVisible, setIsVisible] = useState(false);
  const [position, setPosition] = useState({ top: 0, left: 0 });
  const triggerRef = useRef(null);
  const tooltipRef = useRef(null);

  useEffect(() => {
    if (isVisible && triggerRef.current && tooltipRef.current) {
      const trigger = triggerRef.current.getBoundingClientRect();
      const tooltip = tooltipRef.current.getBoundingClientRect();
      const viewport = { width: window.innerWidth, height: window.innerHeight };

      let top = trigger.top - tooltip.height - 8;
      let left = trigger.left + (trigger.width / 2) - (tooltip.width / 2);

      // Flip to bottom if no space above
      if (top < 8) {
        top = trigger.bottom + 8;
      }

      // Keep within viewport horizontally
      if (left < 8) left = 8;
      if (left + tooltip.width > viewport.width - 8) {
        left = viewport.width - tooltip.width - 8;
      }

      setPosition({ top, left });
    }
  }, [isVisible]);

  return (
    <span className="relative inline-flex items-center">
      <button
        ref={triggerRef}
        type="button"
        className="p-1 text-muted-foreground hover:text-foreground transition-colors focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 rounded"
        onMouseEnter={() => setIsVisible(true)}
        onMouseLeave={() => setIsVisible(false)}
        onFocus={() => setIsVisible(true)}
        onBlur={() => setIsVisible(false)}
        aria-label="More information"
      >
        {children || <Info className="w-4 h-4" />}
      </button>

      {isVisible && (
        <div
          ref={tooltipRef}
          role="tooltip"
          className="fixed z-50 px-3 py-2 text-sm text-primary-foreground bg-primary rounded-lg shadow-lg max-w-xs animate-fade-in"
          style={{ top: position.top, left: position.left }}
        >
          {content}
          <div className="absolute w-2 h-2 bg-primary transform rotate-45 -bottom-1 left-1/2 -translate-x-1/2" />
        </div>
      )}
    </span>
  );
}

// Inline tooltip for use in labels
export function InfoTooltip({ content }) {
  return (
    <Tooltip content={content}>
      <Info className="w-4 h-4" />
    </Tooltip>
  );
}
