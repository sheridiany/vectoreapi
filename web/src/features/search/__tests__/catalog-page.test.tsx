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
        contract_status: 'verified',
        enabled: true,
        interface_count: 3,
        available_interface_count: 2,
        supported_platforms: [
          'TikTok',
          'Douyin',
          'Xiaohongshu',
          'tiktok_shop',
          'Weibo',
          'wechat_mp',
          'wechat_channels',
          'YouTube',
          'Reddit',
          'LinkedIn',
        ],
        request_parameters: ['platform', 'query'],
        information_fields: ['title', 'rank', 'score'],
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
        contract_status: 'verified',
        enabled: true,
        interface_count: 2,
        available_interface_count: 1,
        supported_platforms: ['Web'],
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
    expect(screen.getAllByText('Accessible information').length).toBe(2)
    expect(screen.getByText('Keyword')).toBeInTheDocument()
    expect(screen.getByText('Popularity')).toBeInTheDocument()
    const platformFilters = screen.getByRole('group', {
      name: 'Catalog platforms',
    })
    for (const label of [
      '微博',
      '微信公众号',
      '微信视频号',
      'YouTube',
      'Reddit',
      'LinkedIn',
    ]) {
      expect(platformFilters).toHaveTextContent(label)
    }
    expect(
      screen.getByText('Information contract pending verification')
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('navigation', { name: 'Search navigation' })
    ).not.toBeInTheDocument()
  })

  test('filters live capabilities by platform and search term', async () => {
    const user = userEvent.setup()
    renderCatalog()

    await screen.findByText('Brave Search')
    await user.click(screen.getByRole('button', { name: '抖音' }))
    expect(screen.getByText('Brave Search')).toBeInTheDocument()
    expect(screen.queryByText('Firecrawl')).not.toBeInTheDocument()

    await user.type(
      screen.getByRole('textbox', { name: 'Search capability catalog' }),
      'missing service'
    )
    expect(screen.getByText('No matching capabilities')).toBeInTheDocument()
  })

  test('searches by supported platform', async () => {
    const user = userEvent.setup()
    renderCatalog()

    await screen.findByText('Brave Search')
    await user.type(
      screen.getByRole('textbox', { name: 'Search capability catalog' }),
      'tiktok'
    )

    expect(screen.getByText('Brave Search')).toBeInTheDocument()
    expect(screen.queryByText('Firecrawl')).not.toBeInTheDocument()
  })

  test('distinguishes cataloged interfaces, callable interfaces, and rollout status', async () => {
    vi.mocked(fetchSearchCatalog).mockResolvedValueOnce([
      {
        id: 'available-capability',
        name: 'Available capability',
        category: 'Search',
        description: 'Ready to call.',
        status: 'available',
        contract_status: 'verified',
        enabled: true,
        interface_count: 3,
        available_interface_count: 2,
        supported_platforms: ['TikTok', 'Douyin'],
      },
      {
        id: 'draft-capability',
        name: 'Draft capability',
        category: 'Social media',
        description: 'Being prepared.',
        status: 'catalog',
        contract_status: 'unverified',
        enabled: false,
        interface_count: 2,
        available_interface_count: 0,
        supported_platforms: ['Reddit'],
      },
      {
        id: 'unrouted-capability',
        name: 'Unrouted capability',
        category: 'Business',
        description: 'Published without a healthy route.',
        status: 'unavailable',
        contract_status: 'verified',
        enabled: true,
        interface_count: 1,
        available_interface_count: 0,
        supported_platforms: ['LinkedIn'],
      },
    ])

    renderCatalog()

    expect(await screen.findByText('Available capability')).toBeInTheDocument()
    expect(screen.getByText('Preparing')).toBeInTheDocument()
    expect(screen.getAllByText('Planned information')).toHaveLength(2)
    expect(screen.getByText('Temporarily unavailable')).toBeInTheDocument()
    expect(screen.getAllByText('TikTok').length).toBeGreaterThan(0)
    expect(screen.getAllByText('抖音').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Reddit').length).toBeGreaterThan(0)
    expect(screen.getAllByText('LinkedIn').length).toBeGreaterThan(0)
    expect(
      screen.getByText('Cataloged interfaces').parentElement
    ).toHaveTextContent('6')
    expect(
      screen.getByText('Callable interfaces').parentElement
    ).toHaveTextContent('2')
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
