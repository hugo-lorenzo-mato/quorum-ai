import { Plus } from 'lucide-react';

export default function FAB({ onClick, icon: Icon = Plus, label = 'New', className = '' }) {
  return (
    <button
      onClick={onClick}
      className={`md:hidden fixed bottom-20 right-4 z-50 flex items-center justify-center w-14 h-14 rounded-full bg-primary text-primary-foreground shadow-lg shadow-primary/30 hover:bg-primary/90 hover:scale-105 active:scale-95 transition-all ${className}`}
      aria-label={label}
    >
      <Icon className="w-6 h-6" />
    </button>
  );
}
