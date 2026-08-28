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
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { SearchAdminCatalogPage } from '../admin/catalog-config-page'
import { SearchAdminUpstreamAccountsPage } from '../admin/upstream-accounts-page'
import { SearchAdminUsageLogsPage } from '../admin/usage-logs-page'
import {
  createSearchUpstreamAccount,
  deleteSearchUpstreamAccount,
  exportAdminSearchUsageLogs,
  fetchAdminSearchCatalog,
  fetchAdminSearchUsageLogs,
  fetchAdminSearchUsageStats,
  fetchSearchCapabilityEnterpriseGrants,
  fetchSearchGrantEnterprises,
  fetchSearchUpstreamAccounts,
  syncAdminSearchCatalog,
  testSearchUpstreamAccount,
  updateSearchUpstreamAccount,
  updateAdminSearchCatalogItem,
  updateSearchCapabilityEnterpriseGrants,
} from '../api'

vi.mock('../api', () => ({
  createSearchUpstreamAccount: vi.fn(),
  deleteSearchUpstreamAccount: vi.fn(),
  fetchSearchUpstreamAccounts: vi.fn(),
  testSearchUpstreamAccount: vi.fn(),
  updateSearchUpstreamAccount: vi.fn(),
  fetchAdminSearchCatalog: vi.fn(),
  fetchSearchCapabilityEnterpriseGrants: vi.fn(),
  fetchSearchGrantEnterprises: vi.fn(),
  syncAdminSearchCatalog: vi.fn(),
  updateAdminSearchCatalogItem: vi.fn(),
  updateSearchCapabilityEnterpriseGrants: vi.fn(),
  fetchAdminSearchUsageLogs: vi.fn(),
  fetchAdminSearchUsageStats: vi.fn(),
  exportAdminSearchUsageLogs: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function renderWithQuery(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  )
}

describe('vSearch administration pages', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(fetchSearchUpstreamAccounts).mockResolvedValue([
      {
        id: 1,
        name: 'Primary account',
        provider: 'agentkey-mcp',
        base_url: 'https://api.agentkey.app/v1/mcp',
        key_prefix: 'ak_live_1234••••',
        plan: 'Pro',
        balance: 500,
        balance_micros: 1,
        weight: 10,
        priority: 0,
        pool: 'default',
        pool_id: 1,
        status: 'healthy',
        last_check: 1_787_891_200,
      },
    ])
    vi.mocked(testSearchUpstreamAccount).mockResolvedValue({
      id: 1,
      name: 'Primary account',
      provider: 'agentkey-mcp',
      base_url: 'https://api.agentkey.app/v1/mcp',
      key_prefix: 'ak_live_1234••••',
      plan: 'Pro',
      balance: 500,
      balance_micros: 500_000_000,
      weight: 10,
      priority: 0,
      pool: 'default',
      pool_id: 1,
      status: 'healthy',
      last_check: 1_787_891_200,
    })
    vi.mocked(updateSearchUpstreamAccount).mockImplementation(
      async (input) => ({
        id: input.id,
        name: input.name,
        provider: 'agentkey-mcp',
        base_url: input.base_url,
        key_prefix: 'ak_live_1234••••',
        plan: 'Pro',
        balance: 500,
        balance_micros: 500_000_000,
        weight: input.weight,
        priority: input.priority,
        pool: 'default',
        pool_id: input.pool_id,
        status: input.status,
        last_check: 1_787_891_200,
      })
    )
    vi.mocked(deleteSearchUpstreamAccount).mockResolvedValue()
    vi.mocked(fetchAdminSearchCatalog).mockResolvedValue([
      {
        id: 'brave',
        name: 'Brave Search',
        category: 'Search',
        description: 'Search the public web.',
        status: 'available',
        enabled: true,
        interface_count: 3,
        upstream_cost: 0.1,
        upstream_cost_micros: 1,
        price: 0.2,
        price_micros: 123,
      },
      {
        id: 'firecrawl',
        name: 'Firecrawl',
        category: 'Extract',
        description: 'Extract readable pages.',
        status: 'available',
        enabled: true,
        interface_count: 2,
        upstream_cost: 0.3,
        upstream_cost_micros: 2,
        price: 0.5,
        price_micros: 456,
      },
    ])
    vi.mocked(syncAdminSearchCatalog).mockResolvedValue({ synced: 2 })
    vi.mocked(fetchSearchCapabilityEnterpriseGrants).mockResolvedValue({
      capability_id: 'brave',
      access_mode: 'selected_enterprises',
      enterprise_ids: [11],
    })
    vi.mocked(fetchSearchGrantEnterprises).mockResolvedValue([
      { id: 11, name: 'Northstar Research', code: 'northstar', status: 1 },
      { id: 12, name: 'Orbit Labs', code: 'orbit', status: 1 },
    ])
    vi.mocked(updateSearchCapabilityEnterpriseGrants).mockImplementation(
      async (capabilityID, enterpriseIDs) => ({
        capability_id: capabilityID,
        access_mode:
          enterpriseIDs.length === 0
            ? 'all_enterprises'
            : 'selected_enterprises',
        enterprise_ids: enterpriseIDs,
      })
    )
    vi.mocked(updateAdminSearchCatalogItem).mockImplementation(
      async (id, patch) => ({
        id,
        name: id === 'brave' ? 'Brave Search' : 'Firecrawl',
        category: id === 'brave' ? 'Search' : 'Extract',
        description: 'Capability',
        status: 'available',
        enabled: patch.enabled ?? true,
        interface_count: 1,
        price_micros: patch.price_micros,
      })
    )
    vi.mocked(fetchAdminSearchUsageLogs).mockResolvedValue({
      items: [
        {
          id: 'log-1',
          created_at: '2026-08-28T10:30:00Z',
          service: 'Brave Search',
          endpoint: '/v1/search',
          status: 'indeterminate',
          latency_ms: 420,
          agent_key_name: 'research-bot',
          enterprise_name: 'Northstar Research',
          user_name: 'alice',
          account: 'Primary account',
          request_id: 'request-precise-1',
          error_code: 'UPSTREAM_TIMEOUT',
          upstream_cost: 0.2,
          upstream_cost_micros: 1,
          charge: 0.32,
          charge_micros: 10,
          profit: 0.12,
          profit_micros: 9,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(fetchAdminSearchUsageStats).mockResolvedValue({
      total_requests: 12,
      success_requests: 11,
      error_requests: 1,
      pending_requests: 0,
      indeterminate_requests: 1,
      success_rate: 91.7,
      average_latency_ms: 420,
      upstream_cost: 2.86,
      upstream_cost_micros: 3,
      revenue: 3.5,
      revenue_micros: 9,
      profit: 0.64,
      profit_micros: 6,
    })
    vi.mocked(exportAdminSearchUsageLogs).mockResolvedValue(
      new Blob(['time,service'])
    )
  })

  test('connects and health-checks upstream AgentKey accounts', async () => {
    vi.mocked(createSearchUpstreamAccount).mockResolvedValue({
      id: 2,
      name: 'Backup account',
      provider: 'agentkey-mcp',
      base_url: 'https://api.agentkey.app/v1/mcp',
      key_prefix: 'ak_live_5678••••',
      plan: 'Pro',
      balance: 100,
      balance_micros: 100_000_000,
      weight: 1,
      priority: 0,
      pool: 'default',
      pool_id: 1,
      status: 'healthy',
      last_check: 1_787_891_200,
    })
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    expect(
      (await screen.findAllByText('Primary account')).length
    ).toBeGreaterThan(0)
    expect(
      screen.getAllByText((content) => content.includes('0.000001')).length
    ).toBeGreaterThan(0)
    await user.click(screen.getAllByRole('button', { name: 'Test' })[0])
    await waitFor(() =>
      expect(vi.mocked(testSearchUpstreamAccount).mock.calls.at(-1)?.[0]).toBe(
        1
      )
    )

    await user.click(screen.getByRole('button', { name: 'Connect account' }))
    await user.type(screen.getByLabelText('Account name'), 'Backup account')
    await user.type(screen.getByLabelText('AgentKey secret'), 'ak_live_secret')
    const connectButtons = screen.getAllByRole('button', {
      name: 'Connect account',
    })
    const dialogConnectButton = connectButtons.at(-1)
    expect(dialogConnectButton).toBeDefined()
    if (!dialogConnectButton) return
    await user.click(dialogConnectButton)

    await waitFor(() =>
      expect(
        vi.mocked(createSearchUpstreamAccount).mock.calls.at(-1)?.[0]
      ).toEqual(
        expect.objectContaining({
          name: 'Backup account',
          api_key: 'ak_live_secret',
          base_url: 'https://api.agentkey.app/v1/mcp',
          pool_id: 0,
          status: 'standby',
        })
      )
    )
  })

  test('shows field errors and does not submit an invalid upstream account', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    await screen.findAllByText('Primary account')
    await user.click(screen.getByRole('button', { name: 'Connect account' }))
    const connectButtons = screen.getAllByRole('button', {
      name: 'Connect account',
    })
    const submitButton = connectButtons.at(-1)
    expect(submitButton).toBeDefined()
    if (!submitButton) return
    await user.click(submitButton)

    expect(await screen.findByText('Account name is required')).toBeVisible()
    expect(screen.getByText('AgentKey secret is required')).toBeVisible()
    expect(screen.getByLabelText('Account name')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(createSearchUpstreamAccount).not.toHaveBeenCalled()
  })

  test('maps an upstream secret rejection to the secret field', async () => {
    vi.mocked(createSearchUpstreamAccount).mockRejectedValueOnce(
      new Error('search upstream encrypted secret is required')
    )
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    await screen.findAllByText('Primary account')
    await user.click(screen.getByRole('button', { name: 'Connect account' }))
    await user.type(screen.getByLabelText('Account name'), 'Backup account')
    await user.type(screen.getByLabelText('AgentKey secret'), 'ak_live_secret')
    const connectButtons = screen.getAllByRole('button', {
      name: 'Connect account',
    })
    const submitButton = connectButtons.at(-1)
    expect(submitButton).toBeDefined()
    if (!submitButton) return
    await user.click(submitButton)

    expect(
      await screen.findByText('Check the AgentKey secret and try again.')
    ).toBeVisible()
    expect(screen.getByLabelText('AgentKey secret')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
  })

  test('edits every upstream account setting and can rotate its secret', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    expect(
      (await screen.findAllByText('Primary account')).length
    ).toBeGreaterThan(0)
    await user.click(screen.getAllByRole('button', { name: 'Edit' })[0])

    const nameInput = screen.getByLabelText('Account name')
    const secretInput = screen.getByLabelText('AgentKey secret')
    const urlInput = screen.getByLabelText('AgentKey MCP URL')
    const poolInput = screen.getByLabelText('Pool ID')
    const weightInput = screen.getByLabelText('Weight')
    const priorityInput = screen.getByLabelText('Priority')
    expect(nameInput).toHaveValue('Primary account')
    expect(secretInput).toHaveValue('')

    await user.clear(nameInput)
    await user.type(nameInput, 'Renamed account')
    await user.type(secretInput, 'ak_live_replacement')
    await user.clear(urlInput)
    await user.type(urlInput, 'https://relay.example.com/v1/mcp')
    await user.clear(poolInput)
    await user.type(poolInput, '2')
    await user.clear(weightInput)
    await user.type(weightInput, '7')
    await user.clear(priorityInput)
    await user.type(priorityInput, '3')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(
        vi.mocked(updateSearchUpstreamAccount).mock.calls.at(-1)?.[0]
      ).toEqual({
        id: 1,
        name: 'Renamed account',
        api_key: 'ak_live_replacement',
        base_url: 'https://relay.example.com/v1/mcp',
        pool_id: 2,
        weight: 7,
        priority: 3,
        status: 'healthy',
      })
    )
  })

  test('pauses and deletes an upstream account with confirmation', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    expect(
      (await screen.findAllByText('Primary account')).length
    ).toBeGreaterThan(0)
    await user.click(screen.getAllByRole('button', { name: 'Pause' })[0])
    await waitFor(() =>
      expect(
        vi.mocked(updateSearchUpstreamAccount).mock.calls.at(-1)?.[0]
      ).toEqual({
        id: 1,
        name: 'Primary account',
        base_url: 'https://api.agentkey.app/v1/mcp',
        pool_id: 1,
        weight: 10,
        priority: 0,
        status: 'paused',
      })
    )

    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0])
    expect(
      await screen.findByText('Delete upstream account?')
    ).toBeInTheDocument()
    const confirmDelete = screen
      .getAllByRole('button', { name: 'Delete' })
      .at(-1)
    expect(confirmDelete).toBeDefined()
    if (!confirmDelete) return
    await user.click(confirmDelete)
    await waitFor(() =>
      expect(
        vi.mocked(deleteSearchUpstreamAccount).mock.calls.at(-1)?.[0]
      ).toBe(1)
    )
  })

  test('synchronizes and configures the server catalog without tabs', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminCatalogPage />)

    expect((await screen.findAllByText('Brave Search')).length).toBeGreaterThan(
      0
    )
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Sync catalog' }))
    await waitFor(() => expect(syncAdminSearchCatalog).toHaveBeenCalledOnce())

    await user.click(
      screen.getAllByRole('switch', { name: 'Enable {{name}}' })[0]
    )
    await waitFor(() =>
      expect(updateAdminSearchCatalogItem).toHaveBeenCalledWith('brave', {
        enabled: false,
      })
    )

    await user.click(screen.getByRole('button', { name: 'Extract' }))
    expect(screen.getAllByText('Firecrawl').length).toBeGreaterThan(0)
    expect(screen.queryByText('Brave Search')).not.toBeInTheDocument()
  })

  test('edits catalog prices through exact micros', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminCatalogPage />)

    await screen.findAllByText('Brave Search')
    const priceInput = screen.getAllByLabelText('Price for {{name}}')[0]
    expect(priceInput).toHaveValue(0.000123)
    await user.clear(priceInput)
    await user.type(priceInput, '0.000001')
    await user.click(screen.getAllByRole('button', { name: 'Save' })[0])

    await waitFor(() =>
      expect(updateAdminSearchCatalogItem).toHaveBeenCalledWith('brave', {
        price_micros: 1,
      })
    )
  })

  test('configures enterprise capability access from the catalog row', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminCatalogPage />)

    expect((await screen.findAllByText('Brave Search')).length).toBeGreaterThan(
      0
    )
    await user.click(
      screen.getAllByRole('button', { name: 'Enterprise access' })[0]
    )

    expect(
      await screen.findByText('Enterprise access for {{name}}')
    ).toBeInTheDocument()
    await user.click(screen.getByText('Orbit Labs'))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateSearchCapabilityEnterpriseGrants).toHaveBeenCalledWith(
        'brave',
        [11, 12]
      )
    )
  })

  test('loads platform-wide vSearch accounting from dedicated endpoints', async () => {
    renderWithQuery(<SearchAdminUsageLogsPage />)

    expect((await screen.findAllByText('Brave Search')).length).toBeGreaterThan(
      0
    )
    expect(screen.getAllByText('Northstar Research').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Primary account').length).toBeGreaterThan(0)
    expect(screen.getAllByText('/v1/search').length).toBeGreaterThan(1)
    expect(screen.getAllByText('request-precise-1').length).toBeGreaterThan(1)
    expect(screen.getAllByText('UPSTREAM_TIMEOUT').length).toBeGreaterThan(1)
    expect(screen.getAllByText('Indeterminate').length).toBeGreaterThan(1)
    expect(screen.getAllByText('—').length).toBeGreaterThan(0)
    expect(fetchAdminSearchUsageLogs).toHaveBeenCalledWith({
      page: 1,
      page_size: 20,
      range: 30,
      query: undefined,
      status: undefined,
    })
    expect(fetchAdminSearchUsageStats).toHaveBeenCalledWith({
      range: 30,
      query: undefined,
      status: undefined,
    })
    expect(screen.getAllByText('Upstream cost').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Gross profit').length).toBeGreaterThan(0)
    expect(
      screen.getAllByText((content) => content.includes('0.000003')).length
    ).toBeGreaterThan(0)
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })

  test('renders actionable error states for each management endpoint', async () => {
    vi.mocked(fetchSearchUpstreamAccounts).mockRejectedValueOnce(
      new Error('forbidden')
    )
    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    expect(
      await screen.findByText('Failed to load upstream accounts')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })
})
