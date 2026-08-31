param(
  [string]$Key = '',
  [string]$Origin = ''
)
$ErrorActionPreference = 'Stop'
if (-not $Origin) { $Origin = if ($env:VSEARCH_API_ORIGIN) { $env:VSEARCH_API_ORIGIN } else { 'https://gate.vectorepoch.com' } }
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { throw '需要 Node.js 18 或更高版本才能写入 MCP 配置。' }

$hadApiKey = Test-Path Env:VSEARCH_API_KEY
$hadApiOrigin = Test-Path Env:VSEARCH_API_ORIGIN
$previousApiKey = $env:VSEARCH_API_KEY
$previousApiOrigin = $env:VSEARCH_API_ORIGIN
$temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("vsearch-install-" + [System.Guid]::NewGuid().ToString('N') + '.mjs')

try {
  Invoke-WebRequest -Uri "$($Origin.TrimEnd('/'))/install.mjs" -OutFile $temporary
  $env:VSEARCH_API_KEY = $Key
  $env:VSEARCH_API_ORIGIN = $Origin
  & node $temporary
  if ($LASTEXITCODE -ne 0) { throw "vSearch 安装失败（Node.js 退出码 $LASTEXITCODE）。" }
} finally {
  Remove-Item $temporary -ErrorAction SilentlyContinue
  if ($hadApiKey) { $env:VSEARCH_API_KEY = $previousApiKey } else { Remove-Item Env:VSEARCH_API_KEY -ErrorAction SilentlyContinue }
  if ($hadApiOrigin) { $env:VSEARCH_API_ORIGIN = $previousApiOrigin } else { Remove-Item Env:VSEARCH_API_ORIGIN -ErrorAction SilentlyContinue }
}
