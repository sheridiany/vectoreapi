param(
  [string]$Token = '',
  [string]$Label = 'vsearch-agent',
  [string]$Origin = ''
)
$ErrorActionPreference = 'Stop'
if (-not $Origin) { $Origin = if ($env:VSEARCH_API_ORIGIN) { $env:VSEARCH_API_ORIGIN } else { 'https://search.vectorepoch.com' } }
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { throw '需要 Node.js 18 或更高版本才能写入 MCP 配置。' }

$hadInstallToken = Test-Path Env:VSEARCH_INSTALL_TOKEN
$hadInstallLabel = Test-Path Env:VSEARCH_INSTALL_LABEL
$hadApiOrigin = Test-Path Env:VSEARCH_API_ORIGIN
$previousInstallToken = $env:VSEARCH_INSTALL_TOKEN
$previousInstallLabel = $env:VSEARCH_INSTALL_LABEL
$previousApiOrigin = $env:VSEARCH_API_ORIGIN

try {
  $env:VSEARCH_INSTALL_TOKEN = $Token
  $env:VSEARCH_INSTALL_LABEL = $Label
  $env:VSEARCH_API_ORIGIN = $Origin
  $nodeScript = @'
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

async function main() {
function secureURL(raw, originOnly = false) {
  let parsed;
  try { parsed = new URL(String(raw || '')); } catch { throw new Error('vSearch 服务地址无效。'); }
  const loopback = parsed.hostname === 'localhost' || parsed.hostname === '::1' || parsed.hostname === '[::1]' || /^127\./.test(parsed.hostname);
  if (parsed.username || parsed.password || parsed.search || parsed.hash) throw new Error('vSearch 服务地址无效。');
  if (parsed.protocol !== 'https:' && !(parsed.protocol === 'http:' && loopback)) throw new Error('vSearch 服务地址必须使用 HTTPS；本机回环开发环境可使用 HTTP。');
  if (originOnly && parsed.pathname !== '/') throw new Error('vSearch 服务地址必须是站点根地址。');
  return originOnly ? parsed.origin : parsed.href.replace(/\/$/, '');
}
const origin = secureURL(process.env.VSEARCH_API_ORIGIN, true);
const home = os.homedir();
const bridgeDirectory = path.join(home, '.config', 'vsearch');
const bridgePath = path.join(bridgeDirectory, 'stdio-bridge.mjs');
const bridgeSource = `import process from 'node:process';
const endpoint = process.env.VSEARCH_MCP_URL || '';
const apiKey = process.env.VSEARCH_API_KEY || '';
if (!endpoint || !apiKey) { process.stderr.write('vSearch MCP 配置不完整。\\n'); process.exit(1); }
async function forward(message) {
  const response = await fetch(endpoint, { method: 'POST', redirect: 'error', headers: { accept: 'application/json, text/event-stream', 'content-type': 'application/json', authorization: 'Bearer ' + apiKey }, body: JSON.stringify(message) });
  if (response.status === 202 || response.status === 204) return;
  const text = await response.text();
  if (!response.ok) throw new Error('vSearch MCP 请求失败（' + response.status + '）。');
  if ((response.headers.get('content-type') || '').includes('text/event-stream')) {
    for (const line of text.split(/\\r?\\n/)) { if (line.startsWith('data:')) { process.stdout.write(line.slice(5).trim() + '\\n'); return; } }
    return;
  }
  if (text.trim()) process.stdout.write(text.trim() + '\\n');
}
let buffer = ''; let queue = Promise.resolve(); process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => { buffer += chunk; let index = buffer.indexOf('\\n'); while (index >= 0) { const line = buffer.slice(0, index).trim(); buffer = buffer.slice(index + 1); index = buffer.indexOf('\\n'); if (!line) continue; queue = queue.then(() => forward(JSON.parse(line))).catch((error) => process.stderr.write(error.message + '\\n')); } });
`;
const recoveryDirectory = path.join(bridgeDirectory, 'install-recovery');
const journalPath = path.join(recoveryDirectory, 'journal.json');
const timeoutValue = Number.parseInt(process.env.VSEARCH_INSTALL_TIMEOUT_MS || '5000', 10);
const requestTimeout = Number.isFinite(timeoutValue) && timeoutValue >= 100 && timeoutValue <= 60000 ? timeoutValue : 5000;

function syncDirectory(directory) {
  if (process.platform === 'win32') return;
  let descriptor;
  try {
    descriptor = fs.openSync(directory, 'r');
    fs.fsyncSync(descriptor);
  } catch (error) {
    if (!['EINVAL', 'ENOTSUP', 'EPERM'].includes(error.code)) throw error;
  } finally {
    if (descriptor !== undefined) fs.closeSync(descriptor);
  }
}

function atomicWrite(file, content) {
  const temporary = `${file}.tmp-${process.pid}-${Date.now()}`;
  try {
    const descriptor = fs.openSync(temporary, 'wx', 0o600);
    try { fs.writeFileSync(descriptor, content); fs.fsyncSync(descriptor); } finally { fs.closeSync(descriptor); }
    fs.renameSync(temporary, file);
    fs.chmodSync(file, 0o600);
    syncDirectory(path.dirname(file));
  } catch (error) {
    if (fs.existsSync(temporary)) fs.unlinkSync(temporary);
    throw error;
  }
}

function unlinkDurable(file) {
  if (!fs.existsSync(file)) return;
  fs.unlinkSync(file);
  syncDirectory(path.dirname(file));
}

async function fetchWithTimeout(resource, options) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), requestTimeout);
  try {
    return await fetch(resource, { ...options, signal: controller.signal });
  } finally {
    clearTimeout(timer);
  }
}

