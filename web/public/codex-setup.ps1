# Copyright (C) 2023-2026 QuantumNous
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.

$ErrorActionPreference = 'Stop'

$apiKey = $env:VECTOR_EPOCH_CODEX_API_KEY
$baseUrl = $env:VECTOR_EPOCH_CODEX_BASE_URL
if ([string]::IsNullOrWhiteSpace($baseUrl)) {
  $baseUrl = 'https://gate.vectorepoch.com/v1'
}

function Stop-Setup([string]$message) {
  throw $message
}

if ([string]::IsNullOrWhiteSpace($apiKey)) {
  $apiKey = Read-Host '请输入 Relay API Key'
}

if (-not $apiKey.StartsWith('sk-')) {
  Stop-Setup 'API Key 格式不正确，应以 sk- 开头'
}

if (-not $baseUrl.StartsWith('https://')) {
  Stop-Setup 'API 地址必须使用 HTTPS'
}

function Get-MaskedKey([string]$value) {
  if ($value.Length -le 12) {
    return $value.Substring(0, 4) + '...' + $value.Substring($value.Length - 4)
  }
  return $value.Substring(0, 8) + '...' + $value.Substring($value.Length - 6)
}

Write-Host '正在检测 Windows 环境并验证 Relay API Key...'
try {
  $headers = @{ Authorization = "Bearer $apiKey" }
  Invoke-RestMethod -Uri "$($baseUrl.TrimEnd('/'))/models" -Headers $headers -Method Get -TimeoutSec 30 | Out-Null
} catch {
  Stop-Setup 'Relay API Key 验证失败，请检查 Key 是否有效或网络是否可用。'
}

$codex = Get-Command codex -ErrorAction SilentlyContinue
if ($null -eq $codex) {
  $npm = Get-Command npm -ErrorAction SilentlyContinue
  if ($null -eq $npm) {
    Stop-Setup '未找到 Codex CLI 或 npm，请先安装 Codex CLI 后重试。'
  }
  Write-Host '未找到 Codex CLI，正在通过 npm 安装官方 Codex 包。'
  & $npm.Source install --global @openai/codex
  if ($LASTEXITCODE -ne 0) {
    Stop-Setup 'Codex CLI 安装失败。'
  }
  $codex = Get-Command codex -ErrorAction SilentlyContinue
}

if ($null -eq $codex) {
  Stop-Setup 'Codex CLI 安装后仍不可用，请重新打开 PowerShell 后重试。'
}

$codexHome = $env:CODEX_HOME
if ([string]::IsNullOrWhiteSpace($codexHome)) {
  $codexHome = Join-Path $HOME '.codex'
}

New-Item -ItemType Directory -Force -Path $codexHome | Out-Null
$backupDir = Join-Path $codexHome 'backups'
New-Item -ItemType Directory -Force -Path $backupDir | Out-Null
$timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$configPath = Join-Path $codexHome 'config.toml'
$authPath = Join-Path $codexHome 'auth.json'

if (Test-Path $configPath) {
  Copy-Item $configPath (Join-Path $backupDir "config.toml.$timestamp.bak")
}
if (Test-Path $authPath) {
  Copy-Item $authPath (Join-Path $backupDir "auth.json.$timestamp.bak")
}

$config = @"
# 由向量纪元 Relay 生成。重新执行配置命令即可更新。
model_provider = "vectorepoch"
model = "gpt-5.5"
review_model = "gpt-5.5"
model_reasoning_effort = "xhigh"
sandbox_mode = "workspace-write"

[model_providers.vectorepoch]
name = "向量纪元 Relay"
base_url = "$baseUrl"
wire_api = "responses"
requires_openai_auth = true

[sandbox_workspace_write]
network_access = true

[features]
goals = true
"@

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($configPath, $config, $utf8NoBom)

$apiKey | & $codex.Source login --with-api-key
if ($LASTEXITCODE -ne 0) {
  Stop-Setup 'Codex API Key 写入失败。'
}

$configContent = Get-Content -Raw $configPath
if (-not $configContent.Contains('model_provider = "vectorepoch"') -or
    -not $configContent.Contains("base_url = ""$baseUrl""")) {
  Stop-Setup 'Codex 配置文件校验失败。'
}

& $codex.Source login status *> $null
if ($LASTEXITCODE -ne 0) {
  Stop-Setup 'Codex 登录状态校验失败，未确认配置已生效。'
}

Write-Host 'Codex 已配置完成。' -ForegroundColor Green
Write-Host "已配置 Key: $(Get-MaskedKey $apiKey)"
Write-Host "配置文件: $configPath"
Remove-Item Env:VECTOR_EPOCH_CODEX_API_KEY -ErrorAction SilentlyContinue
Remove-Item Env:VECTOR_EPOCH_CODEX_BASE_URL -ErrorAction SilentlyContinue
