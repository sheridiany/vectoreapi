/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { describe, expect, test } from 'vitest'

import { SEARCH_CAPABILITY_GROUPS } from '../../types'
import {
  adminSearchAgentKeySchema,
  getAdminSearchAgentKeyFormDefaults,
} from '../admin-agent-key-form'

describe('managed AgentKey form schema', () => {
  test('accepts a managed user and normalizes the key name', () => {
    const result = adminSearchAgentKeySchema.safeParse({
      user_id: 7,
      name: '  research-bot  ',
      scopes: ['web-search'],
    })

    expect(result.success).toBe(true)
    if (!result.success) return
    expect(result.data.name).toBe('research-bot')
  })

  test('rejects a missing managed user, invalid name, and empty scopes', () => {
    const result = adminSearchAgentKeySchema.safeParse({
      user_id: 0,
      name: ' ',
      scopes: [],
    })

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues.map((issue) => issue.path[0])).toEqual([
      'user_id',
      'name',
      'scopes',
    ])
  })

  test('defaults to every declared vSearch capability without sharing arrays', () => {
    const first = getAdminSearchAgentKeyFormDefaults(7)
    const second = getAdminSearchAgentKeyFormDefaults(8)

    expect(first.user_id).toBe(7)
    expect(first.scopes).toEqual(
      SEARCH_CAPABILITY_GROUPS.map((group) => group.id)
    )
    expect(first.scopes).not.toBe(second.scopes)
  })
})
