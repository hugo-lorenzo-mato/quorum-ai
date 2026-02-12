import { useMemo, useState } from 'react';
import { SettingSection } from '../index';
import { snapshotApi } from '../../../lib/api';

function buildDefaultExportPath() {
  const now = new Date();
  const pad = (v) => String(v).padStart(2, '0');
  const ts = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
  return `./quorum-snapshot-${ts}.tar.gz`;
}

function parsePathMap(value) {
  const lines = value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);

  const pathMap = {};
  for (const line of lines) {
    const parts = line.split('=');
    if (parts.length !== 2) {
      throw new Error(`Invalid path map entry: ${line}. Use old-path=new-path`);
    }
    const from = parts[0].trim();
    const to = parts[1].trim();
    if (!from || !to) {
      throw new Error(`Invalid path map entry: ${line}. Both paths are required`);
    }
    pathMap[from] = to;
  }

  return pathMap;
}

function parseProjectIDs(value) {
  return value
    .split(',')
    .map((id) => id.trim())
    .filter(Boolean);
}

export default function SnapshotsTab() {
  const [exportPath, setExportPath] = useState(buildDefaultExportPath);
  const [exportProjectIDs, setExportProjectIDs] = useState('');
  const [exportIncludeWorktrees, setExportIncludeWorktrees] = useState(false);

  const [importPath, setImportPath] = useState('');
  const [importMode, setImportMode] = useState('merge');
  const [importConflictPolicy, setImportConflictPolicy] = useState('skip');
  const [importDryRun, setImportDryRun] = useState(true);
  const [importPreserveIDs, setImportPreserveIDs] = useState(false);
  const [importIncludeWorktrees, setImportIncludeWorktrees] = useState(true);
  const [importPathMap, setImportPathMap] = useState('');

  const [validatePath, setValidatePath] = useState('');

  const [isExporting, setIsExporting] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [isValidating, setIsValidating] = useState(false);

  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [result, setResult] = useState(null);

  const isBusy = isExporting || isImporting || isValidating;

  const canExport = useMemo(() => exportPath.trim().length > 0, [exportPath]);
  const canImport = useMemo(() => importPath.trim().length > 0, [importPath]);
  const canValidate = useMemo(() => validatePath.trim().length > 0, [validatePath]);

  const clearFeedback = () => {
    setError('');
    setStatus('');
  };

  const handleExport = async () => {
    clearFeedback();
    setIsExporting(true);
    try {
      const payload = {
        output_path: exportPath.trim(),
        include_worktrees: exportIncludeWorktrees,
      };
      const ids = parseProjectIDs(exportProjectIDs);
      if (ids.length > 0) {
        payload.project_ids = ids;
      }

      const response = await snapshotApi.export(payload);
      setResult(response);
      setStatus(`Snapshot exported to ${response?.output_path || exportPath.trim()}`);
    } catch (err) {
      setError(err.message || 'Snapshot export failed');
    } finally {
      setIsExporting(false);
    }
  };

  const handleImport = async () => {
    clearFeedback();
    setIsImporting(true);
    try {
      const payload = {
        input_path: importPath.trim(),
        mode: importMode,
        conflict_policy: importConflictPolicy,
        dry_run: importDryRun,
        preserve_project_ids: importPreserveIDs,
        include_worktrees: importIncludeWorktrees,
      };

      const pathMap = parsePathMap(importPathMap);
      if (Object.keys(pathMap).length > 0) {
        payload.path_map = pathMap;
      }

      const response = await snapshotApi.import(payload);
      setResult(response);
      setStatus(importDryRun ? 'Dry-run import completed' : 'Snapshot import completed');
    } catch (err) {
      setError(err.message || 'Snapshot import failed');
    } finally {
      setIsImporting(false);
    }
  };

  const handleValidate = async () => {
    clearFeedback();
    setIsValidating(true);
    try {
      const response = await snapshotApi.validate(validatePath.trim());
      setResult(response);
      setStatus('Snapshot is valid');
    } catch (err) {
      setError(err.message || 'Snapshot validation failed');
    } finally {
      setIsValidating(false);
    }
  };

  return (
    <div className="space-y-6">
      <SettingSection
        title="Snapshot Export"
        description="Create a portable backup of registry metadata and project state files."
      >
        <div className="space-y-4">
          <div>
            <label htmlFor="snapshot-export-path" className="block text-sm font-medium text-foreground mb-1">Output Path</label>
            <input
              id="snapshot-export-path"
              type="text"
              value={exportPath}
              onChange={(e) => setExportPath(e.target.value)}
              placeholder="./quorum-snapshot-YYYYMMDD-HHMMSS.tar.gz"
              className="w-full px-3 py-2 border rounded-lg bg-background text-foreground border-input hover:border-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
            />
          </div>

          <div>
            <label htmlFor="snapshot-export-project-ids" className="block text-sm font-medium text-foreground mb-1">Project IDs (optional)</label>
            <input
              id="snapshot-export-project-ids"
              type="text"
              value={exportProjectIDs}
              onChange={(e) => setExportProjectIDs(e.target.value)}
              placeholder="proj-aaa,proj-bbb"
              className="w-full px-3 py-2 border rounded-lg bg-background text-foreground border-input hover:border-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
            />
          </div>

          <label className="flex items-center gap-2 text-sm text-foreground">
            <input
              type="checkbox"
              checked={exportIncludeWorktrees}
              onChange={(e) => setExportIncludeWorktrees(e.target.checked)}
              className="rounded border-input"
            />
            Include .worktrees directories
          </label>

          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleExport}
              disabled={!canExport || isBusy}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:pointer-events-none"
            >
              {isExporting ? 'Exporting...' : 'Export Snapshot'}
            </button>
          </div>
        </div>
      </SettingSection>

      <SettingSection
        title="Snapshot Validate"
        description="Verify archive integrity and checksums before importing."
      >
        <div className="space-y-4">
          <div>
            <label htmlFor="snapshot-validate-path" className="block text-sm font-medium text-foreground mb-1">Snapshot Path</label>
            <input
              id="snapshot-validate-path"
              type="text"
              value={validatePath}
              onChange={(e) => setValidatePath(e.target.value)}
              placeholder="/path/to/quorum-snapshot.tar.gz"
              className="w-full px-3 py-2 border rounded-lg bg-background text-foreground border-input hover:border-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
            />
          </div>

          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleValidate}
              disabled={!canValidate || isBusy}
              className="px-4 py-2 text-sm font-medium rounded-lg border border-border bg-background hover:bg-accent disabled:opacity-50 disabled:pointer-events-none"
            >
              {isValidating ? 'Validating...' : 'Validate Snapshot'}
            </button>
          </div>
        </div>
      </SettingSection>

      <SettingSection
        title="Snapshot Import"
        description="Restore projects and registry data from a snapshot. Use dry-run first for safer migrations."
      >
        <div className="space-y-4">
          <div>
            <label htmlFor="snapshot-import-path" className="block text-sm font-medium text-foreground mb-1">Snapshot Path</label>
            <input
              id="snapshot-import-path"
              type="text"
              value={importPath}
              onChange={(e) => setImportPath(e.target.value)}
              placeholder="/path/to/quorum-snapshot.tar.gz"
              className="w-full px-3 py-2 border rounded-lg bg-background text-foreground border-input hover:border-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label htmlFor="snapshot-import-mode" className="block text-sm font-medium text-foreground mb-1">Mode</label>
              <select
                id="snapshot-import-mode"
                value={importMode}
                onChange={(e) => setImportMode(e.target.value)}
                className="w-full px-3 py-2 border rounded-lg bg-background text-foreground border-input hover:border-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
              >
                <option value="merge">merge</option>
                <option value="replace">replace</option>
              </select>
            </div>

            <div>
              <label htmlFor="snapshot-import-conflict" className="block text-sm font-medium text-foreground mb-1">Conflict Policy</label>
              <select
                id="snapshot-import-conflict"
                value={importConflictPolicy}
                onChange={(e) => setImportConflictPolicy(e.target.value)}
                className="w-full px-3 py-2 border rounded-lg bg-background text-foreground border-input hover:border-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
              >
                <option value="skip">skip</option>
                <option value="overwrite">overwrite</option>
                <option value="fail">fail</option>
              </select>
            </div>
          </div>

          <div>
            <label htmlFor="snapshot-import-path-map" className="block text-sm font-medium text-foreground mb-1">Path Map (optional)</label>
            <textarea
              id="snapshot-import-path-map"
              value={importPathMap}
              onChange={(e) => setImportPathMap(e.target.value)}
              rows={3}
              placeholder="/old/path=/new/path\n/other/old=/other/new"
              className="w-full px-3 py-2 border rounded-lg bg-background text-foreground border-input hover:border-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
            <label className="flex items-center gap-2 text-sm text-foreground">
              <input
                type="checkbox"
                checked={importDryRun}
                onChange={(e) => setImportDryRun(e.target.checked)}
                className="rounded border-input"
              />
              Dry run
            </label>
            <label className="flex items-center gap-2 text-sm text-foreground">
              <input
                type="checkbox"
                checked={importPreserveIDs}
                onChange={(e) => setImportPreserveIDs(e.target.checked)}
                className="rounded border-input"
              />
              Preserve project IDs
            </label>
            <label className="flex items-center gap-2 text-sm text-foreground">
              <input
                type="checkbox"
                checked={importIncludeWorktrees}
                onChange={(e) => setImportIncludeWorktrees(e.target.checked)}
                className="rounded border-input"
              />
              Include .worktrees
            </label>
          </div>

          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleImport}
              disabled={!canImport || isBusy}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:pointer-events-none"
            >
              {isImporting ? 'Importing...' : 'Import Snapshot'}
            </button>
          </div>
        </div>
      </SettingSection>

      {(status || error) && (
        <div className={`p-3 rounded-lg border text-sm ${error ? 'bg-destructive/10 border-destructive/20 text-destructive' : 'bg-primary/10 border-primary/20 text-foreground'}`}>
          {error || status}
        </div>
      )}

      {result && (
        <SettingSection
          title="Last Result"
          description="Most recent response from snapshot operation."
        >
          <pre className="text-xs bg-background border border-border rounded-lg p-3 overflow-auto text-foreground">
            {JSON.stringify(result, null, 2)}
          </pre>
        </SettingSection>
      )}
    </div>
  );
}
