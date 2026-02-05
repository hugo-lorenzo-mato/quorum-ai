import React from 'react';

export default function Logo({ className = "w-6 h-6", ...props }) {
  const gradientId = React.useId();

  return (
    <svg 
      xmlns="http://www.w3.org/2000/svg" 
      viewBox="0 0 24 24" 
      fill="none" 
      className={className}
      {...props}
    >
      <defs>
        <linearGradient id={gradientId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#6366f1" /> {/* Indigo 500 */}
          <stop offset="50%" stopColor="#8b5cf6" /> {/* Violet 500 */}
          <stop offset="100%" stopColor="#06b6d4" /> {/* Cyan 500 */}
        </linearGradient>
      </defs>

      {/* Órbitas Finas */}
      <ellipse cx="12" cy="11" rx="10" ry="4" stroke="#6366f1" strokeWidth="0.75" strokeOpacity="0.5" />
      <ellipse cx="12" cy="11" rx="10" ry="4" stroke="#8b5cf6" strokeWidth="0.75" strokeOpacity="0.5" transform="rotate(60 12 11)" />
      <ellipse cx="12" cy="11" rx="10" ry="4" stroke="#06b6d4" strokeWidth="0.75" strokeOpacity="0.5" transform="rotate(120 12 11)" />
      
      {/* Electrones */}
      <circle cx="22" cy="11" r="1.5" fill="#6366f1" />
      <circle cx="7" cy="19.5" r="1.5" fill="#8b5cf6" />
      <circle cx="7" cy="2.5" r="1.5" fill="#06b6d4" />
      
      {/* Q Central (Core Soft) */}
      <circle cx="12" cy="11" r="4" stroke={`url(#${gradientId})`} strokeWidth="2.5" fill="none" strokeLinecap="round" />
      <path d="M14.5 14C15.5 15 17 16 18 16.5" stroke={`url(#${gradientId})`} strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}