const target = (id, name, file, marker, root = 'mcpServers') => ({ id, name, file, marker, root });
const targets = [
  target('claude-desktop', 'Claude Desktop', process.platform === 'darwin' ? path.join(home, 'Library/Application Support/Claude/claude_desktop_config.json') : path.join(home, '.config/Claude/claude_desktop_config.json'), process.platform === 'darwin' ? path.join(home, 'Library/Application Support/Claude') : path.join(home, '.config/Claude')),
  target('claude-code', 'Claude Code', path.join(home, '.claude.json'), path.join(home, '.claude')),
  target('cursor', 'Cursor', path.join(home, '.cursor/mcp.json'), path.join(home, '.cursor')),
  target('windsurf', 'Windsurf', path.join(home, '.codeium/windsurf/mcp_config.json'), path.join(home, '.codeium/windsurf')),
  target('vscode', 'VS Code', process.platform === 'darwin' ? path.join(home, 'Library/Application Support/Code/User/mcp.json') : path.join(home, '.config/Code/User/mcp.json'), process.platform === 'darwin' ? path.join(home, 'Library/Application Support/Code/User') : path.join(home, '.config/Code/User'), 'servers'),
  target('gemini', 'Gemini CLI', path.join(home, '.gemini/settings.json'), path.join(home, '.gemini')),
];

function validateJournal(journal) {
  if (!journal || journal.version !== 1 || !Array.isArray(journal.targets) || !journal.targets.length) throw new Error('vSearch 安装恢复记录无效。');
  journal.origin = secureURL(journal.origin, true);
  journal.endpoint = secureURL(journal.endpoint);
  if (!String(journal.key || '').startsWith('vr_live_') || !String(journal.activationToken || '').startsWith('vr_search_activate_')) throw new Error('vSearch 安装恢复凭证无效。');
  journal.targets.forEach((item, index) => {
    const expectedBackup = path.join(recoveryDirectory, `backup-${index}.bin`);
    if (!path.isAbsolute(item.file) || item.backup !== expectedBackup || !['bridge', 'config'].includes(item.kind)) throw new Error('vSearch 安装恢复目标无效。');
    if (item.kind === 'config' && !['mcpServers', 'servers'].includes(item.root)) throw new Error('vSearch 安装恢复配置无效。');
  });
  return journal;
}

function readJournal() {
  if (!fs.existsSync(journalPath)) return null;
  try { return validateJournal(JSON.parse(fs.readFileSync(journalPath, 'utf8'))); }
  catch (error) { throw new Error(`vSearch 安装恢复记录无法读取：${error.message}`); }
}

function writeJournal(journal, phase) {
  journal.phase = phase;
  atomicWrite(journalPath, `${JSON.stringify(journal, null, 2)}\n`);
}

