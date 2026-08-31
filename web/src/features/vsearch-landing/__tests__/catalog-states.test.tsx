/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { VSearchLanding } from '..'
import { fetchPublicSearchCatalog } from '../api'

const authState = vi.hoisted(() => ({
  user: null as null | { id: number },
  accessToken: null as string | null,
}))

vi.mock('../api', () => ({
  fetchPublicSearchCatalog: vi.fn(),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (
    selector: (state: {
      auth: { user: null | { id: number }; accessToken: string | null }
    }) => unknown
  ) =>
    selector({
      auth: { user: authState.user, accessToken: authState.accessToken },
    }),
}))

vi.mock('@/components/layout', () => ({
  PublicLayout: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    children,
    search,
    to,
    ...props
  }: {
    children: React.ReactNode
    search?: Record<string, string>
    to: string
  }) => {
    const query = search ? `?${new URLSearchParams(search).toString()}` : ''
    return (
      <a href={`${to}${query}`} {...props}>
        {children}
      </a>
    )
  },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const catalogFixture = [
  {
    id: 'content-search',
    name: 'Content Search',
    category: 'Social media',
    description: 'Search public content across supported platforms.',
    status: 'available' as const,
    enabled: true,
    interface_count: 3,
    available_interface_count: 2,
    supported_platforms: ['TikTok', 'Douyin'],
    price_min_micros: 200_000,
    price_max_micros: 200_000,
    last_synced_at: 1_788_000_000,
  },
  {
    id: 'url-extraction',
    name: 'URL Extraction',
    category: 'Extraction',
    description: 'Extract readable content from public URLs.',
    status: 'catalog' as const,
    enabled: true,
    interface_count: 1,
    supported_platforms: ['Jina'],
  },
  {
    id: 'creator-profile',
    name: 'Creator Profile',
    category: 'Social media',
    description: 'Read public creator profile data.',
    status: 'unavailable' as const,
    enabled: true,
    interface_count: 2,
    available_interface_count: 0,
    supported_platforms: ['TikTok'],
    cost_label: 'Provider price pending',
  },
  {
    id: 'company-search',
    name: 'Company Search',
    category: 'Recruiting',
    description: 'Search public company records.',
    status: 'disabled' as const,
    enabled: false,
    interface_count: 2,
    supported_platforms: ['LinkedIn'],
  },
]

function renderLanding() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <VSearchLanding />
    </QueryClientProvider>
  )
}

describe('public vSearch landing catalog', () => {
  beforeEach(() => {
    authState.user = null
    authState.accessToken = null
    vi.clearAllMocks()
  })

  test('shows a live loading state while the catalog request is pending', () => {
    vi.mocked(fetchPublicSearchCatalog).mockReturnValue(new Promise(() => {}))

    renderLanding()

    expect(screen.getByText('Reading live capability index')).toBeVisible()
    const liveIndex = screen
      .getByText('World to standard capability')
      .closest('aside')
    if (!liveIndex) throw new Error('Public capability index is missing')
    expect(within(liveIndex).getAllByText('—')).toHaveLength(3)
  })

  test('renders the standard capability contract and preserves the catalog redirect for guests', async () => {
    vi.mocked(fetchPublicSearchCatalog).mockResolvedValue(catalogFixture)

    renderLanding()

    expect(await screen.findByText('Live sources')).toBeVisible()

    const liveIndex = screen
      .getByText('World to standard capability')
      .closest('aside')
    if (!liveIndex) throw new Error('Public capability index is missing')
    expect(within(liveIndex).getByText('4')).toBeVisible()
    expect(within(liveIndex).getByText('8')).toBeVisible()
    expect(within(liveIndex).getByText('2')).toBeVisible()

    const hero = screen
      .getByRole('heading', {
        name: 'Turn the real world, into callable capabilities.',
      })
      .closest('section')
    if (!hero) throw new Error('vSearch hero is missing')
    const standardLayer = within(hero).getByText('Standard layer').parentElement
    if (!standardLayer) throw new Error('Standard layer summary is missing')
    expect(within(standardLayer).getByText('4')).toBeVisible()
    expect(within(standardLayer).getByText('8')).toBeVisible()
    expect(
      screen.getByRole('heading', {
        name: 'Users see capabilities, not providers.',
      })
    ).toBeVisible()
    expect(
      screen.getByRole('heading', {
        name: 'One contract connects every public source.',
      })
    ).toBeVisible()
    expect(screen.getAllByText('vSearch.query').length).toBeGreaterThan(0)
    expect(screen.getAllByText('vSearch.extract').length).toBeGreaterThan(0)
    expect(screen.getAllByText('vSearch.creator').length).toBeGreaterThan(0)
    expect(screen.getByText(/POST\s+\/v1\/content\/search/)).toBeVisible()
    expect(screen.getByText(/"source": "web"/)).toBeVisible()
    expect(
      screen.getByRole('link', { name: 'View standard contract' })
    ).toHaveAttribute('href', '#contract')
    expect(document.querySelector('#contract')).not.toBeNull()

    const guestCatalogLinks = [
      ...screen.getAllByRole('link', { name: 'Start with vSearch' }),
      screen.getByRole('link', { name: 'View access guide' }),
    ]
    expect(guestCatalogLinks.length).toBeGreaterThan(0)
    for (const link of guestCatalogLinks) {
      expect(link).toHaveAttribute(
        'href',
        '/sign-in?redirect=%2Fsearch%2Fcatalog'
      )
    }
  })

  test('shows an explicit empty state when no capability contract is published', async () => {
    vi.mocked(fetchPublicSearchCatalog).mockResolvedValue([])

    renderLanding()

    expect(await screen.findByText('Live sources')).toBeVisible()
    const liveIndex = screen
      .getByText('World to standard capability')
      .closest('aside')
    if (!liveIndex) throw new Error('Public capability index is missing')
    expect(within(liveIndex).getAllByText('0')).toHaveLength(3)
  })

  test('keeps the product contract visible without inventing live metrics when the index fails', async () => {
    vi.mocked(fetchPublicSearchCatalog).mockRejectedValue(new Error('network'))

    renderLanding()

    expect(await screen.findByText('Live index unavailable')).toBeVisible()
    const liveIndex = screen
      .getByText('World to standard capability')
      .closest('aside')
    if (!liveIndex) throw new Error('Public capability index is missing')
    expect(within(liveIndex).getAllByText('—')).toHaveLength(3)
    expect(screen.getByText('Canonical contract')).toBeVisible()
    expect(fetchPublicSearchCatalog).toHaveBeenCalledTimes(1)
  })

  test('takes an authenticated user directly to the complete catalog', async () => {
    authState.user = { id: 1 }
    authState.accessToken = 'access-token'
    vi.mocked(fetchPublicSearchCatalog).mockResolvedValue(catalogFixture)

    renderLanding()

    expect(await screen.findByText('Live sources')).toBeVisible()
    const catalogLinks = screen.getAllByRole('link', {
      name: 'Open vSearch catalog',
    })
    expect(catalogLinks).toHaveLength(1)
    for (const link of catalogLinks) {
      expect(link).toHaveAttribute('href', '/search/catalog')
    }
    expect(
      screen.getByRole('link', { name: 'View access guide' })
    ).toHaveAttribute('href', '/search/catalog')
  })
})
