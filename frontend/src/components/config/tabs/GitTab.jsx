import { useEffect } from 'react';
import { useConfigField, useConfigSelect } from '../../../hooks/useConfigField';
import {
  SettingSection,
  TextInputSetting,
  SelectSetting,
  ToggleSetting,
} from '../index';

export function GitTab() {
  return (
    <div className="space-y-6">
      <WorktreeSection />
      <TaskProgressSection />
      <FinalizationSection />
      <GitHubSection />
    </div>
  );
}

function WorktreeSection() {
  const dir = useConfigField('git.worktree.dir');
  const mode = useConfigSelect('git.worktree.mode', 'worktree_modes');
  const autoClean = useConfigField('git.worktree.auto_clean');

  const isDisabled = mode.value === 'disabled';

  return (
    <SettingSection
      title="Worktree Management"
      description="Configure temporary worktrees for task isolation during execution"
    >
      <SelectSetting
        label="Worktree Mode"
        description="When to create isolated worktrees for task execution"
        tooltip="always: Every task gets its own worktree. parallel: Only when 2+ tasks can run concurrently (recommended). disabled: All tasks run in the main working directory."
        value={mode.value || 'parallel'}
        onChange={mode.onChange}
        options={mode.options.length > 0 ? mode.options : [
          { value: 'always', label: 'Always' },
          { value: 'parallel', label: 'Parallel (Recommended)' },
          { value: 'disabled', label: 'Disabled' },
        ]}
        error={mode.error}
        disabled={mode.disabled}
      />

      <TextInputSetting
        label="Worktree Directory"
        description="Directory where worktrees are created"
        tooltip="Relative path from project root. Default: .worktrees"
        placeholder=".worktrees"
        value={dir.value || ''}
        onChange={dir.onChange}
        error={dir.error}
        disabled={dir.disabled || isDisabled}
      />

      <ToggleSetting
        label="Auto Clean"
        description="Remove worktrees after task completion"
        tooltip="When enabled, worktrees are automatically deleted after the task completes. Requires auto_commit to be enabled to prevent data loss."
        checked={autoClean.value}
        onChange={autoClean.onChange}
        error={autoClean.error}
        disabled={autoClean.disabled || isDisabled}
      />

      {autoClean.value && !isDisabled && (
        <div className="p-3 bg-warning/10 border border-warning/20 rounded-lg">
          <p className="text-sm text-foreground">
            <strong className="text-warning">Important:</strong> Auto Clean requires Task Auto Commit to be enabled to prevent data loss.
          </p>
        </div>
      )}
    </SettingSection>
  );
}

function TaskProgressSection() {
  const autoCommit = useConfigField('git.task.auto_commit');

  return (
    <SettingSection
      title="Task Progress"
      description="Configure how task progress is saved during workflow execution"
    >
      <ToggleSetting
        label="Auto Commit"
        description="Commit changes after each task completes"
        tooltip="When enabled, automatically creates a git commit after each successful task completion. This saves work even if the workflow crashes later."
        checked={autoCommit.value}
        onChange={autoCommit.onChange}
        error={autoCommit.error}
        disabled={autoCommit.disabled}
      />
    </SettingSection>
  );
}

