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
*/
import { describe, expect, test } from 'vitest'

import { canManageSearch, canViewSearchUsage } from '../route-guard'

describe('search administration permissions', () => {
  test('allows platform administrators to configure the vSearch runtime', () => {
    expect(canManageSearch({ id: 1, username: 'root', role: 100 })).toBe(true)
    expect(
      canManageSearch({
        id: 2,
        username: 'owner',
        role: 1,
        enterprise: {
          id: 7,
          name: 'Acme',
          code: 'acme',
          membership_id: 1,
          role: 'owner',
        },
      })
    ).toBe(false)
  })

  test('denies runtime configuration to enterprise administrators, regular members, and signed-out users', () => {
    expect(canManageSearch(null)).toBe(false)
    expect(canManageSearch({ id: 3, username: 'member', role: 1 })).toBe(false)
    expect(
      canManageSearch({
        id: 4,
        username: 'enterprise-admin',
        role: 1,
        enterprise: {
          id: 7,
          name: 'Acme',
          code: 'acme',
          membership_id: 2,
          role: 'admin',
        },
      })
    ).toBe(false)
  })

  test('limits gateway-wide usage views to platform administrators', () => {
    expect(canViewSearchUsage({ id: 1, username: 'root', role: 100 })).toBe(
      true
    )
    expect(
      canViewSearchUsage({
        id: 2,
        username: 'owner',
        role: 1,
        enterprise: {
          id: 7,
          name: 'Acme',
          code: 'acme',
          membership_id: 1,
          role: 'owner',
        },
      })
    ).toBe(false)
  })
})
