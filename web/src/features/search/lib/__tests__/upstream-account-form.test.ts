/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

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
  createUpstreamAccountSchema,
  getUpstreamAccountServerFormError,
  updateUpstreamAccountSchema,
} from '../upstream-account-form'

const validAccount = {
  name: 'Primary account',
  api_key: 'ak_live_secret',
  base_url: 'https://api.agentkey.app/v1/mcp',
  pool_id: 0,
  weight: 1,
  priority: 0,
  status: 'standby' as const,
}

describe('upstream account form schema', () => {
  test('requires a secret when creating an account', () => {
    const result = createUpstreamAccountSchema.safeParse({
      ...validAccount,
      api_key: ' ',
    })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['api_key'])
  })

  test('allows an empty secret when editing an account', () => {
    const result = updateUpstreamAccountSchema.safeParse({
      ...validAccount,
      api_key: '',
    })

    expect(result.success).toBe(true)
  })

  test('rejects remote HTTP endpoints but allows loopback HTTP', () => {
    const remoteResult = createUpstreamAccountSchema.safeParse({
      ...validAccount,
      base_url: 'http://relay.example.com/v1/mcp',
    })
    const loopbackResult = createUpstreamAccountSchema.safeParse({
      ...validAccount,
      base_url: 'http://127.0.0.1:3100/v1/mcp',
    })

    expect(remoteResult.success).toBe(false)
    expect(loopbackResult.success).toBe(true)
  })

  test('rejects invalid routing numbers', () => {
    const result = createUpstreamAccountSchema.safeParse({
      ...validAccount,
      pool_id: -1,
      weight: 101,
      priority: -1,
    })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues.map((issue) => issue.path[0])).toEqual([
      'pool_id',
      'weight',
      'priority',
    ])
  })

  test('maps an AgentKey URL rejection to the URL field', () => {
    const result = getUpstreamAccountServerFormError(
      new Error('AgentKey MCP URL must use HTTPS')
    )

    expect(result.field).toBe('base_url')
  })

  test('uses an HTTP response message when mapping a field rejection', () => {
    const result = getUpstreamAccountServerFormError({
      message: 'Request failed with status code 400',
      response: { data: { message: '上游服务地址必须使用 HTTPS。' } },
    })

    expect(result.field).toBe('base_url')
  })
})
