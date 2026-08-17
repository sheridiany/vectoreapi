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
  canAppointEnterpriseOwner,
  canManageEnterprise,
  canViewEnterprise,
} from '../permissions'

describe('enterprise permissions', () => {
  test('root, owner, and admin can manage enterprise data', () => {
    expect(canManageEnterprise({ role: 100 })).toBe(true)
    expect(
      canManageEnterprise({ role: 1, enterprise: { role: 'owner' } })
    ).toBe(true)
    expect(
      canManageEnterprise({ role: 1, enterprise: { role: 'admin' } })
    ).toBe(true)
  })

  test('member and auditor cannot manage enterprise data', () => {
    expect(
      canManageEnterprise({ role: 1, enterprise: { role: 'member' } })
    ).toBe(false)
    expect(
      canManageEnterprise({ role: 1, enterprise: { role: 'auditor' } })
    ).toBe(false)
    expect(canManageEnterprise(null)).toBe(false)
  })

  test('auditor can view enterprise reporting without managing it', () => {
    expect(
      canViewEnterprise({ role: 1, enterprise: { role: 'auditor' } })
    ).toBe(true)
    expect(canViewEnterprise({ role: 1, enterprise: { role: 'member' } })).toBe(
      false
    )
  })

  test('only root can appoint an enterprise owner', () => {
    expect(canAppointEnterpriseOwner({ role: 100 })).toBe(true)
    expect(
      canAppointEnterpriseOwner({ role: 1, enterprise: { role: 'owner' } })
    ).toBe(false)
  })
})
