import { useState, useRef, useEffect } from 'react';

// Same colors as backend: internal/project/registry.go
const PROJECT_COLORS = [
  '#4A90D9',  // Blue
  '#7B68EE',  // Medium Slate Blue
  '#20B2AA',  // Light Sea Green
  '#FF6B6B',  // Coral Red
  '#FFA500',  // Orange
  '#9370DB',  // Medium Purple
  '#3CB371',  // Medium Sea Green
  '#FFD700',  // Gold
  '#00CED1',  // Dark Turquoise
  '#FF69B4',  // Hot Pink
];

/**
 * ColorPicker component for selecting project colors.
 * Shows predefined colors + custom color input.
 */
export default function ColorPicker({ value, onChange, className = '' }) {
  const [showPicker, setShowPicker] = useState(false);
  const pickerRef = useRef(null);

  // Close picker when clicking outside
  useEffect(() => {
    const handleClickOutside = (e) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target)) {
        setShowPicker(false);
      }
    };

    if (showPicker) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showPicker]);

  const handleColorSelect = (color) => {
    onChange(color);
    setShowPicker(false);
  };

  return (
    <div className={`relative ${className}`} ref={pickerRef}>
      {/* Color trigger button */}
      <button
        type="button"
        onClick={() => setShowPicker(!showPicker)}
        className="w-7 h-7 rounded-md border-2 border-border hover:border-foreground/50 transition-all shadow-sm hover:scale-105"
        style={{ backgroundColor: value || PROJECT_COLORS[0] }}
        title="Change color"
      />

      {/* Color picker dropdown */}
      {showPicker && (
        <div className="absolute top-full left-0 mt-2 p-3 bg-popover border border-border rounded-xl shadow-xl z-50 animate-in fade-in zoom-in-95 duration-100">
          <div className="grid grid-cols-5 gap-2 mb-3">
            {PROJECT_COLORS.map((color) => (
              <button
                key={color}
                type="button"
                onClick={() => handleColorSelect(color)}
                className={`w-7 h-7 rounded-md border-2 transition-all hover:scale-110 ${
                  value === color
                    ? 'border-foreground ring-2 ring-primary/50'
                    : 'border-transparent hover:border-foreground/30'
                }`}
                style={{ backgroundColor: color }}
                title={color}
              />
            ))}
          </div>

          {/* Custom color input */}
          <div className="flex items-center gap-2 pt-2 border-t border-border">
            <label className="text-xs text-muted-foreground">Custom:</label>
            <input
              type="color"
              value={value || PROJECT_COLORS[0]}
              onChange={(e) => handleColorSelect(e.target.value)}
              className="w-8 h-6 rounded cursor-pointer border border-border"
            />
            <input
              type="text"
              value={value || ''}
              onChange={(e) => {
                const val = e.target.value;
                if (/^#[0-9A-Fa-f]{0,6}$/.test(val) || val === '') {
                  onChange(val);
                }
              }}
              placeholder="#RRGGBB"
              className="flex-1 px-2 py-1 text-xs font-mono rounded border border-input bg-background text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              maxLength={7}
            />
          </div>
        </div>
      )}
    </div>
  );
}

// Export colors for use in other components
export { PROJECT_COLORS };
