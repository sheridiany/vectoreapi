import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import process from 'node:process'

function secureURL(raw, originOnly = false) {
  let parsed
  try {
    parsed = new URL(String(raw || ''))
  } catch {
    throw new Error('vSearch 服务地址无效。')
  }
  const loopback =
    parsed.hostname === 'localhost' ||
    parsed.hostname === '::1' ||
    /^127\./.test(parsed.hostname)
  if (parsed.username || parsed.password || parsed.search || parsed.hash) {
    throw new Error('vSearch 服务地址无效。')
  }
  if (
    parsed.protocol !== 'https:' &&
    !(parsed.protocol === 'http:' && loopback)
  ) {
    throw new Error(
      'vSearch 服务地址必须使用 HTTPS；本机回环开发环境可使用 HTTP。'
    )
  }
  if (originOnly && parsed.pathname !== '/') {
    throw new Error('vSearch 服务地址必须是站点根地址。')
  }
  return originOnly ? parsed.origin : parsed.href.replace(/\/$/, '')
}

function atomicWrite(file, content) {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 })
  const temporary = `${file}.tmp-${process.pid}-${Date.now()}`
  try {
    const descriptor = fs.openSync(temporary, 'wx', 0o600)
    try {
      fs.writeFileSync(descriptor, content)
      fs.fsyncSync(descriptor)
    } finally {
      fs.closeSync(descriptor)
    }
    fs.renameSync(temporary, file)
    fs.chmodSync(file, 0o600)
  } catch (error) {
    if (fs.existsSync(temporary)) fs.unlinkSync(temporary)
    throw error
  }
}

function restore(snapshot) {
  if (snapshot.existed) atomicWrite(snapshot.file, snapshot.original)
  else if (fs.existsSync(snapshot.file)) fs.unlinkSync(snapshot.file)
}

async function verify(endpoint, key) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 8000)
  try {
    const response = await fetch(endpoint, {
      method: 'POST',
      redirect: 'error',
      signal: controller.signal,
      headers: {
        accept: 'application/json, text/event-stream',
        'content-type': 'application/json',
        authorization: `Bearer ${key}`,
      },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: 'vsearch-install-verify',
        method: 'initialize',
        params: {
          protocolVersion: '2025-06-18',
          capabilities: {},
          clientInfo: { name: 'vsearch-installer', version: '1' },
        },
      }),
    })
    const body = await response.text()
    if (!response.ok) {
      throw new Error(`vSearch Key 验证失败（${response.status}）。`)
    }
    let payload
    if ((response.headers.get('content-type') || '').includes('text/event-stream')) {
      const data = body.split(/\r?\n/).find((line) => line.startsWith('data:'))
      if (!data) throw new Error('MCP initialize 未返回有效数据。')
      payload = JSON.parse(data.slice(5).trim())
    } else {
      payload = JSON.parse(body)
    }
    if (
      payload?.jsonrpc !== '2.0' ||
      payload?.id !== 'vsearch-install-verify' ||
      !payload?.result
    ) {
      throw new Error('MCP initialize 响应无效。')
    }
  } finally {
    clearTimeout(timer)
  }
}

const key = String(process.env.VSEARCH_API_KEY || '').trim()
if (!key.startsWith('vr_live_')) {
  throw new Error('缺少有效的 vSearch Key。请使用 --key vr_live_xxx。')
}
const origin = secureURL(
  process.env.VSEARCH_API_ORIGIN || 'https://gate.vectorepoch.com',
  true
)
const endpoint = secureURL(process.env.VSEARCH_MCP_URL || `${origin}/v1/mcp`)
await verify(endpoint, key)

const home = os.homedir()
const bridgeDirectory = path.join(home, '.config', 'vsearch')
const bridgePath = path.join(bridgeDirectory, 'stdio-bridge.mjs')
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
`

const target = (id, name, file, marker, root = 'mcpServers') => ({
  id,
  name,
  file,
  marker,
  root,
})
const targets = [
  target(
    'claude-desktop',
    'Claude Desktop',
    process.platform === 'darwin'
      ? path.join(home, 'Library/Application Support/Claude/claude_desktop_config.json')
      : path.join(home, '.config/Claude/claude_desktop_config.json'),
    process.platform === 'darwin'
      ? path.join(home, 'Library/Application Support/Claude')
      : path.join(home, '.config/Claude')
  ),
  target('claude-code', 'Claude Code', path.join(home, '.claude.json'), path.join(home, '.claude')),
  target('cursor', 'Cursor', path.join(home, '.cursor/mcp.json'), path.join(home, '.cursor')),
  target('windsurf', 'Windsurf', path.join(home, '.codeium/windsurf/mcp_config.json'), path.join(home, '.codeium/windsurf')),
  target(
    'vscode',
    'VS Code',
    process.platform === 'darwin'
      ? path.join(home, 'Library/Application Support/Code/User/mcp.json')
      : path.join(home, '.config/Code/User/mcp.json'),
    process.platform === 'darwin'
      ? path.join(home, 'Library/Application Support/Code/User')
      : path.join(home, '.config/Code/User'),
    'servers'
  ),
  target('gemini', 'Gemini CLI', path.join(home, '.gemini/settings.json'), path.join(home, '.gemini')),
]
const requested = String(process.env.VSEARCH_AGENT || 'auto').toLowerCase()
let selected =
  requested === 'auto'
    ? targets.filter((item) => fs.existsSync(item.file) || fs.existsSync(item.marker))
    : targets.filter(
        (item) => item.id === requested || item.id.startsWith(requested)
      )
if (!selected.length) {
  selected = [
    target(
      'generic',
      'Generic MCP client',
      process.env.VSEARCH_CONFIG_PATH || path.join(home, '.config/vsearch/mcp.json'),
      ''
    ),
  ]
}

const snapshots = [
  {
    file: bridgePath,
    existed: fs.existsSync(bridgePath),
    original: fs.existsSync(bridgePath)
      ? fs.readFileSync(bridgePath)
      : Buffer.from(''),
  },
]
for (const item of selected) {
  snapshots.push({
    file: item.file,
    existed: fs.existsSync(item.file),
    original: fs.existsSync(item.file)
      ? fs.readFileSync(item.file)
      : Buffer.from(''),
  })
}
const server = {
  command: process.execPath,
  args: [bridgePath],
  env: { VSEARCH_MCP_URL: endpoint, VSEARCH_API_KEY: key },
}
try {
  atomicWrite(bridgePath, bridgeSource)
  for (const item of selected) {
    const source = fs.existsSync(item.file)
      ? fs.readFileSync(item.file, 'utf8')
      : ''
    let config = {}
    if (source.trim()) {
      try {
        config = JSON.parse(source)
      } catch {
        throw new Error(`已有配置无法解析，请先修复：${item.file}`)
      }
    }
    config[item.root] =
      config[item.root] && typeof config[item.root] === 'object'
        ? config[item.root]
        : {}
    config[item.root].vsearch =
      item.root === 'servers'
        ? { type: 'stdio', ...server }
        : { ...(config[item.root].vsearch || {}), ...server }
    atomicWrite(item.file, `${JSON.stringify(config, null, 2)}\n`)
    process.stdout.write(`✓ ${item.name}: ${item.file}\n`)
  }
} catch (error) {
  for (const snapshot of [...snapshots].reverse()) restore(snapshot)
  throw error
}
process.stdout.write(
  `vSearch 已接入 ${selected.length} 个 Agent；原 Key 保持不变。\n`
)
