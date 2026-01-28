import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { SettingSection } from '../SettingSection';

describe('SettingSection', () => {
  it('renders title and description', () => {
    render(
      <SettingSection title="Test Title" description="Test description">
        <div>Content</div>
      </SettingSection>
    );

    expect(screen.getByText('Test Title')).toBeInTheDocument();
    expect(screen.getByText('Test description')).toBeInTheDocument();
  });

  it('renders children', () => {
    render(
      <SettingSection title="Test">
        <div data-testid="child">Child content</div>
      </SettingSection>
    );

    expect(screen.getByTestId('child')).toBeInTheDocument();
  });

  it('applies danger variant styles', () => {
    const { container } = render(
      <SettingSection title="Danger" variant="danger">
        <div>Content</div>
      </SettingSection>
    );

    // Check for danger styling on the section element
    const section = container.querySelector('section');
    expect(section).toHaveClass('border-red-200');
  });
});