function FinalizationSection() {
  const autoCommit = useConfigField('git.task.auto_commit');
  const autoPush = useConfigField('git.finalization.auto_push');
  const autoPr = useConfigField('git.finalization.auto_pr');
  const autoMerge = useConfigField('git.finalization.auto_merge');
  const prBaseBranch = useConfigField('git.finalization.pr_base_branch');
  const mergeStrategy = useConfigSelect('git.finalization.merge_strategy', 'merge_strategies');

  const { value: autoPushValue, onChange: setAutoPush } = autoPush;
  const { value: autoPrValue, onChange: setAutoPr } = autoPr;
  const { value: autoMergeValue, onChange: setAutoMerge } = autoMerge;

  // Handle dependency chain: when a toggle is disabled, disable all dependents
  useEffect(() => {
    if (!autoPushValue && autoPrValue) {
      setAutoPr(false);
    }
  }, [autoPushValue, autoPrValue, setAutoPr]);

  useEffect(() => {
    if (!autoPrValue && autoMergeValue) {
      setAutoMerge(false);
    }
  }, [autoPrValue, autoMergeValue, setAutoMerge]);

  return (
    <SettingSection
      title="Workflow Finalization"
      description="Configure how completed workflows are delivered to the remote repository"
    >
      {/* Dependency Chain Visualization */}
      <div className="mb-4 p-3 bg-muted border border-border rounded-lg">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span className={autoCommit.value ? 'text-success font-medium' : ''}>
            Commit
          </span>
          <span>→</span>
          <span className={autoPush.value ? 'text-success font-medium' : ''}>
            Push
          </span>
          <span>→</span>
          <span className={autoPr.value ? 'text-success font-medium' : ''}>
            PR
          </span>
          <span>→</span>
          <span className={autoMerge.value ? 'text-success font-medium' : ''}>
            Merge
          </span>
        </div>
        <p className="mt-1 text-xs text-muted-foreground">
          Each step requires the previous step to be enabled
        </p>
      </div>

      <ToggleSetting
        label="Auto Push"
        description="Push workflow branch to remote"
        tooltip="When enabled, automatically pushes the workflow branch to the remote repository after all tasks complete."
        checked={autoPush.value}
        onChange={autoPush.onChange}
        error={autoPush.error}
        disabled={autoPush.disabled}
      />

      <ToggleSetting
        label="Auto PR"
        description="Create pull request for the workflow"
        tooltip="When enabled, automatically creates a single pull request containing all workflow changes. Requires auto_push."
        checked={autoPr.value}
        onChange={autoPr.onChange}
        error={autoPr.error}
        disabled={autoPr.disabled || !autoPush.value}
        helperText={!autoPush.value ? "Enable 'Auto Push' first" : undefined}
      />

      <TextInputSetting
        label="PR Base Branch"
        description="Target branch for pull requests"
        tooltip="The branch to merge into. Leave empty to use the repository's default branch (main/master)."
        placeholder="(repository default)"
        value={prBaseBranch.value || ''}
        onChange={prBaseBranch.onChange}
        error={prBaseBranch.error}
        disabled={prBaseBranch.disabled || !autoPr.value}
      />

      <ToggleSetting
        label="Auto Merge"
        description="Automatically merge PR when checks pass"
        tooltip="When enabled, automatically merges the PR once all status checks pass. Disabled by default for safety (requires human review). Requires auto_pr."
        checked={autoMerge.value}
        onChange={autoMerge.onChange}
        error={autoMerge.error}
        disabled={autoMerge.disabled || !autoPr.value}
        helperText={!autoPr.value ? "Enable 'Auto PR' first" : undefined}
      />

      {autoMerge.value && (
        <div className="p-3 bg-warning/10 border border-warning/20 rounded-lg">
          <p className="text-sm text-foreground">
            <strong className="text-warning">Caution:</strong> Auto merge bypasses human review. Use with care in production environments.
          </p>
        </div>
      )}

      <SelectSetting
        label="Merge Strategy"
        description="How to merge PRs when auto merge is enabled"
        tooltip="merge: Create a merge commit. squash: Squash all commits into one. rebase: Rebase commits onto base branch."
        value={mergeStrategy.value || 'squash'}
        onChange={mergeStrategy.onChange}
        options={mergeStrategy.options.length > 0 ? mergeStrategy.options : [
          { value: 'merge', label: 'Merge Commit' },
          { value: 'squash', label: 'Squash (Recommended)' },
          { value: 'rebase', label: 'Rebase' },
        ]}
        error={mergeStrategy.error}
        disabled={mergeStrategy.disabled || !autoMerge.value}
      />
    </SettingSection>
  );
}

function GitHubSection() {
  const remote = useConfigField('github.remote');

  return (
    <SettingSection
      title="GitHub Integration"
      description="Configure GitHub remote settings"
    >
      <TextInputSetting
        label="Remote Name"
        description="Git remote name for GitHub operations"
        tooltip="The name of the git remote pointing to GitHub. Default is 'origin'."
        placeholder="origin"
        value={remote.value || ''}
        onChange={remote.onChange}
        error={remote.error}
        disabled={remote.disabled}
      />

      <div className="p-3 bg-muted/50 border border-border rounded-lg">
        <p className="text-sm text-muted-foreground">
          <strong>Note:</strong> GitHub authentication is configured via the <code className="px-1 py-0.5 bg-muted rounded text-xs">GITHUB_TOKEN</code> or <code className="px-1 py-0.5 bg-muted rounded text-xs">GH_TOKEN</code> environment variable.
        </p>
      </div>
    </SettingSection>
  );
}

export default GitTab;
