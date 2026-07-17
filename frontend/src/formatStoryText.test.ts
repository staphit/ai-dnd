import { describe, expect, it } from 'vitest';
import { formatDialogueBreaks, formatStoryText, stripChoiceFooter } from './formatStoryText';

describe('formatStoryText', () => {
  it('strips legacy choice footers', () => {
    expect(stripChoiceFooter('門開了。\n\n可考慮：Imp')).toBe('門開了。');
  });

  it('puts quoted speech on its own lines without relying on the model', () => {
    const raw = '托文抬起頭說：「別碰那道綠光。」接著他指向西側。';
    const out = formatDialogueBreaks(raw);
    expect(out).toContain('\n「別碰那道綠光。」\n');
    expect(out.split('\n').length).toBeGreaterThanOrEqual(2);
  });

  it('formats end-to-end story text', () => {
    const out = formatStoryText('空氣變冷。「誰在那？」低語從梁後傳來。\n\n可考慮：搜索');
    expect(out).not.toContain('可考慮');
    expect(out).toMatch(/「誰在那？」/);
    expect(out.includes('\n')).toBe(true);
  });
});
