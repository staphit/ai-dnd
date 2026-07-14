import assert from 'node:assert/strict';
import test from 'node:test';
import { codexModelOptions, normalizeCodexModel } from '../codex-exec.mjs';

test('allows documented Codex model choices and preserves the default', () => {
  assert.equal(normalizeCodexModel(''), process.env.CODEX_MODEL?.trim() || '');
  assert.equal(normalizeCodexModel('gpt-5.6-terra'), 'gpt-5.6-terra');
  assert.ok(codexModelOptions.some((model) => model.id === 'gpt-5.6-sol'));
});

test('rejects arbitrary model arguments', () => {
  assert.throws(() => normalizeCodexModel('--dangerously-bypass-approvals-and-sandbox'), /不支援/);
});
