import { useEffect } from 'react';
import { useConfigField } from '../../../hooks/useConfigField';
import {
  SettingSection,
  TextInputSetting,
  ToggleSetting,
} from '../index';

export function GitTab() {
  return (
    <div className="space-y-6">
      <GitAutomationSection />
      <GitNamingSection />
      <GitHubSection />
    </div>
  );
}

function GitAutomationSection() {
  const autoCommit = useConfigField('git.auto_commit');
  const autoPush = useConfigField('git.auto_push');
  const autoPr = useConfigField('git.auto_pr');
  const autoMerge = useConfigField('git.auto_merge');

  // Handle dependency chain: when a toggle is disabled, disable all dependents
  useEffect(() => {
    if (!autoCommit.value && autoPush.value) {
      autoPush.onChange(false);
    }
  }, [autoCommit.value]);

  useEffect(() => {
    if (!autoPush.value && autoPr.value) {
      autoPr.onChange(false);
    }
  }, [autoPush.value]);

  useEffect(() => {
    if (!autoPr.value && autoMerge.value) {
      autoMerge.onChange(false);
    }
  }, [autoPr.value]);

  return (
    <SettingSection
      title="Git Automation"
      description="Configure automatic git operations after task completion"
    >
      {/* Dependency Chain Visualization */}
      <div className="mb-4 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
        <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
          <span className={autoCommit.value ? 'text-green-600 dark:text-green-400 font-medium' : ''}>
            Commit
          </span>
          <span>→</span>
          <span className={autoPush.value ? 'text-green-600 dark:text-green-400 font-medium' : ''}>
            Push
          </span>
          <span>→</span>
          <span className={autoPr.value ? 'text-green-600 dark:text-green-400 font-medium' : ''}>
            PR
          </span>
          <span>→</span>
          <span className={autoMerge.value ? 'text-green-600 dark:text-green-400 font-medium' : ''}>
            Merge
          </span>
        </div>
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-500">
          Each step requires the previous step to be enabled
        </p>
      </div>

      <ToggleSetting
        label="Auto Commit"
        description="Commit changes after successful task execution"
        tooltip="When enabled, automatically creates a git commit after each successful task completion."
        checked={autoCommit.value}
        onChange={autoCommit.onChange}
        error={autoCommit.error}
        disabled={autoCommit.disabled}
      />

      <ToggleSetting
        label="Auto Push"
        description="Push commits to remote repository"
        tooltip="When enabled, automatically pushes commits to the remote after committing. Requires auto_commit."
        checked={autoPush.value}
        onChange={autoPush.onChange}
        error={autoPush.error}
        disabled={autoPush.disabled || !autoCommit.value}
        helperText={!autoCommit.value ? "Enable 'Auto Commit' first" : undefined}
      />

      <ToggleSetting
        label="Auto PR"
        description="Create pull request after push"
        tooltip="When enabled, automatically creates a pull request after pushing. Requires auto_push."
        checked={autoPr.value}
        onChange={autoPr.onChange}
        error={autoPr.error}
        disabled={autoPr.disabled || !autoPush.value}
        helperText={!autoPush.value ? "Enable 'Auto Push' first" : undefined}
      />

      <ToggleSetting
        label="Auto Merge"
        description="Merge PR if all checks pass"
        tooltip="When enabled, automatically merges the PR once all status checks pass. Requires auto_pr."
        checked={autoMerge.value}
        onChange={autoMerge.onChange}
        error={autoMerge.error}
        disabled={autoMerge.disabled || !autoPr.value}
        helperText={!autoPr.value ? "Enable 'Auto PR' first" : undefined}
      />
    </SettingSection>
  );
}

function GitNamingSection() {
  const commitPrefix = useConfigField('git.commit_prefix');
  const branchPrefix = useConfigField('git.branch_prefix');

  return (
    <SettingSection
      title="Naming Conventions"
      description="Configure prefixes for commits and branches"
    >
      <TextInputSetting
        label="Commit Prefix"
        tooltip="Prefix added to all auto-generated commit messages. Leave empty for no prefix."
        placeholder="[quorum]"
        value={commitPrefix.value}
        onChange={commitPrefix.onChange}
        error={commitPrefix.error}
        disabled={commitPrefix.disabled}
      />

      <TextInputSetting
        label="Branch Prefix"
        tooltip="Prefix for auto-created branches. Example: 'quorum/' creates branches like 'quorum/task-123'."
        placeholder="quorum/"
        value={branchPrefix.value}
        onChange={branchPrefix.onChange}
        error={branchPrefix.error}
        disabled={branchPrefix.disabled}
      />
    </SettingSection>
  );
}

function GitHubSection() {
  const owner = useConfigField('github.owner');
  const repo = useConfigField('github.repo');

  // These are typically read from git remote and may be read-only
  const hasGitHub = owner.value && repo.value;

  return (
    <SettingSection
      title="GitHub Repository"
      description="GitHub repository information (detected from git remote)"
    >
      {hasGitHub ? (
        <div className="p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
          <div className="flex items-center gap-2">
            <svg className="w-5 h-5 text-gray-700 dark:text-gray-300" fill="currentColor" viewBox="0 0 24 24">
              <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
            </svg>
            <a
              href={`https://github.com/${owner.value}/${repo.value}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 dark:text-blue-400 hover:underline font-medium"
            >
              {owner.value}/{repo.value}
            </a>
          </div>
        </div>
      ) : (
        <div className="p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
          <p className="text-sm text-yellow-700 dark:text-yellow-300">
            No GitHub repository detected. Make sure you have a remote named 'origin' pointing to GitHub.
          </p>
        </div>
      )}

      <TextInputSetting
        label="Owner"
        tooltip="GitHub repository owner (username or organization)."
        placeholder="owner"
        value={owner.value}
        onChange={owner.onChange}
        error={owner.error}
        disabled={owner.disabled}
      />

      <TextInputSetting
        label="Repository"
        tooltip="GitHub repository name."
        placeholder="repo"
        value={repo.value}
        onChange={repo.onChange}
        error={repo.error}
        disabled={repo.disabled}
      />
    </SettingSection>
  );
}

export default GitTab;
