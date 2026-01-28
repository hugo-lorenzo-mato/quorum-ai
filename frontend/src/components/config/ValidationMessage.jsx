import { AlertCircle } from 'lucide-react';

export function ValidationMessage({ error, className = '' }) {
  if (!error) return null;

  return (
    <div
      role="alert"
      className={`flex items-start gap-2 mt-1.5 text-sm text-error ${className}`}
    >
      <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
      <span>{error}</span>
    </div>
  );
}
