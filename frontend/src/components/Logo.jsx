import React from 'react';

export default function Logo({ className = "w-6 h-6", ...props }) {
  // Unique IDs for gradients to avoid conflicts if multiple logos are rendered
  const gradientId = React.useId();
  const fillId = React.useId();

  return (
    <svg 
      xmlns="http://www.w3.org/2000/svg" 
      viewBox="0 0 24 24" 
      fill="none" 
      stroke={`url(#${gradientId})`}
      strokeWidth="2" 
      strokeLinecap="round" 
      strokeLinejoin="round"
      className={className}
      {...props}
    >
      <defs>
        <linearGradient id={gradientId} x1="2" y1="2" x2="22" y2="22" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#6366f1" /> {/* Indigo 500 */}
          <stop offset="50%" stopColor="#8b5cf6" /> {/* Violet 500 */}
          <stop offset="100%" stopColor="#06b6d4" /> {/* Cyan 500 */}
        </linearGradient>
        <linearGradient id={fillId} x1="2" y1="2" x2="22" y2="22" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#6366f1" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#06b6d4" stopOpacity="0.3" />
        </linearGradient>
      </defs>
      
      {/* The Q Outer Shell - Hexagonal with Tail */}
      {/* Top Left to Bottom Left */}
      <path d="M12 2.5L4.5 6.8V17.2L12 21.5" />
      {/* Top Right part */}
      <path d="M12 2.5L19.5 6.8V12" />
      {/* The Tail extending from bottom right */}
      <path d="M17.5 18.5L21.5 21.5" />
      
      {/* Inner Nexus (Consensus) */}
      <g strokeWidth="1.5">
        {/* Central Core */}
        <circle cx="12" cy="12" r="2.5" fill={`url(#${fillId})`} stroke="none" />
        
        {/* Connection Lines */}
        <path d="M12 12L7 7" strokeOpacity="0.5" />
        <path d="M12 12L17 7" strokeOpacity="0.5" />
        <path d="M12 12V17" strokeOpacity="0.5" />
        
        {/* Agent Nodes */}
        <circle cx="7" cy="7" r="1.5" fill="currentColor" stroke="none" className="text-foreground" />
        <circle cx="17" cy="7" r="1.5" fill="currentColor" stroke="none" className="text-foreground" />
        <circle cx="12" cy="17" r="1.5" fill="currentColor" stroke="none" className="text-foreground" />
      </g>
    </svg>
  );
}
