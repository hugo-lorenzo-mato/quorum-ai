import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import AgentConstellation from '../AgentConstellation';

describe('AgentConstellation', () => {
  it('returns null when agents array is empty', () => {
    const { container } = render(<AgentConstellation agents={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders an SVG with the correct aria-label', () => {
    const agents = [
      { name: 'claude', status: 'started' },
      { name: 'gemini', status: 'completed' },
    ];
    const { container } = render(<AgentConstellation agents={agents} />);
    const svg = container.querySelector('svg');
    expect(svg).toBeTruthy();
    expect(svg.getAttribute('aria-label')).toBe('Agent constellation visualization');
  });

  it('renders agent name labels', () => {
    const agents = [
      { name: 'claude', status: 'started' },
      { name: 'gemini', status: 'completed' },
    ];
    const { container } = render(<AgentConstellation agents={agents} />);
    const texts = Array.from(container.querySelectorAll('text'));
    const labels = texts.map(t => t.textContent);
    expect(labels).toContain('claude');
    expect(labels).toContain('gemini');
  });

  it('renders moderator at center when specified', () => {
    const agents = [
      { name: 'claude', status: 'started' },
      { name: 'gemini', status: 'thinking' },
      { name: 'codex', status: 'started' },
    ];
    const { container } = render(
      <AgentConstellation agents={agents} moderator="claude" round={1} />
    );
    const texts = Array.from(container.querySelectorAll('text'));
    const labels = texts.map(t => t.textContent);
    // Moderator should show "Mod" label
    expect(labels).toContain('Mod');
    // Moderator name below center
    expect(labels).toContain('claude');
    // Other agents as participant labels
    expect(labels).toContain('gemini');
    expect(labels).toContain('codex');
  });

  it('shows consensus score when provided', () => {
    const agents = [
      { name: 'claude', status: 'completed' },
      { name: 'gemini', status: 'completed' },
    ];
    const { container } = render(
      <AgentConstellation agents={agents} moderator="claude" consensusScore={0.85} round={2} />
    );
    const texts = Array.from(container.querySelectorAll('text'));
    const labels = texts.map(t => t.textContent);
    expect(labels).toContain('85%');
  });

  it('renders connection lines between agents and center', () => {
    const agents = [
      { name: 'claude', status: 'started' },
      { name: 'gemini', status: 'started' },
      { name: 'codex', status: 'started' },
    ];
    const { container } = render(<AgentConstellation agents={agents} />);
    // Lines to center (one per participant) + peer lines (3 for a triangle)
    const lines = container.querySelectorAll('line');
    expect(lines.length).toBe(6); // 3 to center + 3 inter-agent
  });

  it('applies animated dash for active agents', () => {
    const agents = [
      { name: 'claude', status: 'thinking' },
      { name: 'gemini', status: 'completed' },
    ];
    const { container } = render(<AgentConstellation agents={agents} />);
    const animatedLines = container.querySelectorAll('.animate-dash-flow');
    // Claude is active -> its connection line should animate
    expect(animatedLines.length).toBeGreaterThan(0);
  });

  it('renders round label when no moderator', () => {
    const agents = [
      { name: 'claude', status: 'started' },
      { name: 'gemini', status: 'started' },
    ];
    const { container } = render(
      <AgentConstellation agents={agents} round={3} />
    );
    const texts = Array.from(container.querySelectorAll('text'));
    const labels = texts.map(t => t.textContent);
    expect(labels).toContain('R3');
    expect(labels).toContain('consensus');
  });

  it('respects custom width and height', () => {
    const agents = [{ name: 'claude', status: 'started' }];
    const { container } = render(
      <AgentConstellation agents={agents} width={400} height={300} />
    );
    const svg = container.querySelector('svg');
    expect(svg.getAttribute('width')).toBe('400');
    expect(svg.getAttribute('height')).toBe('300');
  });
});
