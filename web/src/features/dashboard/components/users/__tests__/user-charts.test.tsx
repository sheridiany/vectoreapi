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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { createInstance } from 'i18next'
import { useState } from 'react'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { describe, expect, test, vi } from 'vitest'

import type { UserChartsFilters } from '@/features/dashboard/types'

import { UserCharts } from '../user-charts'

const getUserQuotaDataByUsers = vi.hoisted(() => vi.fn())

vi.mock('@/features/dashboard/api', () => ({
  getUserQuotaDataByUsers,
}))

vi.mock('@/context/theme-provider', () => ({
  useTheme: () => ({ resolvedTheme: 'light' }),
}))

vi.mock('@visactor/react-vchart', () => ({
  VChart: () => null,
}))

vi.mock('@visactor/vchart', () => ({
  ThemeManager: { setCurrentTheme: vi.fn() },
}))

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'User Consumption Ranking': 'User Consumption Ranking',
        'User Consumption Trend': 'User Consumption Trend',
        'Top Users': 'Top Users',
        'Top {{count}}': 'Top {{count}}',
        'Total:': 'Total:',
        Share: 'Share',
        'No data available': 'No data available',
        Hour: 'Hour',
        Day: 'Day',
        Week: 'Week',
        '1 Day': '1 Day',
        '7 Days': '7 Days',
        '14 Days': '14 Days',
        '29 Days': '29 Days',
      },
    },
  },
})

function renderCharts(enterpriseId?: number) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  function Harness() {
    const [filters, setFilters] = useState<UserChartsFilters>({
      timeGranularity: 'day',
      selectedRange: 7,
      topUserLimit: 10,
    })
    return (
      <UserCharts
        filters={filters}
        enterpriseId={enterpriseId}
        onFiltersChange={setFilters}
      />
    )
  }

  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <Harness />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

describe('user charts enterprise scope', () => {
  test('requests data for the enterprise selected by the dashboard parent', async () => {
    getUserQuotaDataByUsers.mockResolvedValue({ success: true, data: [] })
    renderCharts(2)

    await waitFor(() => {
      expect(getUserQuotaDataByUsers).toHaveBeenLastCalledWith(
        expect.objectContaining({ enterprise_id: 2 })
      )
    })
  })

  test('renders user consumption as ranked cards instead of a bar chart', async () => {
    getUserQuotaDataByUsers.mockResolvedValue({
      success: true,
      data: [
        {
          user_id: 101,
          username: 'alice-login',
          display_name: 'Alice Chen',
          quota: 80,
          created_at: 1,
        },
        { username: 'Bob', quota: 20, created_at: 1 },
      ],
    })
    renderCharts()

    expect(
      await screen.findByRole('list', { name: 'User Consumption Ranking' })
    ).toBeInTheDocument()
    expect(screen.getByText('Alice Chen')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })

  test('does not render a page-local enterprise selector', () => {
    getUserQuotaDataByUsers.mockResolvedValue({ success: true, data: [] })

    renderCharts()

    expect(
      screen.queryByRole('combobox', { name: 'Enterprise filter' })
    ).not.toBeInTheDocument()
  })
})
