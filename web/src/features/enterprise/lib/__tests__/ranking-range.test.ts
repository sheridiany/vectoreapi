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

import { getCustomRankingRange } from '../ranking-range'

describe('enterprise ranking date range', () => {
  test('includes the full selected end date', () => {
    const range = getCustomRankingRange('2026-08-10', '2026-08-10')

    expect(range).toEqual({
      start: new Date('2026-08-10T00:00:00').getTime() / 1000,
      end: new Date('2026-08-10T23:59:59').getTime() / 1000,
    })
  })

  test('rejects incomplete or reversed dates', () => {
    expect(getCustomRankingRange('', '2026-08-10')).toBeUndefined()
    expect(getCustomRankingRange('2026-08-11', '2026-08-10')).toBeUndefined()
  })
})
