import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import MarkdownViewer from '../MarkdownViewer';
import { useUIStore } from '../../stores';

describe('MarkdownViewer', () => {
  beforeEach(() => {
    useUIStore.setState({ theme: 'light' });
  });

  it('toggles frontmatter visibility', () => {
    const markdown = `---\ntitle: Test\n---\n# Heading`;

    render(<MarkdownViewer markdown={markdown} />);

    const metadataButton = screen.getByRole('button', { name: 'Metadata' });
    expect(screen.queryByText('title: Test')).not.toBeInTheDocument();

    fireEvent.click(metadataButton);

    expect(screen.getByText('title: Test')).toBeInTheDocument();
    expect(screen.getByText('Heading')).toBeInTheDocument();
  });

  it('skips the metadata panel when no frontmatter is provided', () => {
    render(<MarkdownViewer markdown="Just text" />);

    expect(screen.queryByRole('button', { name: 'Metadata' })).not.toBeInTheDocument();
    expect(screen.getByText('Just text')).toBeInTheDocument();
  });

  it('copies content to clipboard and shows visual feedback', async () => {
    const mockClipboard = { writeText: vi.fn().mockResolvedValue() };
    Object.assign(navigator, { clipboard: mockClipboard });

    const markdown = '# Test Content\n\nSome text here.';
    render(<MarkdownViewer markdown={markdown} />);

    const copyButton = screen.getByRole('button', { name: 'Copy content' });
    expect(copyButton).toBeInTheDocument();

    fireEvent.click(copyButton);

    expect(mockClipboard.writeText).toHaveBeenCalledWith(markdown);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument();
    });
  });
});
