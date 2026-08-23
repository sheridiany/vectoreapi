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
import { describe, expect, test } from 'vitest'

import {
  CODEX_API_BASE_URL,
  CODEX_SETUP_SCRIPT_URL,
  CODEX_WINDOWS_SETUP_SCRIPT_URL,
  buildCodexSetupCommand,
  detectCodexPlatform,
} from '../codex-setup'

describe('detectCodexPlatform', () => {
  test('detects supported desktop platforms', () => {
    expect(detectCodexPlatform('MacIntel')).toBe('macos')
    expect(detectCodexPlatform('Win32')).toBe('windows')
    expect(detectCodexPlatform('Linux x86_64')).toBe('linux')
  })
})

describe('buildCodexSetupCommand', () => {
  test('creates a Unix command with the public gateway and API key', () => {
    const command = buildCodexSetupCommand('sk-example', 'macos')

    expect(command).toContain(`curl -fsSL '${CODEX_SETUP_SCRIPT_URL}'`)
    expect(command).toContain("bash -s -- --api-key 'sk-example'")
    expect(command).toContain(`--base-url '${CODEX_API_BASE_URL}'`)
    expect(command).not.toContain(' | --base-url')
  })

  test('creates a PowerShell command for Windows', () => {
    const command = buildCodexSetupCommand('sk-example', 'windows')

    expect(command).toContain("$env:VECTOR_EPOCH_CODEX_API_KEY='sk-example'")
    expect(command).toContain(CODEX_WINDOWS_SETUP_SCRIPT_URL)
    expect(command).not.toContain('bash -s')
  })

  test('quotes shell-sensitive key characters', () => {
    const command = buildCodexSetupCommand("sk-example'with spaces", 'linux')

    expect(command).toContain("'sk-example'\\''with spaces'")
  })
})
