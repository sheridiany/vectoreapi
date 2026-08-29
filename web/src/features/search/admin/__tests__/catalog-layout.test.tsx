/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { fetchAdminSearchCatalog, type SearchAdminCatalogItem } from '../../api'
import { SearchAdminCatalogPage } from '../catalog-config-page'

vi.mock('../../api', () => ({
  fetchAdminSearchCatalog: vi.fn(),
  fetchSearchCapabilityEnterpriseGrants: vi.fn(),
  fetchSearchGrantEnterprises: vi.fn(),
  publishAdminSearchCatalog: vi.fn(),
  syncAdminSearchCatalog: vi.fn(),
  updateAdminSearchCatalogItem: vi.fn(),
  updateSearchCapabilityEnterpriseGrants: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

describe('vSearch admin catalog layout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  test('wraps long desktop descriptions before they can cover later columns', async () => {
    const longDescription = `每日天气预报 API https://example.com/${'unbroken-capability-parameter-description'.repeat(12)}`
    const capability: SearchAdminCatalogItem = {
      id: 'qweather',
      name: 'QWeather',
      category: 'Weather',
      description: longDescription,
      status: 'unavailable',
      schema_status: 'unavailable',
      enabled: false,
      interface_count: 0,
      upstream_cost_micros: 100_000,
      price_micros: 100_000,
    }
    vi.mocked(fetchAdminSearchCatalog).mockResolvedValue([capability])
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <SearchAdminCatalogPage />
      </QueryClientProvider>
    )

    const descriptions = await screen.findAllByText(longDescription)
    const desktopDescription = descriptions.find((description) =>
      description.closest('[data-slot="table-cell"]')
    )

    expect(desktopDescription).toBeDefined()
    expect(desktopDescription).toHaveClass('whitespace-normal', 'break-words')
  })
})
