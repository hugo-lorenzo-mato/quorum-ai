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

  it('copies code block content to clipboard', async () => {
    const mockClipboard = { writeText: vi.fn().mockResolvedValue() };
    Object.assign(navigator, { clipboard: mockClipboard });

    const codeContent = 'const x = 1;';
    const markdown = '```javascript\n' + codeContent + '\n```';
    render(<MarkdownViewer markdown={markdown} />);

    const copyButton = screen.getByRole('button', { name: 'Copy code' });
    expect(copyButton).toBeInTheDocument();

    fireEvent.click(copyButton);

    expect(mockClipboard.writeText).toHaveBeenCalledWith(codeContent);

    await waitFor(() => {
      // We look for the button that changed state. 
      // Note: The global button is still "Copy content", so "Copied!" should be unique if we only clicked one.
      expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument();
    });
  });

  it('renders markdown tables with css grid rows for comparison layouts', () => {
    const markdown = [
      '| Feature | Claude | Codex |',
      '| --- | --- | --- |',
      '| Context handling | Strong | Strong |',
      '| Tool execution | Limited | Strong |',
    ].join('\n');

    render(<MarkdownViewer markdown={markdown} />);

    const table = screen.getByRole('table');
    const rows = table.querySelectorAll('tr');
    expect(rows.length).toBe(3);
    expect(rows[0]).toHaveClass('grid');
    expect(rows[1].style.gridTemplateColumns).toContain('repeat(2');
    expect(screen.getByText('Context handling')).toBeInTheDocument();
    expect(screen.getByText('Tool execution')).toBeInTheDocument();
  });
});
