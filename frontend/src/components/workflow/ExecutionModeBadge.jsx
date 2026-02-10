import PropTypes from 'prop-types';
import { Users, User, Zap, Eye } from 'lucide-react';

function capitalizeFirst(str) {
  if (!str) return '';
  return str.charAt(0).toUpperCase() + str.slice(1);
}

function getModeKey(blueprint) {
  const raw = blueprint?.execution_mode;
  if (raw === 'single_agent' || raw === 'interactive') return raw;
  return 'multi_agent';
}

const MODE_META = {
  multi_agent: {
    badgeClass: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
    detailedClass: 'bg-purple-50 dark:bg-purple-950',
    IconBadge: Users,
    IconInline: Users,
    iconClass: 'text-purple-600 dark:text-purple-400',
    labelBadge: 'Multi-Agent Consensus',
    labelInline: 'Multi-Agent',
    labelDetailed: 'Multi-Agent Consensus',
  },
  single_agent: {
    badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    detailedClass: 'bg-blue-50 dark:bg-blue-950',
    IconBadge: Zap,
    IconInline: User,
    iconClass: 'text-blue-600 dark:text-blue-400',
    labelBadge: 'Single Agent',
    labelInline: 'Single Agent',
    labelDetailed: 'Single Agent',
  },
  interactive: {
    badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
    detailedClass: 'bg-amber-50 dark:bg-amber-950',
    IconBadge: Eye,
    IconInline: Eye,
    iconClass: 'text-amber-600 dark:text-amber-400',
    labelBadge: 'Interactive',
    labelInline: 'Interactive',
    labelDetailed: 'Interactive (Review between phases)',
  },
};

/**
 * Displays the execution mode of a workflow as a badge.
 * @param {Object} props
 * @param {Object} props.blueprint - Workflow blueprint
 * @param {string} [props.blueprint.execution_mode] - 'multi_agent', 'single_agent', or 'interactive'
 * @param {string} [props.blueprint.single_agent_name] - Agent name for single-agent mode
 * @param {string} [props.variant] - 'badge', 'inline', or 'detailed'
 */
export function ExecutionModeBadge({ blueprint, variant = 'badge' }) {
  const modeKey = getModeKey(blueprint);
  const meta = MODE_META[modeKey] || MODE_META.multi_agent;
  const agentName = blueprint?.single_agent_name;

  if (variant === 'detailed') {
    const Icon = meta.IconBadge;
    return (
      <div className={`flex items-center gap-2 px-3 py-2 rounded-lg ${meta.detailedClass}`}>
        <Icon className={`w-4 h-4 ${meta.iconClass}`} />
        <div>
          <span className="font-medium">{meta.labelDetailed}</span>
          {modeKey === 'single_agent' && agentName && (
            <span className="text-sm ml-1 opacity-80">({capitalizeFirst(agentName)})</span>
          )}
        </div>
      </div>
    );
  }

  if (variant === 'inline') {
    const Icon = meta.IconInline;
    return (
      <span className="text-sm text-muted-foreground">
        <Icon className="w-3.5 h-3.5 inline mr-1" />
        {meta.labelInline}
        {modeKey === 'single_agent' && agentName && <span className="ml-1">({capitalizeFirst(agentName)})</span>}
      </span>
    );
  }

  const Icon = meta.IconBadge;
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${meta.badgeClass}`}>
      <Icon className="w-3 h-3" />
      {meta.labelBadge}
      {modeKey === 'single_agent' && agentName && <span className="opacity-75">• {capitalizeFirst(agentName)}</span>}
    </span>
  );
}

ExecutionModeBadge.propTypes = {
  blueprint: PropTypes.shape({
    // execution_mode can be omitted/empty to indicate default (multi_agent).
    execution_mode: PropTypes.oneOf(['multi_agent', 'single_agent', 'interactive', '']),
    single_agent_name: PropTypes.string,
  }),
  variant: PropTypes.oneOf(['badge', 'inline', 'detailed']),
};
