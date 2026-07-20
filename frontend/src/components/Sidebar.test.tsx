import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Sidebar } from './Sidebar';
import { makeScript } from '../test/fixtures';

describe('Sidebar script progress', () => {
  it('omits the 劇本進度 block when no script is present', () => {
    render(<Sidebar page="table" setPage={() => {}} />);
    expect(screen.queryByLabelText('劇本進度')).not.toBeInTheDocument();
  });

  it('renders stage, node title, progress and alignment when a script is running', () => {
    render(<Sidebar page="table" setPage={() => {}} script={makeScript({ stage: '中期', nodeTitle: '灰燼議會', alignment: 3, visitedCount: 4, totalNodes: 12 })} />);
    const block = screen.getByLabelText('劇本進度');
    expect(block).toBeVisible();
    expect(screen.getByText('中期')).toBeVisible();
    expect(screen.getByText('灰燼議會')).toBeVisible();
    expect(screen.getByText('5/12 節點')).toBeVisible();
    expect(screen.getByText('命運傾向・光明')).toBeVisible();
    expect(screen.queryByText(/結局/)).not.toBeInTheDocument();
  });

  it('shows the ending badge when the script has ended', () => {
    render(<Sidebar page="table" setPage={() => {}} script={makeScript({ stage: '結局', ended: true, ending: 'bad', alignment: -4, visitedCount: 12, totalNodes: 12 })} />);
    expect(screen.getByText('沉沒結局')).toBeVisible();
    expect(screen.getByText('命運傾向・幽暗')).toBeVisible();
  });
});
