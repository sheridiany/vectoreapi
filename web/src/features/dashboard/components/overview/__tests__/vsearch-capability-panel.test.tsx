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

import { fetchSearchCatalog } from '@/features/search/api'

import { VSearchCapabilityPanel } from '../vsearch-capability-panel'

vi.mock('@/features/search/api', () => ({ fetchSearchCatalog: vi.fn() }))

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
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) =>
      key
        .replace('{{count}}', String(options?.count ?? ''))
        .replace('{{total}}', String(options?.total ?? '')),
  }),
}))

function renderPanel() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <VSearchCapabilityPanel />
    </QueryClientProvider>
  )
}

describe('dashboard vSearch capability panel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(fetchSearchCatalog).mockResolvedValue([
      {
        id: 'social-search',
        name: 'Social content search',
        category: 'Social media',
        description: 'Search public social content.',
        status: 'available',
        contract_status: 'verified',
        enabled: true,
        interface_count: 3,
        available_interface_count: 1,
        supported_platforms: ['TikTok', 'Douyin', 'Xiaohongshu'],
      },
      {
        id: 'product-search',
        name: 'Product search',
        category: 'Commerce',
        description: 'Search public products.',
        status: 'catalog',
        contract_status: 'unverified',
        enabled: false,
        interface_count: 2,
        available_interface_count: 0,
        supported_platforms: ['Taobao', 'TikTok Shop'],
      },
    ])
  })

  test('shows standard capability coverage separately from callable readiness', async () => {
    renderPanel()

    expect(
      await screen.findByRole('heading', { name: 'vSearch data capabilities' })
    ).toBeInTheDocument()
    expect(await screen.findByText('Social content search')).toBeInTheDocument()
    expect(screen.getByText('2 standard capabilities')).toBeInTheDocument()
    expect(screen.getByText('1 ready to call')).toBeInTheDocument()
    expect(screen.getByText('Product search')).toBeInTheDocument()
    expect(screen.getByText('TikTok')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Explore vSearch' })
    ).toHaveAttribute('href', '/search/catalog')
  })
})
