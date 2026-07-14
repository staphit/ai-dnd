import { spawn } from 'node:child_process';
import process from 'node:process';
import { codexModel } from './codex-exec.mjs';

const codexCommand = process.env.CODEX_CLI_PATH?.trim() || 'codex';
const statusCacheMs = 30_000;
let cachedStatus;

function cleanEnvironment() {
  const env = { ...process.env };
  delete env.OPENAI_API_KEY;
  delete env.CODEX_API_KEY;
  return env;
}

function probeLogin() {
  return new Promise((resolve, reject) => {
    const child = spawn(codexCommand, ['login', 'status'], {
      cwd: process.cwd(),
      env: cleanEnvironment(),
      windowsHide: true,
      stdio: 'ignore',
    });
    const timer = setTimeout(() => {
      child.kill();
      reject(new Error('Codex CLI 登入檢查逾時'));
    }, 10_000);
    child.on('error', (error) => {
      clearTimeout(timer);
      if ('code' in error && error.code === 'ENOENT') {
        reject(new Error('找不到 Codex CLI；請先安裝 Codex，或設定 CODEX_CLI_PATH'));
        return;
      }
      reject(error);
    });
    child.on('close', (code) => {
      clearTimeout(timer);
      resolve(code === 0);
    });
  });
}

export async function getCodexStatus() {
  const now = Date.now();
  if (cachedStatus && now - cachedStatus.checkedAt < statusCacheMs) {
    return cachedStatus.value;
  }

  let value;
  try {
    const configured = await probeLogin();
    value = {
      configured,
      provider: configured ? 'Codex CLI（ChatGPT 登入）' : 'Codex CLI',
      model: codexModel,
      message: configured ? undefined : 'Codex CLI 尚未登入，請先執行 codex login',
    };
  } catch (error) {
    value = {
      configured: false,
      provider: 'Codex CLI',
      model: codexModel,
      message: error instanceof Error ? error.message : String(error),
    };
  }
  cachedStatus = { checkedAt: now, value };
  return value;
}
