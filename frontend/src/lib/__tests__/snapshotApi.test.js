import { beforeEach, describe, expect, it, vi } from 'vitest';
import { snapshotApi } from '../api';
import useProjectStore from '../../stores/projectStore';

describe('snapshotApi', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn();
    useProjectStore.setState({ currentProjectId: 'proj-123' });
  });

  it('calls export endpoint without project query parameter', async () => {
    globalThis.fetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ output_path: '/tmp/snapshot.tar.gz' }),
    });

    await snapshotApi.export({ output_path: '/tmp/snapshot.tar.gz' });

    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/snapshots/export',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('calls import endpoint with payload', async () => {
    globalThis.fetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ dry_run: true }),
    });

    await snapshotApi.import({ input_path: '/tmp/snapshot.tar.gz', dry_run: true });

    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/snapshots/import',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ input_path: '/tmp/snapshot.tar.gz', dry_run: true }),
      })
    );
  });

  it('calls validate endpoint with input path', async () => {
    globalThis.fetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ project_count: 1 }),
    });

    await snapshotApi.validate('/tmp/snapshot.tar.gz');

    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/v1/snapshots/validate',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ input_path: '/tmp/snapshot.tar.gz' }),
      })
    );
  });
});
