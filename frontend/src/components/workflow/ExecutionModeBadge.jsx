import { Users, User, Zap, Eye } from 'lucide-react';

/**
 * Displays the execution mode of a workflow as a badge.
 * @param {Object} props
 * @param {Object} props.blueprint - Workflow blueprint
 * @param {string} [props.blueprint.execution_mode] - 'multi_agent', 'single_agent', or 'interactive'
 * @param {string} [props.blueprint.single_agent_name] - Agent name for single-agent mode
 * @param {string} [props.variant] - 'badge', 'inline', or 'detailed'
 */
export function ExecutionModeBadge({ blueprint, variant = 'badge' }) {
  const isSingleAgent = blueprint?.execution_mode === 'single_agent';
  const isInteractive = blueprint?.execution_mode === 'interactive';
  const agentName = blueprint?.single_agent_name;

  if (variant === 'detailed') {
    return (
      <div className={`flex items-center gap-2 px-3 py-2 rounded-lg ${
        isInteractive ? 'bg-amber-50 dark:bg-amber-950' :
        isSingleAgent ? 'bg-blue-50 dark:bg-blue-950' : 'bg-purple-50 dark:bg-purple-950'
      }`}>
        {isInteractive ? (
          <>
            <Eye className="w-4 h-4 text-amber-600 dark:text-amber-400" />
            <span className="font-medium text-amber-900 dark:text-amber-100">
              Interactive (Review between phases)
            </span>
          </>
        ) : isSingleAgent ? (
          <>
            <Zap className="w-4 h-4 text-blue-600 dark:text-blue-400" />
            <div>
              <span className="font-medium text-blue-900 dark:text-blue-100">Single Agent</span>
              {agentName && (
                <span className="text-blue-700 dark:text-blue-300 text-sm ml-1">
                  ({capitalizeFirst(agentName)})
                </span>
              )}
            </div>
          </>
        ) : (
          <>
            <Users className="w-4 h-4 text-purple-600 dark:text-purple-400" />
            <span className="font-medium text-purple-900 dark:text-purple-100">
              Multi-Agent Consensus
            </span>
          </>
        )}
      </div>
    );
  }

  if (variant === 'inline') {
    return (
      <span className="text-sm text-muted-foreground">
        {isInteractive ? (
          <>
            <Eye className="w-3.5 h-3.5 inline mr-1" />
            Interactive
          </>
        ) : isSingleAgent ? (
          <>
            <User className="w-3.5 h-3.5 inline mr-1" />
            Single Agent
            {agentName && <span className="ml-1">({capitalizeFirst(agentName)})</span>}
          </>
        ) : (
          <>
            <Users className="w-3.5 h-3.5 inline mr-1" />
            Multi-Agent
          </>
        )}
      </span>
    );
  }

  // Default: badge variant
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${
      isInteractive
        ? 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
        : isSingleAgent
          ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
          : 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
    }`}>
      {isInteractive ? (
        <>
          <Eye className="w-3 h-3" />
          Interactive
        </>
      ) : isSingleAgent ? (
        <>
          <Zap className="w-3 h-3" />
          Single Agent
          {agentName && <span className="opacity-75">• {capitalizeFirst(agentName)}</span>}
        </>
      ) : (
        <>
          <Users className="w-3 h-3" />
          Multi-Agent Consensus
        </>
      )}
    </span>
  );
}

function capitalizeFirst(str) {
  if (!str) return '';
  return str.charAt(0).toUpperCase() + str.slice(1);
}
