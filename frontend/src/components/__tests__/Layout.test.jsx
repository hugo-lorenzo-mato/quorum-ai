import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import Layout from '../Layout';
import { useUIStore } from '../../stores';

const baseState = {
  sidebarOpen: true,
  theme: 'light',
  connectionMode: 'sse',
  retrySSEFn: null,
};

const renderLayout = (path = '/') => {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Layout>
        <div>Content</div>
      </Layout>
    </MemoryRouter>
  );
};

describe('Layout', () => {
  beforeEach(() => {
    useUIStore.setState(baseState);
  });

  it('renders the active navigation label in the header', () => {
    renderLayout('/workflows');

    expect(screen.getByRole('heading', { level: 1, name: 'Workflows' })).toBeInTheDocument();
    expect(screen.getByText('Content')).toBeInTheDocument();
  });

  it('renders connection status with retry action when polling', () => {
    const retrySSEFn = vi.fn();
    useUIStore.setState({ connectionMode: 'polling', retrySSEFn });

    renderLayout('/');

    expect(screen.getByText('Polling')).toBeInTheDocument();
    fireEvent.click(screen.getByTitle('Retry connection'));
    expect(retrySSEFn).toHaveBeenCalledTimes(1);
  });

  it('hides the brand label when the sidebar is collapsed', () => {
    useUIStore.setState({ sidebarOpen: false });

    renderLayout('/');

    expect(screen.queryByText('Quorum AI')).not.toBeInTheDocument();
  });
});
