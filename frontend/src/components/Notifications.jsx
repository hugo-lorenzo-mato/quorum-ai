import { CheckCircle2, AlertTriangle, Info, XCircle, X } from 'lucide-react';
import { useUIStore } from '../stores';

const typeConfig = {
  success: {
    icon: CheckCircle2,
    className: 'border-success/30 bg-success/10 text-success',
  },
  error: {
    icon: XCircle,
    className: 'border-destructive/30 bg-destructive/10 text-destructive',
  },
  warning: {
    icon: AlertTriangle,
    className: 'border-warning/30 bg-warning/10 text-warning',
  },
  info: {
    icon: Info,
    className: 'border-primary/30 bg-primary/10 text-primary',
  },
};

export default function Notifications() {
  const { notifications, removeNotification } = useUIStore();

  if (!notifications.length) return null;

  return (
    <div className="fixed top-4 right-4 z-[60] flex flex-col gap-2 w-[90vw] max-w-sm">
      {notifications.map((notification) => {
        const config = typeConfig[notification.type] || typeConfig.info;
        const Icon = config.icon;

        return (
          <div
            key={notification.id}
            className={`flex items-start gap-3 rounded-lg border px-4 py-3 shadow-lg backdrop-blur ${config.className}`}
            role="alert"
          >
            <Icon className="w-5 h-5 mt-0.5 flex-shrink-0" />
            <div className="flex-1 text-sm leading-relaxed">
              {notification.message}
            </div>
            <button
              onClick={() => removeNotification(notification.id)}
              className="rounded-md p-1 text-current/70 hover:text-current hover:bg-white/10 transition"
              aria-label="Dismiss notification"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        );
      })}
    </div>
  );
}

