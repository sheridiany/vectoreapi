/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { fetchSearchCatalog } from '../api'
import { SearchCatalogPage } from '../search-catalog-page'

vi.mock('../api', () => ({ fetchSearchCatalog: vi.fn() }))

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    children,
    to,
    ...props
  }: {
    children: React.ReactNode
    to: string
  }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function renderCatalog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <SearchCatalogPage />
    </QueryClientProvider>
  )
}

describe('vSearch capability page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(fetchSearchCatalog).mockResolvedValue([
      {
        id: 'brave',
        name: 'Brave Search',
        category: 'Search',
        description: 'Search the public web.',
        status: 'available',
        enabled: true,
        interface_count: 3,
        recent_latency_ms: 280,
        price_min_micros: 1,
        price_max_micros: 1,
      },
      {
        id: 'firecrawl',
        name: 'Firecrawl',
        category: 'Extract',
        description: 'Extract readable pages.',
        status: 'available',
        enabled: true,
        interface_count: 2,
      },
    ])
  })

  test('loads the live catalog without in-page navigation tabs', async () => {
    renderCatalog()

    expect(await screen.findByText('Brave Search')).toBeInTheDocument()
    expect(fetchSearchCatalog).toHaveBeenCalledOnce()
    expect(
      screen.getAllByText((content) => content.includes('0.000001')).length
    ).toBeGreaterThan(0)
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('navigation', { name: 'Search navigation' })
    ).not.toBeInTheDocument()
  })

  test('filters live capabilities by category and search term', async () => {
    const user = userEvent.setup()
    renderCatalog()

    await screen.findByText('Brave Search')
    await user.click(screen.getByRole('button', { name: 'Extract' }))
    expect(screen.getByText('Firecrawl')).toBeInTheDocument()
    expect(screen.queryByText('Brave Search')).not.toBeInTheDocument()

    await user.type(
      screen.getByRole('textbox', { name: 'Search capability catalog' }),
      'missing service'
    )
    expect(screen.getByText('No matching capabilities')).toBeInTheDocument()
  })

  test('offers retry when the catalog endpoint fails', async () => {
    vi.mocked(fetchSearchCatalog).mockRejectedValueOnce(new Error('network'))
    renderCatalog()

    expect(
      await screen.findByText('Failed to load capability catalog')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })
})