function cleanupOrphanedBackups() {
  if (!fs.existsSync(recoveryDirectory) || fs.existsSync(journalPath)) return;
  for (const entry of fs.readdirSync(recoveryDirectory)) {
    if (/^backup-\d+\.bin$/.test(entry)) unlinkDurable(path.join(recoveryDirectory, entry));
  }
}

function createJournal(originValue, endpoint, key, activationToken, snapshots) {
  fs.mkdirSync(recoveryDirectory, { recursive: true, mode: 0o700 });
  fs.chmodSync(recoveryDirectory, 0o700);
  cleanupOrphanedBackups();
  if (fs.existsSync(journalPath)) throw new Error('vSearch 仍有待恢复的安装。');
  const recoveryTargets = snapshots.map((item, index) => {
    const backup = path.join(recoveryDirectory, `backup-${index}.bin`);
    atomicWrite(backup, item.original);
    return { kind: item.kind, name: item.name, file: item.file, root: item.root || '', existed: item.existed, backup };
  });
  const journal = { version: 1, phase: 'prepared', origin: originValue, endpoint, key, activationToken, targets: recoveryTargets };
  writeJournal(journal, 'prepared');
  return journal;
}

function candidateServer(journal) {
  const bridge = journal.targets.find((item) => item.kind === 'bridge');
  return { command: process.execPath, args: [bridge.file], env: { VSEARCH_MCP_URL: journal.endpoint, VSEARCH_API_KEY: journal.key } };
}

function applyCandidate(journal) {
  const server = candidateServer(journal);
  for (const item of journal.targets) {
    if (item.kind === 'bridge') {
      atomicWrite(item.file, bridgeSource);
      continue;
    }
    let source = fs.existsSync(item.file) ? fs.readFileSync(item.file, 'utf8') : fs.readFileSync(item.backup, 'utf8');
    let config = {};
    if (source.trim()) { try { config = JSON.parse(source); } catch { config = JSON.parse(fs.readFileSync(item.backup, 'utf8') || '{}'); } }
    config[item.root] = config[item.root] && typeof config[item.root] === 'object' ? config[item.root] : {};
    config[item.root].vsearch = item.root === 'servers' ? { type: 'stdio', ...server } : { ...(config[item.root].vsearch || {}), ...server };
    atomicWrite(item.file, `${JSON.stringify(config, null, 2)}\n`);
  }
  writeJournal(journal, journal.phase === 'activated' ? 'activated' : 'written');
}

function restoreJournal(journal) {
  for (const item of [...journal.targets].reverse()) {
    if (item.existed) atomicWrite(item.file, fs.readFileSync(item.backup));
    else unlinkDurable(item.file);
  }
}

function cleanupJournal(journal) {
  unlinkDurable(journalPath);
  for (const item of journal.targets) unlinkDurable(item.backup);
  try { fs.rmdirSync(recoveryDirectory); syncDirectory(path.dirname(recoveryDirectory)); } catch (error) { if (!['ENOENT', 'ENOTEMPTY'].includes(error.code)) throw error; }
}

async function verifyCandidate(journal) {
  try {
    const response = await fetchWithTimeout(journal.endpoint, {
      method: 'POST', redirect: 'error', headers: { accept: 'application/json', 'content-type': 'application/json', authorization: `Bearer ${journal.key}` },
      body: JSON.stringify({ jsonrpc: '2.0', id: 'vsearch-install-verify', method: 'initialize', params: { protocolVersion: '2025-03-26', capabilities: {}, clientInfo: { name: 'vsearch-installer', version: '1' } } }),
    });
    const body = await response.text();
    const mediaType = (response.headers.get('content-type') || '').split(';', 1)[0].trim().toLowerCase();
    if (mediaType !== 'application/json' && !mediaType.endsWith('+json') && mediaType !== 'text/event-stream') {
      return { state: 'uncertain', status: response.status, message: 'MCP initialize 返回了非 JSON 响应。' };
    }
    let payload;
    try {
      if (mediaType === 'text/event-stream') {
        const eventPayloads = body.split(/\r?\n\r?\n/).map((event) => event.split(/\r?\n/)
          .filter((line) => line.startsWith('data:'))
          .map((line) => line.slice(5).replace(/^ /, ''))
          .join('\n'))
          .filter((data) => data && data !== '[DONE]');
        if (!eventPayloads.length) throw new Error('missing SSE data');
        payload = JSON.parse(eventPayloads[0]);
      } else {
        payload = JSON.parse(body);
      }
    } catch {
      return { state: 'uncertain', status: response.status, message: 'MCP initialize 返回的不是有效 JSON-RPC。' };
    }
    if (response.status === 401 || response.status === 403) {
      const pendingKeyError = payload && payload.jsonrpc === '2.0' && payload.id === null
        && payload.error?.code === -32001 && payload.error?.message === 'vSearch key is invalid';
      return pendingKeyError
        ? { state: 'pending', status: response.status }
        : { state: 'uncertain', status: response.status, message: 'MCP 鉴权响应无法确认来自 vSearch。' };
    }
    if (!response.ok) return { state: 'uncertain', status: response.status };
    const validResult = payload && payload.jsonrpc === '2.0' && payload.id === 'vsearch-install-verify'
      && Object.prototype.hasOwnProperty.call(payload, 'result') && payload.result !== null
      && typeof payload.result === 'object' && !Array.isArray(payload.result);
    if (!validResult) return { state: 'uncertain', status: response.status, message: 'MCP initialize 响应与请求不匹配。' };
    return { state: 'active' };
  } catch (error) {
    return { state: 'uncertain', message: error.message };
  }
}

