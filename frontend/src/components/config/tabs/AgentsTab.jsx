import { AgentCard } from '../AgentCard';

const AGENTS = ['claude', 'codex', 'gemini', 'copilot'];

export function AgentsTab() {
  return (
    <div className="space-y-6">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
          AI Agents
        </h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Configure the AI agents available for workflow execution. Enable at least one agent.
        </p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {AGENTS.map((agentKey) => (
          <AgentCard key={agentKey} agentKey={agentKey} />
        ))}
      </div>

      <AgentWarnings />
    </div>
  );
}

function AgentWarnings() {
  // This component could show warnings based on agent configuration
  // For example: "No agents enabled" or "Consider enabling multiple agents for redundancy"
  return null; // Warnings handled by validation system
}

export default AgentsTab;
