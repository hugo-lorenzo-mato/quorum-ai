import { useConfigField, useConfigSelect } from '../../../hooks/useConfigField';
import {
  SettingSection,
  SelectSetting,
  TextInputSetting,
  ToggleSetting,
  ArrayInputSetting,
} from '../index';

export function IssuesTab() {
  return (
    <div className="space-y-6">
      <GeneralIssuesSection />
      <TemplateSection />
      <LabelsSection />
      <GitLabSection />
    </div>
  );
}

function GeneralIssuesSection() {
  const enabled = useConfigField('issues.enabled');
  const provider = useConfigSelect('issues.provider', 'issue_providers');
  const autoGenerate = useConfigField('issues.auto_generate');

  return (
    <SettingSection
      title="Issue Generation"
      description="Configure automatic issue creation from workflow artifacts"
    >
      <ToggleSetting
        label="Enable Issue Generation"
        description="Allow generating GitHub/GitLab issues from workflow results"
        tooltip="When enabled, you can generate issues from the workflow detail page after execution completes."
        checked={enabled.value}
        onChange={enabled.onChange}
        error={enabled.error}
        disabled={enabled.disabled}
      />

      <SelectSetting
        label="Provider"
        description="Issue tracking platform"
        tooltip="Select GitHub or GitLab as your issue tracking provider."
        value={provider.value || 'github'}
        onChange={provider.onChange}
        options={provider.options.length > 0 ? provider.options : [
          { value: 'github', label: 'GitHub' },
          { value: 'gitlab', label: 'GitLab' },
        ]}
        error={provider.error}
        disabled={provider.disabled || !enabled.value}
      />

      <ToggleSetting
        label="Auto Generate"
        description="Automatically generate issues after workflow completion"
        tooltip="When enabled, issues will be created automatically when a workflow completes successfully."
        checked={autoGenerate.value}
        onChange={autoGenerate.onChange}
        error={autoGenerate.error}
        disabled={autoGenerate.disabled || !enabled.value}
      />
    </SettingSection>
  );
}

function TemplateSection() {
  const enabled = useConfigField('issues.enabled');
  const language = useConfigSelect('issues.template.language', 'template_languages');
  const tone = useConfigSelect('issues.template.tone', 'template_tones');
  const titleFormat = useConfigField('issues.template.title_format');
  const includeDiagrams = useConfigField('issues.template.include_diagrams');
  const customInstructions = useConfigField('issues.template.custom_instructions');

  const isDisabled = !enabled.value;

  return (
    <SettingSection
      title="Issue Template"
      description="Customize how issues are formatted and written"
    >
      <SelectSetting
        label="Language"
        description="Language for generated issue content"
        tooltip="The language in which issue titles and descriptions will be written."
        value={language.value || 'english'}
        onChange={language.onChange}
        options={language.options.length > 0 ? language.options : [
          { value: 'english', label: 'English' },
          { value: 'spanish', label: 'Spanish' },
          { value: 'french', label: 'French' },
          { value: 'german', label: 'German' },
          { value: 'portuguese', label: 'Portuguese' },
          { value: 'chinese', label: 'Chinese' },
          { value: 'japanese', label: 'Japanese' },
        ]}
        error={language.error}
        disabled={language.disabled || isDisabled}
      />

      <SelectSetting
        label="Tone"
        description="Writing style for issue descriptions"
        tooltip="Controls the formality and style of the generated issue text."
        value={tone.value || 'professional'}
        onChange={tone.onChange}
        options={tone.options.length > 0 ? tone.options : [
          { value: 'professional', label: 'Professional' },
          { value: 'casual', label: 'Casual' },
          { value: 'technical', label: 'Technical' },
          { value: 'concise', label: 'Concise' },
        ]}
        error={tone.error}
        disabled={tone.disabled || isDisabled}
      />

      <TextInputSetting
        label="Title Format"
        description="Template for issue titles"
        tooltip="Use placeholders: {workflow_title}, {task_title}, {task_id}. Example: '[Quorum] {task_title}'"
        placeholder="[Quorum] {task_title}"
        value={titleFormat.value}
        onChange={titleFormat.onChange}
        error={titleFormat.error}
        disabled={titleFormat.disabled || isDisabled}
      />

      <ToggleSetting
        label="Include Diagrams"
        description="Add Mermaid diagrams to issues when available"
        tooltip="When enabled, any architecture or flow diagrams from the workflow will be included in the issue body."
        checked={includeDiagrams.value}
        onChange={includeDiagrams.onChange}
        error={includeDiagrams.error}
        disabled={includeDiagrams.disabled || isDisabled}
      />

      <div className="py-3">
        <div className="flex items-center gap-2 mb-2">
          <label className={`text-sm font-medium ${isDisabled ? 'text-muted-foreground' : 'text-foreground'}`}>
            Custom Instructions
          </label>
        </div>
        <p className="text-xs text-muted-foreground mb-2">
          Additional instructions for issue generation (appended to default template)
        </p>
        <textarea
          value={customInstructions.value || ''}
          onChange={(e) => customInstructions.onChange(e.target.value)}
          disabled={customInstructions.disabled || isDisabled}
          placeholder="Add any specific formatting requirements, sections to include, or custom context..."
          rows={4}
          className={`
            w-full px-3 py-2
            border rounded-lg bg-background text-foreground
            transition-colors resize-y
            focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
            disabled:opacity-50 disabled:cursor-not-allowed
            placeholder:text-muted-foreground
            ${customInstructions.error ? 'border-error' : 'border-input hover:border-muted-foreground'}
          `}
        />
      </div>
    </SettingSection>
  );
}