async function activateCandidate(journal) {
  try {
    const response = await fetchWithTimeout(`${journal.origin}/api/agent/install/activate`, {
      method: 'POST', redirect: 'error', headers: { accept: 'application/json', 'content-type': 'application/json' },
      body: JSON.stringify({ token: journal.activationToken }),
    });
    const payload = await response.json().catch(() => ({}));
    if (response.ok && payload.success) return { state: 'active' };
    return { state: response.status >= 500 ? 'uncertain' : 'rejected', message: payload.message || `密钥激活失败（${response.status}）。` };
  } catch (error) {
    return { state: 'uncertain', message: error.message };
  }
}

async function finishJournal(journal) {
  const existing = await verifyCandidate(journal);
  if (existing.state === 'active') {
    applyCandidate(journal);
    cleanupJournal(journal);
    return 'active';
  }
  if (journal.phase === 'activated') {
    throw new Error('密钥已激活，但 MCP 配置验证失败；未回滚本地配置，已保留恢复记录，请修正服务地址后重跑安装器。');
  }
  if (existing.state !== 'pending') {
    throw new Error(`${existing.message || 'MCP 服务状态无法确认。'}；未修改本地配置、未激活候选密钥，已保留恢复记录，请检查服务地址后重跑安装器。`);
  }
  try {
    applyCandidate(journal);
  } catch (error) {
    const verification = await verifyCandidate(journal);
    if (verification.state === 'pending') {
      restoreJournal(journal);
      cleanupJournal(journal);
      throw error;
    }
    throw new Error(`${error.message}；已保留恢复记录，请稍后重跑安装器。`);
  }
  let sawUncertain = false;
  let lastActivation = { state: 'rejected', message: '密钥激活失败。' };
  for (let attempt = 0; attempt < 2; attempt += 1) {
    lastActivation = await activateCandidate(journal);
    if (lastActivation.state === 'active') {
      writeJournal(journal, 'activated');
      const verification = await verifyCandidate(journal);
      if (verification.state === 'active') {
        cleanupJournal(journal);
        return 'active';
      }
      throw new Error('密钥已激活，但 MCP 配置验证失败；未回滚本地配置，已保留恢复记录，请修正服务地址后重跑安装器。');
    }
    if (lastActivation.state === 'uncertain') sawUncertain = true;
    if (attempt === 0) await new Promise((resolve) => setTimeout(resolve, 250));
  }
  const verification = await verifyCandidate(journal);
  if (verification.state === 'active') {
    cleanupJournal(journal);
    return 'active';
  }
  if (verification.state === 'pending' && !sawUncertain) {
    restoreJournal(journal);
    cleanupJournal(journal);
    throw new Error(`${lastActivation.message}；候选密钥已确认无效，本地配置已安全回滚。`);
  }
  throw new Error(`${lastActivation.message || '密钥激活状态无法确认。'}；未回滚本地配置，已保留恢复记录，请稍后重跑安装器。`);
}

