import http from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import process from 'node:process';

const root = path.dirname(fileURLToPath(import.meta.url));
try { process.loadEnvFile(path.join(root, '.env')); } catch (error) { if (error?.code !== 'ENOENT') throw error; }
const { getAgentStatus, runDungeonMaster } = await import('./dm-agent.mjs');
const { getImageStatus, generateSceneImage, generateCharacterImage } = await import('./scene-image.mjs');
const { codexModelOptions, normalizeCodexModel } = await import('./codex-cli.mjs');
const { buildDmRequest } = await import('./dm-request.mjs');
const publicRoot = path.join(root, 'web-dist');
const generatedRoot = path.join(root, 'campaign-data', 'images');
const port = Number(process.env.PORT || 4318);

const mime = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8', '.svg': 'image/svg+xml', '.woff2': 'font/woff2', '.json': 'application/json; charset=utf-8', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.png': 'image/png', '.webp': 'image/webp' };

function json(response, status, body) {
  response.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' });
  response.end(JSON.stringify(body));
}

async function readJson(request) {
  let raw = '';
  for await (const chunk of request) {
    raw += chunk;
    if (raw.length > 1_000_000) throw new Error('Request body is too large');
  }
  return JSON.parse(raw || '{}');
}

async function handleStatus(response) {
  const status = await getAgentStatus();
  json(response, 200, { connected: status.configured, provider: status.provider, model: status.model, models: codexModelOptions, imageModel: getImageStatus().model, message: status.configured ? undefined : status.message });
}

async function handleSceneImage(request, response) {
  try {
    const body = await readJson(request);
    const title = String(body.campaign?.title || '').trim().slice(0, 180);
    const scene = String(body.campaign?.scene || '').trim().slice(0, 240);
    const narration = String(body.narration || '').trim().slice(0, 5000);
    const players = Array.isArray(body.players) ? body.players.slice(0, 4).map((player) => ({ name: String(player?.name || '冒險者').slice(0, 100), className: String(player?.className || '旅人').slice(0, 100) })) : [];
    if (!title || !scene || !narration) return json(response, 400, { error: '需要戰役、場景與最近敘事才能生成插圖。' });
    const result = await generateSceneImage({ title, scene, narration, players }, generatedRoot, AbortSignal.timeout(450_000));
    json(response, 200, result);
  } catch (error) {
    json(response, 503, { error: error instanceof Error ? error.message : String(error) });
  }
}

async function handleCharacterImage(request, response) {
  try {
    const body = await readJson(request);
    const input = {
      name: String(body.name || '').trim().slice(0, 100),
      species: String(body.species || '').trim().slice(0, 80),
      className: String(body.className || '').trim().slice(0, 100),
      background: String(body.background || '').trim().slice(0, 100),
      appearance: String(body.appearance || '').trim().slice(0, 1200),
    };
    if (!input.name || !input.appearance) return json(response, 400, { error: '需要角色名稱與外觀描述才能生成角色圖。' });
    const result = await generateCharacterImage(input, generatedRoot, AbortSignal.timeout(450_000));
    json(response, 200, result);
  } catch (error) {
    json(response, 503, { error: error instanceof Error ? error.message : String(error) });
  }
}

async function handleDm(request, response) {
  try {
    const body = await readJson(request);
    const { prompt } = buildDmRequest(body);
    const selectedModel = normalizeCodexModel(body.model);
    const output = await runDungeonMaster(prompt, AbortSignal.timeout(210_000), selectedModel);
    const checkText = output.requiresCheck && output.check ? `\n\n檢定：${output.check.character} 進行 DC ${output.check.dc} 的${output.check.ability}（${output.check.skill}）檢定。${output.check.reason}` : '';
    const choiceText = output.choices.length ? `\n\n可考慮：${output.choices.join('／')}` : '';
    const status = await getAgentStatus();
    json(response, 200, {
      text: `${output.narration}${checkText}${choiceText}`,
      scene: output.scene,
      objective: output.objective,
      objectiveContext: output.objectiveContext,
      stakes: output.stakes,
      choices: output.choices,
      requiresCheck: output.requiresCheck,
      check: output.check,
      privateMessages: output.privateMessages,
      effects: output.effects,
      combat: output.combat,
      actionIssues: output.actionIssues,
      experienceAwards: output.experienceAwards,
      model: selectedModel || status.model,
    });
  } catch (error) {
    json(response, Number(error?.statusCode || 503), { error: error instanceof Error ? error.message : String(error) });
  }
}

async function serveStatic(request, response) {
  const urlPath = decodeURIComponent(new URL(request.url || '/', 'http://localhost').pathname);
  const requested = urlPath === '/' ? 'index.html' : urlPath.replace(/^\/+/, '');
  let filePath = path.resolve(publicRoot, requested);
  if (!filePath.startsWith(`${publicRoot}${path.sep}`) && filePath !== publicRoot) return response.writeHead(403).end('Forbidden');
  try {
    const info = await stat(filePath);
    if (info.isDirectory()) filePath = path.join(filePath, 'index.html');
    const body = await readFile(filePath);
    response.writeHead(200, { 'content-type': mime[path.extname(filePath)] || 'application/octet-stream', 'x-content-type-options': 'nosniff' });
    response.end(body);
  } catch {
    try {
      const body = await readFile(path.join(publicRoot, 'index.html'));
      response.writeHead(200, { 'content-type': mime['.html'] }); response.end(body);
    } catch { response.writeHead(404).end('Build the web app first with npm run web:build'); }
  }
}

async function serveGenerated(request, response) {
  const urlPath = decodeURIComponent(new URL(request.url || '/', 'http://localhost').pathname);
  const filename = path.basename(urlPath);
  if (!/^[a-zA-Z0-9-]+\.(?:png|jpe?g|webp)$/i.test(filename)) return response.writeHead(400).end('Invalid image path');
  try {
    const body = await readFile(path.join(generatedRoot, filename));
    response.writeHead(200, { 'content-type': mime[path.extname(filename).toLowerCase()] || 'application/octet-stream', 'cache-control': 'private, max-age=31536000, immutable', 'x-content-type-options': 'nosniff' });
    response.end(body);
  } catch { response.writeHead(404).end('Image not found'); }
}

const server = http.createServer(async (request, response) => {
  if (request.method === 'GET' && request.url === '/api/status') return handleStatus(response);
  if (request.method === 'POST' && request.url === '/api/dm') return handleDm(request, response);
  if (request.method === 'POST' && request.url === '/api/scene-image') return handleSceneImage(request, response);
  if (request.method === 'POST' && request.url === '/api/character-image') return handleCharacterImage(request, response);
  if (request.method === 'GET' && request.url?.startsWith('/generated/')) return serveGenerated(request, response);
  await serveStatic(request, response);
});

server.listen(port, '127.0.0.1', async () => {
  console.log(`D&D local table: http://127.0.0.1:${port}`);
  const status = await getAgentStatus();
  console.log(`Codex CLI: ${status.configured ? status.model : status.message}`);
});