function LabelsSection() {
  const enabled = useConfigField('issues.enabled');
  const defaultLabels = useConfigField('issues.default_labels');
  const defaultAssignees = useConfigField('issues.default_assignees');

  const isDisabled = !enabled.value;

  return (
    <SettingSection
      title="Labels & Assignees"
      description="Default labels and assignees for generated issues"
    >
      <ArrayInputSetting
        label="Default Labels"
        description="Labels automatically applied to all generated issues"
        tooltip="These labels will be added to every issue. You can add more during issue preview."
        value={defaultLabels.value || []}
        onChange={defaultLabels.onChange}
        error={defaultLabels.error}
        disabled={defaultLabels.disabled || isDisabled}
        placeholder="Add label (e.g., 'quorum-generated')..."
      />

      <ArrayInputSetting
        label="Default Assignees"
        description="Users automatically assigned to generated issues"
        tooltip="GitHub/GitLab usernames to assign to issues by default."
        value={defaultAssignees.value || []}
        onChange={defaultAssignees.onChange}
        error={defaultAssignees.error}
        disabled={defaultAssignees.disabled || isDisabled}
        placeholder="Add username..."
      />
    </SettingSection>
  );
}

function GitLabSection() {
  const enabled = useConfigField('issues.enabled');
  const provider = useConfigField('issues.provider');
  const useEpics = useConfigField('issues.gitlab.use_epics');
  const projectId = useConfigField('issues.gitlab.project_id');

  const isGitLab = provider.value === 'gitlab';
  const isDisabled = !enabled.value || !isGitLab;

  if (!isGitLab) {
    return null;
  }

  return (
    <SettingSection
      title="GitLab Settings"
      description="GitLab-specific configuration options"
    >
      <ToggleSetting
        label="Use Epics"
        description="Create main issue as an Epic with linked child issues"
        tooltip="When enabled, the main workflow issue becomes an Epic with sub-issues linked as children."
        checked={useEpics.value}
        onChange={useEpics.onChange}
        error={useEpics.error}
        disabled={useEpics.disabled || isDisabled}
      />

      <TextInputSetting
        label="Project ID"
        description="GitLab project ID (if different from auto-detected)"
        tooltip="Override the automatically detected project ID. Leave empty to use auto-detection."
        placeholder="12345678"
        value={projectId.value}
        onChange={projectId.onChange}
        error={projectId.error}
        disabled={projectId.disabled || isDisabled}
      />
    </SettingSection>
  );
}

export default IssuesTab;
