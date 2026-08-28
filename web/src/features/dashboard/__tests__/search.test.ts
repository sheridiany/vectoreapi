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

import { dashboardSearchSchema } from '../search'

describe('dashboard enterprise search parameter', () => {
  test('uses all enterprises only when the parameter is absent', () => {
    expect(dashboardSearchSchema.parse({})).toEqual({})
  })

  test('accepts a positive integer enterprise id from a copied URL', () => {
    expect(dashboardSearchSchema.parse({ enterprise_id: '23' })).toEqual({
      enterprise_id: 23,
    })
  })

  test.each(['', '0', '-1', '1.5', 'not-a-number'])(
    'rejects invalid enterprise id %s instead of broadening to all enterprises',
    (enterpriseId) => {
      expect(() =>
        dashboardSearchSchema.parse({ enterprise_id: enterpriseId })
      ).toThrow()
    }
  )
})
