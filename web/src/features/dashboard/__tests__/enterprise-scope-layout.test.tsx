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
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { Dashboard } from '../index'

const routeState = vi.hoisted(() => ({
  section: 'models',
  enterpriseId: 7 as number | undefined,
}))
const navigate = vi.hoisted(() => vi.fn())
const getAllEnterprises = vi.hoisted(() => vi.fn())
const toastError = vi.hoisted(() => vi.fn())

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const original =
    await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...original,
    getRouteApi: () => ({
      useParams: () => ({ section: routeState.section }),
      useSearch: () => ({ enterprise_id: routeState.enterpriseId }),
    }),
    useNavigate: () => navigate,
  }
})

vi.mock('@/features/enterprise/api', () => ({
  getAllEnterprises,
}))

vi.mock('sonner', () => ({
  toast: { error: toastError },
}))

vi.mock('@/components/page-transition', () => ({
  FadeIn: (props: { children: React.ReactNode }) => props.children,
}))

vi.mock(
  '@/features/dashboard/components/models/models-chart-preferences',
  () => ({
    ModelsChartPreferences: () => <button type='button'>Preferences</button>,
  })
)

vi.mock('@/features/dashboard/components/models/models-filter-dialog', () => ({
  ModelsFilter: () => <button type='button'>Filter</button>,
}))

vi.mock('@/features/dashboard/components/overview/overview-dashboard', () => ({
  OverviewDashboard: () => <div>Overview content</div>,
}))

vi.mock('@/features/dashboard/components/models/log-stat-cards', () => ({
  LogStatCards: () => <div>Stats</div>,
}))

vi.mock('@/features/dashboard/components/models/model-charts', () => ({
  ModelCharts: () => <div>Model charts</div>,
}))

vi.mock(
  '@/features/dashboard/components/models/consumption-distribution-chart',
  () => ({
    ConsumptionDistributionChart: () => <div>Consumption chart</div>,
  })
)

vi.mock('@/features/dashboard/components/models/performance-overview', () => ({
  PerformanceOverview: () => <div>Performance</div>,
}))

vi.mock('@/features/dashboard/components/users/user-charts', () => ({
  UserCharts: () => <div>User charts</div>,
}))

vi.mock('@/features/dashboard/components/flow/flow-charts', () => ({
  FlowCharts: () => <div>Flow charts</div>,
}))

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'All enterprises': 'All enterprises',
        'Enterprise filter': 'Enterprise filter',
        Filter: 'Filter',
        Flow: 'Flow',
        'Model Call Analytics': 'Model Call Analytics',
        'No permission to perform this action':
          'No permission to perform this action',
        Overview: 'Overview',
        Preferences: 'Preferences',
        'User Analytics': 'User Analytics',
        'Unable to load enterprises': 'Unable to load enterprises',
      },
    },
  },
})

function renderDashboard() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <Dashboard />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

describe('dashboard enterprise scope layout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeState.section = 'models'
    routeState.enterpriseId = 7
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'root',
      role: ROLE.SUPER_ADMIN,
    })
    getAllEnterprises.mockResolvedValue({
      success: true,
      data: {
        items: [{ id: 7, name: 'Vector Epoch', code: 'vector', status: 1 }],
      },
    })
  })

  test('shows the root enterprise selector before model page actions', async () => {
    renderDashboard()

    const selector = await screen.findByRole('combobox', {
      name: 'Enterprise filter',
    })
    const preferences = screen.getByRole('button', { name: 'Preferences' })

    expect(
      selector.compareDocumentPosition(preferences) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  test('does not show the enterprise selector on overview', () => {
    routeState.section = 'overview'
    renderDashboard()

    expect(screen.getByText('Overview content')).toBeInTheDocument()
    expect(
      screen.queryByRole('combobox', { name: 'Enterprise filter' })
    ).not.toBeInTheDocument()
  })

  test('removes a copied enterprise scope for non-root administrators', async () => {
    useAuthStore.getState().auth.setUser({
      id: 2,
      username: 'admin',
      role: ROLE.ADMIN,
    })

    renderDashboard()

    expect(
      screen.queryByRole('combobox', { name: 'Enterprise filter' })
    ).not.toBeInTheDocument()
    expect(screen.queryByText('Stats')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        'No permission to perform this action'
      )
    )
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({
        to: '/dashboard/$section',
        params: { section: 'models' },
        search: { enterprise_id: undefined },
        replace: true,
      })
    )
  })

  test('preserves the selected enterprise when switching dashboard tabs', async () => {
    const user = userEvent.setup()
    renderDashboard()

    await user.click(screen.getByRole('tab', { name: 'Flow' }))

    expect(navigate).toHaveBeenCalledWith({
      to: '/dashboard/$section',
      params: { section: 'flow' },
      search: { enterprise_id: 7 },
    })
  })
})
