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

import { DashboardEnterpriseSelector } from '../enterprise-selector'

const getAllEnterprises = vi.hoisted(() => vi.fn())
const toastError = vi.hoisted(() => vi.fn())

vi.mock('@/features/enterprise/api', () => ({
  getAllEnterprises,
}))

vi.mock('sonner', () => ({
  toast: { error: toastError },
}))

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'All enterprises': 'All enterprises',
        Disabled: 'Disabled',
        'Enterprise filter': 'Enterprise filter',
        'Selected enterprise is no longer available':
          'Selected enterprise is no longer available',
        'Unable to load enterprises': 'Unable to load enterprises',
      },
    },
  },
})

function renderSelector(onChange = vi.fn(), enterpriseId?: number) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return {
    onChange,
    ...render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={queryClient}>
          <DashboardEnterpriseSelector
            enterpriseId={enterpriseId}
            onChange={onChange}
          />
        </QueryClientProvider>
      </I18nextProvider>
    ),
  }
}

describe('dashboard enterprise selector', () => {
  beforeEach(() => vi.clearAllMocks())

  test('selects an enterprise and returns to all enterprises with keyboard-accessible options', async () => {
    getAllEnterprises.mockResolvedValue({
      success: true,
      data: {
        items: [
          { id: 2, name: 'Star Data', code: 'star-data', status: 1 },
          { id: 3, name: 'Vector Epoch', code: 'vector-epoch', status: 1 },
        ],
      },
    })
    const onChange = vi.fn()
    const user = userEvent.setup()
    renderSelector(onChange)

    const trigger = await screen.findByRole('combobox', {
      name: 'Enterprise filter',
    })
    trigger.focus()
    await user.keyboard('{Enter}')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{ArrowDown}{Enter}')
    expect(onChange).toHaveBeenLastCalledWith(2)

    await user.keyboard('{Enter}{Home}{Enter}')
    expect(onChange).toHaveBeenLastCalledWith(undefined)
  })

  test('keeps a selected enterprise id visible while the option list loads', () => {
    getAllEnterprises.mockReturnValue(new Promise(() => {}))

    renderSelector(vi.fn(), 42)

    expect(
      screen.getByRole('combobox', { name: 'Enterprise filter' })
    ).toHaveTextContent('42')
  })

  test('shows long enterprise names in a truncated trigger container', async () => {
    const longName =
      'Vector Epoch International Artificial Intelligence Operations Company'
    getAllEnterprises.mockResolvedValue({
      success: true,
      data: { items: [{ id: 9, name: longName, code: 'vector', status: 1 }] },
    })

    renderSelector(vi.fn(), 9)

    expect(await screen.findByText(longName)).toHaveClass('truncate')
  })

  test('keeps disabled enterprises selectable and marks their status', async () => {
    getAllEnterprises.mockResolvedValue({
      success: true,
      data: {
        items: [{ id: 8, name: 'Paused Company', code: 'paused', status: 2 }],
      },
    })
    const onChange = vi.fn()
    const user = userEvent.setup()
    renderSelector(onChange)

    const trigger = await screen.findByRole('combobox', {
      name: 'Enterprise filter',
    })
    await user.click(trigger)
    await user.click(
      screen.getByRole('option', { name: 'Paused Company · Disabled' })
    )

    expect(onChange).toHaveBeenCalledWith(8)
  })

  test('reports and clears an enterprise id that is no longer available', async () => {
    getAllEnterprises.mockResolvedValue({
      success: true,
      data: { items: [] },
    })
    const onChange = vi.fn()

    renderSelector(onChange, 42)

    await waitFor(() => expect(onChange).toHaveBeenCalledWith(undefined))
    expect(toastError).toHaveBeenCalledWith(
      'Selected enterprise is no longer available'
    )
  })
})
