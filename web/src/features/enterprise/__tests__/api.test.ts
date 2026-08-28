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

import { getAllEnterprises } from '../api'

vi.mock('@/lib/api', () => ({
  api: {
    get: vi.fn(),
  },
}))

describe('enterprise list API', () => {
  beforeEach(() => vi.clearAllMocks())

  test('loads every page when more than 100 enterprises exist', async () => {
    vi.mocked(api.get)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            items: [{ id: 1, name: 'First' }],
            total: 201,
            page: 1,
            page_size: 100,
          },
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            items: [{ id: 101, name: 'Second page' }],
            total: 201,
            page: 2,
            page_size: 100,
          },
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            items: [{ id: 201, name: 'Third page' }],
            total: 201,
            page: 3,
            page_size: 100,
          },
        },
      })

    const result = await getAllEnterprises()

    expect(api.get).toHaveBeenCalledTimes(3)
    expect(api.get).toHaveBeenNthCalledWith(2, '/api/enterprise/admin/', {
      params: { p: 2, page_size: 100 },
    })
    expect(api.get).toHaveBeenNthCalledWith(3, '/api/enterprise/admin/', {
      params: { p: 3, page_size: 100 },
    })
    expect(result.data?.items.map((enterprise) => enterprise.id)).toEqual([
      1, 101, 201,
    ])
  })
})
