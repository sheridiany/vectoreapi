/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { fetchSearchUsageLogs, fetchSearchUsageStats } from '../api'
import { SearchLogsPage } from '../search-logs-page'

vi.mock('../api', () => ({
  fetchSearchUsageLogs: vi.fn(),
  fetchSearchUsageStats: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function renderLogs() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <SearchLogsPage />
    </QueryClientProvider>
  )
}

describe('vSearch user logs page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(fetchSearchUsageLogs).mockResolvedValue({
      items: [
        {
          id: 'log-1',
          created_at: '2026-08-28T10:30:00Z',
          service: 'Brave Search',
          endpoint: '/v1/search',
          status: 'pending',
          latency_ms: 1500,
          agent_key_name: 'research-bot',
          request_id: 'request-1',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(fetchSearchUsageStats).mockResolvedValue({
      total_requests: 1,
      success_requests: 1,
      error_requests: 0,
      success_rate: 100,
      average_latency_ms: 1500,
      quota: 12,
    })
  })

  test('uses the dedicated vSearch log and statistics endpoints', async () => {
    renderLogs()

    expect((await screen.findAllByText('Brave Search')).length).toBeGreaterThan(
      0
    )
    expect(screen.getAllByText('/v1/search').length).toBeGreaterThan(0)
    expect(screen.getAllByText('research-bot').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Pending').length).toBeGreaterThan(0)
    expect(fetchSearchUsageLogs).toHaveBeenCalledWith({
      page: 1,
      page_size: 20,
      range: 30,
      query: undefined,
    })
    expect(fetchSearchUsageStats).toHaveBeenCalledWith({
      range: 30,
      query: undefined,
    })
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })

  test('refetches when the service or AgentKey query changes', async () => {
    const user = userEvent.setup()
    renderLogs()
    await screen.findAllByText('Brave Search')

    await user.type(
      screen.getByRole('textbox', { name: 'Search logs' }),
      'research-bot'
    )

    await waitFor(() =>
      expect(fetchSearchUsageLogs).toHaveBeenLastCalledWith(
        expect.objectContaining({ query: 'research-bot', page: 1 })
      )
    )
  })

  test('shows a retry action when the log request fails', async () => {
    vi.mocked(fetchSearchUsageLogs).mockRejectedValueOnce(new Error('network'))
    renderLogs()

    expect(
      await screen.findByText('Failed to load vSearch logs')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })
})
