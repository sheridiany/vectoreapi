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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  getDashboardPerfMetricsSummary,
  getFlowQuotaDates,
  getUserQuotaDataByUsers,
  getUserQuotaDates,
} from '../api'

vi.mock('@/lib/api', () => ({
  api: {
    get: vi.fn(),
  },
}))

describe('dashboard enterprise API contract', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.get).mockResolvedValue({ data: { success: true, data: [] } })
  })

  test('adds the enterprise scope to model usage requests for administrators', async () => {
    await getUserQuotaDates(
      {
        start_timestamp: 10,
        end_timestamp: 20,
        default_time: 'day',
        enterprise_id: 7,
      },
      true
    )

    expect(api.get).toHaveBeenCalledWith('/api/data', {
      params: {
        start_timestamp: 10,
        end_timestamp: 20,
        default_time: 'day',
        enterprise_id: 7,
      },
    })
  })

  test('adds the enterprise scope to flow requests for administrators', async () => {
    await getFlowQuotaDates(
      {
        start_timestamp: 10,
        end_timestamp: 20,
        enterprise_id: 7,
      },
      true
    )

    expect(api.get).toHaveBeenCalledWith('/api/data/flow', {
      params: {
        start_timestamp: 10,
        end_timestamp: 20,
        enterprise_id: 7,
      },
    })
  })

  test('adds the enterprise scope to user and performance requests', async () => {
    await getUserQuotaDataByUsers({
      start_timestamp: 10,
      end_timestamp: 20,
      enterprise_id: 7,
    })
    await getDashboardPerfMetricsSummary(24, 7)

    expect(api.get).toHaveBeenNthCalledWith(1, '/api/data/users', {
      params: {
        start_timestamp: 10,
        end_timestamp: 20,
        enterprise_id: 7,
      },
    })
    expect(api.get).toHaveBeenNthCalledWith(2, '/api/data/performance', {
      params: { hours: 24, enterprise_id: 7 },
    })
  })
})
