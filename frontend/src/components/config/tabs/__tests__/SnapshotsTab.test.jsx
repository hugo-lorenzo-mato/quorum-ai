import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SnapshotsTab from '../SnapshotsTab';

const { mockSnapshotApi } = vi.hoisted(() => ({
  mockSnapshotApi: {
    export: vi.fn(),
    import: vi.fn(),
    validate: vi.fn(),
  },
}));

vi.mock('../../../../lib/api', () => ({
  snapshotApi: mockSnapshotApi,
}));

describe('SnapshotsTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSnapshotApi.export.mockResolvedValue({ output_path: '/tmp/quorum-snapshot.tar.gz' });
    mockSnapshotApi.import.mockResolvedValue({ dry_run: true, projects: [] });
    mockSnapshotApi.validate.mockResolvedValue({ project_count: 1, files: [] });
  });

  it('renders snapshot sections', () => {
    render(<SnapshotsTab />);

    expect(screen.getByRole('heading', { level: 3, name: 'Snapshot Export' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 3, name: 'Snapshot Validate' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 3, name: 'Snapshot Import' })).toBeInTheDocument();
  });

  it('exports snapshot with parsed project IDs', async () => {
    render(<SnapshotsTab />);

    const idsInput = screen.getByLabelText('Project IDs (optional)');
    await userEvent.type(idsInput, 'proj-a, proj-b');

    await userEvent.click(screen.getByRole('button', { name: 'Export Snapshot' }));

    await waitFor(() => {
      expect(mockSnapshotApi.export).toHaveBeenCalledWith(
        expect.objectContaining({
          project_ids: ['proj-a', 'proj-b'],
        })
      );
    });

    expect(screen.getByText(/Snapshot exported/i)).toBeInTheDocument();
  });

  it('validates and imports snapshot', async () => {
    render(<SnapshotsTab />);

    const validateInput = document.querySelector('#snapshot-validate-path');
    const importInput = document.querySelector('#snapshot-import-path');
    expect(validateInput).toBeTruthy();
    expect(importInput).toBeTruthy();

    await userEvent.type(validateInput, '/tmp/snapshot.tar.gz');
    await userEvent.click(screen.getByRole('button', { name: 'Validate Snapshot' }));

    await waitFor(() => {
      expect(mockSnapshotApi.validate).toHaveBeenCalledWith('/tmp/snapshot.tar.gz');
    });

    await userEvent.type(importInput, '/tmp/snapshot.tar.gz');
    await userEvent.click(screen.getByRole('button', { name: 'Import Snapshot' }));

    await waitFor(() => {
      expect(mockSnapshotApi.import).toHaveBeenCalledWith(
        expect.objectContaining({
          input_path: '/tmp/snapshot.tar.gz',
          dry_run: true,
        })
      );
    });
  });
});
