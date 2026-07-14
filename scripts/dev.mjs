import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const vite = path.join(root, 'node_modules', 'vite', 'bin', 'vite.js');
const children = [
  spawn(process.execPath, [path.join(root, 'server.mjs')], { cwd: root, stdio: 'inherit' }),
  spawn(process.execPath, [vite], { cwd: root, stdio: 'inherit' }),
];

function stop(signal = 'SIGTERM') {
  for (const child of children) {
    if (!child.killed) child.kill(signal);
  }
}

process.on('SIGINT', () => {
  stop('SIGINT');
  process.exit(0);
});
process.on('SIGTERM', () => {
  stop();
  process.exit(0);
});

for (const child of children) {
  child.on('exit', (code) => {
    if (code && code !== 0) {
      stop();
      process.exitCode = code;
    }
  });
}
