import { spawn } from 'node:child_process';
import { readdir, realpath } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';

const codexCommand = process.env.CODEX_CLI_PATH?.trim() || 'codex';
const configuredModel = process.env.CODEX_MODEL?.trim() || '';
const maxCapturedOutput = 4_000_000;
const statusCacheMs = 30_000;
let cachedStatus;

export const codexModel = configuredModel || 'Codex 預設模型';
export const codexImageModel = 'Codex $imagegen（gpt-image-2）';

function cleanCodexEnvironment() {
  const env = { ...process.env };
  delete env.OPENAI_API_KEY;
  delete env.CODEX_API_KEY;
  delete env.OPENAI_MODEL;
  delete env.OPENAI_IMAGE_MODEL;
  delete env.OPENAI_AGENT_TRACING;
  return env;
}

function appendCaptured(current, chunk) {
  const next = current + chunk;
  return next.length <= maxCapturedOutput
    ? next
    : next.slice(next.length - maxCapturedOutput);
}

function formatCliError(stderr, fallback) {
  const detail = stderr.trim().split(/\r?\n/).slice(-12).join('\n');
  return detail || fallback;
}

/**
 * @param {string[]} args
 * @param {{ cwd: string, input?: string, signal?: AbortSignal, timeoutMs?: number }} options
 */
function runProcess(args, options) {
  const { cwd, input = '', signal, timeoutMs = 180_000 } = options;
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new Error('Codex CLI 工作已取消'));
      return;
    }

    let stdout = '';
    let stderr = '';
    let settled = false;
    const child = spawn(codexCommand, args, {
      cwd,
      env: cleanCodexEnvironment(),
      windowsHide: true,
      stdio: ['pipe', 'pipe', 'pipe'],
    });

    const finish = (callback) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      signal?.removeEventListener('abort', abort);
      callback();
    };
    const abort = () => {
      child.kill();
      finish(() => reject(new Error('Codex CLI 工作已取消')));
    };
    const timer = setTimeout(() => {
      child.kill();
      finish(() => reject(new Error(`Codex CLI 超過 ${Math.ceil(timeoutMs / 1000)} 秒仍未完成`)));
    }, timeoutMs);

    signal?.addEventListener('abort', abort, { once: true });
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (chunk) => { stdout = appendCaptured(stdout, chunk); });
    child.stderr.on('data', (chunk) => { stderr = appendCaptured(stderr, chunk); });
    child.stdin.on('error', () => {});
    child.on('error', (error) => {
      finish(() => {
        if ('code' in error && error.code === 'ENOENT') {
          reject(new Error('找不到 Codex CLI；請先安裝 Codex，或設定 CODEX_CLI_PATH'));
          return;
        }
        reject(error);
      });
    });
    child.on('close', (code) => {
      finish(() => {
        if (code !== 0) {
          reject(new Error(formatCliError(stderr, `Codex CLI 結束代碼：${code}`)));
          return;
        }
        resolve({ stdout, stderr });
      });
    });

    child.stdin.end(input);
  });
}

function baseExecArgs(cwd, sandbox) {
  const args = [
    'exec',
    '--ephemeral',
    '--color', 'never',
    '--sandbox', sandbox,
    '--cd', cwd,
  ];
  if (configuredModel) args.push('--model', configuredModel);
  return args;
}

export async function getCodexStatus() {
  const now = Date.now();
  if (cachedStatus && now - cachedStatus.checkedAt < statusCacheMs) {
    return cachedStatus.value;
  }

  let value;
  try {
    const { stdout } = await runProcess(['login', 'status'], {
      cwd: process.cwd(),
      timeoutMs: 10_000,
    });
    value = {
      configured: /logged in/i.test(stdout),
      provider: 'Codex CLI（ChatGPT 登入）',
      model: codexModel,
      message: /logged in/i.test(stdout) ? undefined : 'Codex CLI 尚未登入',
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

/**
 * @param {string} prompt
 * @param {{ cwd: string, schemaPath: string, signal?: AbortSignal, timeoutMs?: number }} options
 */
export async function runCodexStructured(prompt, options) {
  const args = baseExecArgs(options.cwd, 'read-only');
  args.push('--output-schema', options.schemaPath);
  const { stdout } = await runProcess(args, {
    cwd: options.cwd,
    input: prompt,
    signal: options.signal,
    timeoutMs: options.timeoutMs,
  });
  try {
    return JSON.parse(stdout.trim());
  } catch {
    throw new Error('Codex CLI 沒有回傳有效的結構化 JSON');
  }
}

function parseJsonLines(raw) {
  return raw
    .split(/\r?\n/)
    .filter(Boolean)
    .flatMap((line) => {
      try {
        return [JSON.parse(line)];
      } catch {
        return [];
      }
    });
}

async function findImages(directory) {
  const found = [];
  const entries = await readdir(directory, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      found.push(...await findImages(fullPath));
    } else if (/\.(?:png|jpe?g|webp)$/i.test(entry.name)) {
      found.push(fullPath);
    }
  }
  return found;
}

export async function runCodexImageGeneration(prompt, options) {
  const args = baseExecArgs(options.cwd, 'read-only');
  args.push('--json');
  const { stdout } = await runProcess(args, {
    cwd: options.cwd,
    input: prompt,
    signal: options.signal,
    timeoutMs: options.timeoutMs || 420_000,
  });
  const events = parseJsonLines(stdout);
  const threadId = events.find((event) => event?.type === 'thread.started')?.thread_id;
  if (!threadId || !/^[a-zA-Z0-9-]+$/.test(threadId)) {
    throw new Error('Codex CLI 沒有回報圖片工作識別碼');
  }

  const codeHome = process.env.CODEX_HOME || path.join(os.homedir(), '.codex');
  const allowedRoot = await realpath(path.join(codeHome, 'generated_images'));
  const sessionRoot = await realpath(path.join(allowedRoot, threadId));
  if (sessionRoot !== allowedRoot && !sessionRoot.startsWith(`${allowedRoot}${path.sep}`)) {
    throw new Error('Codex 圖片輸出位於不允許的位置');
  }
  const images = await findImages(sessionRoot);
  if (images.length !== 1) {
    throw new Error(images.length
      ? 'Codex imagegen 產生了多張圖片，無法判斷要使用哪一張'
      : 'Codex imagegen 沒有產生圖片檔案');
  }
  return realpath(images[0]);
}