fs.mkdirSync(bridgeDirectory, { recursive: true, mode: 0o700 });
const pendingJournal = readJournal();
if (pendingJournal) {
  await finishJournal(pendingJournal);
  for (const item of pendingJournal.targets.filter((item) => item.kind === 'config')) process.stdout.write(`✓ ${item.name}: ${item.file}\n`);
  process.stdout.write('vSearch 已恢复上次安装；完整密钥未输出。\n');
  return;
}

if (!String(process.env.VSEARCH_INSTALL_TOKEN || '').trim()) throw new Error('缺少一次性安装凭证。');
const requested = String(process.env.VSEARCH_AGENT || 'auto').toLowerCase();
let selected = requested === 'auto' ? targets.filter((item) => fs.existsSync(item.file) || fs.existsSync(item.marker)) : targets.filter((item) => item.id === requested || item.id.startsWith(requested));
if (!selected.length) selected = [target('generic', 'Generic MCP client', process.env.VSEARCH_CONFIG_PATH || path.join(home, '.config/vsearch/mcp.json'), '')];

const snapshots = selected.map((item) => {
  let config = {};
  const existed = fs.existsSync(item.file);
  const original = existed ? fs.readFileSync(item.file) : Buffer.from('');
  if (original.toString().trim()) { try { config = JSON.parse(original.toString()); } catch { throw new Error(`已有配置无法解析，请先修复：${item.file}`); } }
  fs.mkdirSync(path.dirname(item.file), { recursive: true, mode: 0o700 });
  const probe = `${item.file}.preflight-${process.pid}`;
  fs.writeFileSync(probe, '{}\n', { mode: 0o600 });
  fs.unlinkSync(probe);
  return { kind: 'config', ...item, config, existed, original };
});
const bridgeExisted = fs.existsSync(bridgePath);
const bridgeOriginal = bridgeExisted ? fs.readFileSync(bridgePath) : Buffer.from('');
const bridgeProbe = `${bridgePath}.preflight-${process.pid}`;
fs.writeFileSync(bridgeProbe, bridgeOriginal.length ? bridgeOriginal : '// vSearch installer preflight\n', { mode: 0o600 });
fs.unlinkSync(bridgeProbe);

const response = await fetchWithTimeout(`${origin}/api/agent/install`, {
  method: 'POST', redirect: 'error', headers: { accept: 'application/json', 'content-type': 'application/json' },
  body: JSON.stringify({ token: process.env.VSEARCH_INSTALL_TOKEN, label: process.env.VSEARCH_INSTALL_LABEL || 'vsearch-agent' }),
});
const payload = await response.json().catch(() => ({}));
if (!response.ok || !payload.data?.secret || !payload.data?.activation_token) throw new Error(payload.message || `安装凭证无效（${response.status}）。`);
const endpoint = secureURL(payload.data.mcp_url || payload.data.mcpUrl || `${origin}/v1/mcp`);
const journal = createJournal(origin, endpoint, payload.data.secret, payload.data.activation_token, [
  { kind: 'bridge', name: 'vSearch bridge', file: bridgePath, existed: bridgeExisted, original: bridgeOriginal },
  ...snapshots,
]);
await finishJournal(journal);
for (const item of snapshots) process.stdout.write(`✓ ${item.name}: ${item.file}\n`);
process.stdout.write(`vSearch 已接入 ${selected.length} 个 Agent；完整密钥未输出。\n`);
}
main().catch((error) => { process.stderr.write(error.message + '\n'); process.exitCode = 1; });
'@
  node -e $nodeScript
  if ($LASTEXITCODE -ne 0) { throw "vSearch 安装失败（Node.js 退出码 $LASTEXITCODE）。" }
} finally {
  if ($hadInstallToken) { $env:VSEARCH_INSTALL_TOKEN = $previousInstallToken } else { Remove-Item Env:VSEARCH_INSTALL_TOKEN -ErrorAction SilentlyContinue }
  if ($hadInstallLabel) { $env:VSEARCH_INSTALL_LABEL = $previousInstallLabel } else { Remove-Item Env:VSEARCH_INSTALL_LABEL -ErrorAction SilentlyContinue }
  if ($hadApiOrigin) { $env:VSEARCH_API_ORIGIN = $previousApiOrigin } else { Remove-Item Env:VSEARCH_API_ORIGIN -ErrorAction SilentlyContinue }
}
