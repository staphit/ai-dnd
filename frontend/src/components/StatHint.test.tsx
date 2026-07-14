import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StatHint } from './StatHint';

describe('StatHint', () => {
  it('exposes an accessible rules explanation when enabled', () => {
    render(<StatHint hint="dex">敏捷</StatHint>);
    expect(screen.getByText('敏捷')).toHaveAttribute('aria-describedby');
    expect(screen.getByRole('tooltip')).toHaveTextContent('影響先攻');
  });

  it('removes the tooltip and focus target when disabled', () => {
    render(<StatHint hint="ac" enabled={false}>AC 16</StatHint>);
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    expect(screen.getByText('AC 16')).not.toHaveAttribute('tabindex');
  });
});
