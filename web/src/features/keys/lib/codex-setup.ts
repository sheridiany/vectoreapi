/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export const CODEX_PUBLIC_GATEWAY_URL = 'https://gate.vectorepoch.com'
export const CODEX_API_BASE_URL = `${CODEX_PUBLIC_GATEWAY_URL}/v1`
export const CODEX_SETUP_SCRIPT_URL = `${CODEX_PUBLIC_GATEWAY_URL}/codex-setup.sh`
export const CODEX_WINDOWS_SETUP_SCRIPT_URL = `${CODEX_PUBLIC_GATEWAY_URL}/codex-setup.ps1`

export type CodexPlatform = 'macos' | 'windows' | 'linux'

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", "'\\''")}'`
}

function powershellQuote(value: string): string {
  return `'${value.replaceAll("'", "''")}'`
}

export function detectCodexPlatform(platform?: string): CodexPlatform {
  const browserPlatform =
    platform ??
    (typeof navigator !== 'undefined'
      ? [
          (navigator as Navigator & { userAgentData?: { platform?: string } })
            .userAgentData?.platform,
          navigator.platform,
          navigator.userAgent,
        ]
          .filter(Boolean)
          .join(' ')
      : '')

  if (/windows|win32|win64/i.test(browserPlatform)) return 'windows'
  if (/linux/i.test(browserPlatform)) return 'linux'
  return 'macos'
}

export function buildCodexSetupCommand(
  apiKey: string,
  platform: CodexPlatform = detectCodexPlatform()
): string {
  if (platform === 'windows') {
    return [
      '$env:VECTOR_EPOCH_CODEX_API_KEY=',
      powershellQuote(apiKey.trim()),
      '; $env:VECTOR_EPOCH_CODEX_BASE_URL=',
      powershellQuote(CODEX_API_BASE_URL),
      '; try { & ([scriptblock]::Create((Invoke-RestMethod -Uri ',
      powershellQuote(CODEX_WINDOWS_SETUP_SCRIPT_URL),
      '))) } finally { Remove-Item Env:VECTOR_EPOCH_CODEX_API_KEY,Env:VECTOR_EPOCH_CODEX_BASE_URL -ErrorAction SilentlyContinue }',
    ].join('')
  }

  return [
    'curl -fsSL',
    shellQuote(CODEX_SETUP_SCRIPT_URL),
    '| bash -s -- --api-key',
    shellQuote(apiKey.trim()),
    '--base-url',
    shellQuote(CODEX_API_BASE_URL),
  ].join(' ')
}
