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
*/
import { afterEach, describe, expect, test } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  requireEnterpriseMember,
  requireEnterpriseViewer,
} from '../route-guard'

function expectRedirect(callback: () => unknown, destination: string) {
  try {
    callback()
    throw new Error('expected a redirect')
  } catch (error) {
    expect(error).toBeInstanceOf(Response)
    expect(
      (error as Response & { options?: { to?: string } }).options?.to
    ).toBe(destination)
  }
}

afterEach(() => {
  useAuthStore.getState().auth.setUser(null)
})

describe('enterprise route guards', () => {
  test('redirects platform administrators to platform enterprise management', () => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'root',
      role: ROLE.SUPER_ADMIN,
    })

    expectRedirect(requireEnterpriseViewer, '/enterprise/admin')
  })

  test('rejects authenticated users without an enterprise membership', () => {
    useAuthStore.getState().auth.setUser({
      id: 2,
      username: 'member',
      role: ROLE.USER,
    })

    expectRedirect(requireEnterpriseMember, '/403')
  })

  test('allows an enterprise auditor to open scoped reporting pages', () => {
    const user = {
      id: 3,
      username: 'auditor',
      role: ROLE.USER,
      enterprise: {
        id: 10,
        name: 'Acme',
        code: 'acme',
        membership_id: 11,
        role: 'auditor' as const,
      },
    }
    useAuthStore.getState().auth.setUser(user)

    expect(requireEnterpriseViewer()).toBe(user)
  })
})
