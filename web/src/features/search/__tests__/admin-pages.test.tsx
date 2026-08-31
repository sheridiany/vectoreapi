/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError } from 'axios'
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
  reconcileAdminSearchUsage,
  syncAdminSearchCatalog,
  testSearchUpstreamAccount,
  updateSearchUpstreamAccount,
  updateAdminSearchCatalogItem,
  updateSearchCapabilityEnterpriseGrants,
} from '../api'

const toastError = vi.hoisted(() => vi.fn())
const handleServerError = vi.hoisted(() => vi.fn())
const translate = vi.hoisted(() => vi.fn((key: string) => key))

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
  reconcileAdminSearchUsage: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: translate }),
}))

vi.mock('sonner', () => ({
  toast: {
    error: toastError,
    success: vi.fn(),
    warning: vi.fn(),
  },
}))

vi.mock('@/lib/handle-server-error', () => ({
  handleServerError,
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
        provider: 'justoneapi_rest',
        base_url: 'https://api.justoneapi.com',
        key_prefix: 'ak_live_1234••••',
        plan: 'Pro',
        balance: 500,
        balance_micros: 1,
        balance_currency: 'CNY',
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
      provider: 'justoneapi_rest',
      base_url: 'https://api.justoneapi.com',
      key_prefix: 'ak_live_1234••••',
      plan: 'Pro',
      balance: 500,
      balance_micros: 500_000_000,
      balance_currency: 'CNY',
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
        provider: input.provider,
        base_url: input.base_url,
        key_prefix: 'ak_live_1234••••',
        plan: 'Pro',
        balance: 500,
        balance_micros: 500_000_000,
        balance_currency: 'CNY',
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
        schema_status: 'available',
        contract_status: 'verified',
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
        schema_status: 'available',
        contract_status: 'verified',
        enabled: true,
        interface_count: 2,
        upstream_cost: 0.3,
        upstream_cost_micros: 2,
        price: 0.5,
        price_micros: 456,
      },
    ])
    vi.mocked(syncAdminSearchCatalog).mockResolvedValue({
      synced: 2,
      published: 0,
      skipped: 0,
      failures: [],
      synced_service_ids: ['vr_svc_brave', 'vr_svc_firecrawl'],
    })
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
        contract_status: 'verified',
        enabled: patch.enabled ?? true,
        interface_count: 1,
        price_micros: patch.price_micros,
      })
    )
    vi.mocked(fetchAdminSearchUsageLogs).mockResolvedValue({
      items: [
        {
          id: 1,
          created_at: '2026-08-28T10:30:00Z',
          service: 'Brave Search',
          endpoint: '/v1/search',
          status: 'indeterminate',
          latency_ms: 420,
          key_name: 'research-bot',
          enterprise_name: 'Northstar Research',
          user_name: 'alice',
          account: 'Primary account',
          request_id: 'request-precise-1',
          error_code: 'UPSTREAM_TIMEOUT',
          execution_phase: 'dispatching',
          billing_state: 'reserved',
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
    vi.mocked(reconcileAdminSearchUsage).mockImplementation(
      async (id, input) => ({
        id,
        request_id: 'request-precise-1',
        status: 'indeterminate',
        billing_state: input.action === 'settle' ? 'committed' : 'refunded',
        reconciliation_action: input.action,
        reconciliation_note: input.note,
        reconciled_by: 9,
        reconciled_at: 1_788_000_000,
        started: true,
      })
    )
  })

  test('connects and health-checks direct provider accounts', async () => {
    vi.mocked(createSearchUpstreamAccount).mockResolvedValue({
      id: 2,
      name: 'Backup account',
      provider: 'tikhub_rest',
      base_url: 'https://api.tikhub.io',
      key_prefix: 'ak_live_5678••••',
      plan: 'Pro',
      balance: 100,
      balance_micros: 100_000_000,
      balance_currency: 'USD',
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
    expect(
      screen.getAllByText((content) => content.includes('CNY')).length
    ).toBeGreaterThan(0)
    await user.click(screen.getAllByRole('button', { name: 'Test' })[0])
    await waitFor(() =>
      expect(vi.mocked(testSearchUpstreamAccount).mock.calls.at(-1)?.[0]).toBe(
        1
      )
    )

    await user.click(screen.getByRole('button', { name: 'Connect account' }))
    await user.type(screen.getByLabelText('Account name'), 'Backup account')
    await user.click(screen.getByLabelText('Provider'))
    await user.click(screen.getByRole('option', { name: 'TikHub REST' }))
    expect(screen.getByLabelText('Provider API base URL')).toHaveValue(
      'https://api.tikhub.io'
    )
    await user.type(screen.getByLabelText('Provider API key'), 'tk_live_secret')
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
          provider: 'tikhub_rest',
          name: 'Backup account',
          api_key: 'tk_live_secret',
          base_url: 'https://api.tikhub.io',
          pool_id: 0,
          status: 'healthy',
        })
      )
    )
  })

  test('does not invent a currency for an unknown provider balance', async () => {
    vi.mocked(fetchSearchUpstreamAccounts).mockResolvedValue([
      {
        id: 2,
        name: 'Unverified TikHub account',
        provider: 'tikhub_rest',
        base_url: 'https://api.tikhub.io',
        key_prefix: 'tk_live_5678••••',
        plan: '',
        balance: 0,
        balance_micros: 0,
        balance_currency: '',
        weight: 1,
        priority: 0,
        pool: 'default',
        pool_id: 1,
        status: 'healthy',
        last_check: 0,
      },
    ])

    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    expect(
      (await screen.findAllByText('Unverified TikHub account')).length
    ).toBeGreaterThan(0)
    expect(screen.queryByText(/CNY/)).not.toBeInTheDocument()
    expect(screen.getAllByText('—').length).toBeGreaterThan(0)
  })

  test('enables a paused upstream account as an active route', async () => {
    vi.mocked(fetchSearchUpstreamAccounts).mockResolvedValue([
      {
        id: 3,
        name: 'Paused JustOneAPI account',
        provider: 'justoneapi_rest',
        base_url: 'https://api.justoneapi.com',
        key_prefix: 'ak_live_9012••••',
        plan: '',
        balance: 0,
        balance_micros: 0,
        balance_currency: '',
        weight: 1,
        priority: 0,
        pool: 'default',
        pool_id: 1,
        status: 'paused',
        last_check: 0,
      },
    ])
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUpstreamAccountsPage />)

    await screen.findAllByText('Paused JustOneAPI account')
    await user.click(screen.getAllByRole('button', { name: 'Enable' })[0])

    await waitFor(() =>
      expect(
        vi.mocked(updateSearchUpstreamAccount).mock.calls.at(-1)?.[0]
      ).toEqual(
        expect.objectContaining({
          id: 3,
          status: 'healthy',
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
    expect(screen.getByText('Provider API key is required')).toBeVisible()
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
    await user.type(
      screen.getByLabelText('Provider API key'),
      'joa_live_secret'
    )
    const connectButtons = screen.getAllByRole('button', {
      name: 'Connect account',
    })
    const submitButton = connectButtons.at(-1)
    expect(submitButton).toBeDefined()
    if (!submitButton) return
    await user.click(submitButton)

    expect(
      await screen.findByText('Check the provider API key and try again.')
    ).toBeVisible()
    expect(screen.getByLabelText('Provider API key')).toHaveAttribute(
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
    const secretInput = screen.getByLabelText('Provider API key')
    const urlInput = screen.getByLabelText('Provider API base URL')
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
        provider: 'justoneapi_rest',
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
        provider: 'justoneapi_rest',
        name: 'Primary account',
        base_url: 'https://api.justoneapi.com',
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

  test('synchronizes the standard catalog into draft without publishing', async () => {
    vi.mocked(syncAdminSearchCatalog).mockResolvedValueOnce({
      synced: 20,
      published: 0,
      skipped: 0,
      failures: [],
      synced_service_ids: ['vr_svc_social', 'vr_svc_commerce'],
    })
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminCatalogPage />)

    await screen.findAllByText('Brave Search')
    expect(
      screen.queryByRole('button', {
        name: 'Publish synchronized capabilities',
      })
    ).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Sync catalog' }))
    await waitFor(() => expect(syncAdminSearchCatalog).toHaveBeenCalledOnce())
    expect(translate).toHaveBeenCalledWith(
      '{{count}} capabilities synchronized',
      { count: 2 }
    )
    expect(
      screen.queryByRole('button', {
        name: 'Publish synchronized capabilities',
      })
    ).not.toBeInTheDocument()
  })

  test('shows one actionable error when catalog synchronization fails', async () => {
    vi.mocked(syncAdminSearchCatalog).mockRejectedValueOnce(
      new Error('Connect and health-check a provider account first.')
    )
    renderWithQuery(<SearchAdminCatalogPage />)

    await screen.findAllByText('Brave Search')
    await userEvent.click(screen.getByRole('button', { name: 'Sync catalog' }))

    await waitFor(() => expect(toastError).toHaveBeenCalledOnce())
    expect(toastError).toHaveBeenCalledWith(
      'Connect and health-check a provider account first.'
    )
  })

  test('routes an HTTP 429 through the localized server error handler once', async () => {
    const rateLimitError = Object.assign(
      new AxiosError('Request failed with status code 429'),
      { response: { status: 429, data: '' } }
    )
    vi.mocked(syncAdminSearchCatalog).mockRejectedValueOnce(rateLimitError)
    renderWithQuery(<SearchAdminCatalogPage />)

    await screen.findAllByText('Brave Search')
    await userEvent.click(screen.getByRole('button', { name: 'Sync catalog' }))

    await waitFor(() => expect(handleServerError).toHaveBeenCalledOnce())
    expect(handleServerError).toHaveBeenCalledWith(rateLimitError)
    expect(toastError).not.toHaveBeenCalled()
  })

  test('labels schema-unavailable capabilities and prevents enabling them', async () => {
    vi.mocked(fetchAdminSearchCatalog).mockResolvedValueOnce([
      {
        id: 'unavailable',
        name: 'Unavailable Search',
        category: 'Search',
        description: 'Missing parameter schema.',
        status: 'unavailable',
        schema_status: 'unavailable',
        contract_status: 'unverified',
        enabled: false,
        interface_count: 0,
        healthy_route_count: 1,
      },
    ])
    renderWithQuery(<SearchAdminCatalogPage />)

    expect(
      (await screen.findAllByText('Parameter schema unavailable')).length
    ).toBeGreaterThan(0)
    const availabilitySwitches = screen.getAllByRole('switch', {
      name: 'Enable {{name}}',
    })
    expect(availabilitySwitches.length).toBeGreaterThan(0)
    for (const availabilitySwitch of availabilitySwitches) {
      expect(availabilitySwitch).toHaveAttribute('aria-disabled', 'true')
    }
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

  test('filters admin usage logs to unresolved indeterminate requests', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUsageLogsPage />)

    await screen.findAllByText('Brave Search')
    await user.click(
      screen.getByRole('button', { name: 'Indeterminate', pressed: false })
    )

    await waitFor(() =>
      expect(fetchAdminSearchUsageLogs).toHaveBeenLastCalledWith({
        page: 1,
        page_size: 20,
        range: 30,
        query: undefined,
        status: 'indeterminate',
      })
    )
  })

  test('requires an audit note before settling an unresolved reserved request', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUsageLogsPage />)

    const settleButtons = await screen.findAllByRole('button', {
      name: 'Settle charge',
    })
    expect(settleButtons.length).toBe(2)
    expect(
      screen.getAllByRole('button', { name: 'Refund reservation' })
    ).toHaveLength(2)

    await user.click(settleButtons[0])

    expect(
      screen.getByRole('alertdialog', { name: 'Settle this vSearch request?' })
    ).toBeInTheDocument()
    const confirmButton = screen.getByRole('button', {
      name: 'Confirm settlement',
    })
    expect(confirmButton).toBeEnabled()

    await user.click(confirmButton)
    const noteInput = screen.getByRole('textbox', { name: 'Operator note' })
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'An audit note is required for this financial action.'
    )
    expect(noteInput).toHaveAttribute('aria-invalid', 'true')
    expect(noteInput).toHaveAccessibleDescription(
      'An audit note is required for this financial action.'
    )
    expect(reconcileAdminSearchUsage).not.toHaveBeenCalled()

    await user.type(noteInput, 'Confirmed in the upstream provider audit log')
    await user.click(confirmButton)

    await waitFor(() =>
      expect(reconcileAdminSearchUsage).toHaveBeenCalledWith(1, {
        action: 'settle',
        note: 'Confirmed in the upstream provider audit log',
      })
    )
    await waitFor(() => {
      expect(fetchAdminSearchUsageLogs).toHaveBeenCalledTimes(2)
      expect(fetchAdminSearchUsageStats).toHaveBeenCalledTimes(2)
    })
  })

  test('keeps the reconciliation dialog and note when the request fails and refreshes the latest state', async () => {
    vi.mocked(reconcileAdminSearchUsage).mockRejectedValueOnce(
      new Error('settlement unavailable')
    )
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUsageLogsPage />)

    const settleButtons = await screen.findAllByRole('button', {
      name: 'Settle charge',
    })
    await user.click(settleButtons[0])
    const noteInput = screen.getByRole('textbox', { name: 'Operator note' })
    await user.type(noteInput, 'Confirmed by provider request ID')
    await user.click(screen.getByRole('button', { name: 'Confirm settlement' }))

    await waitFor(() =>
      expect(reconcileAdminSearchUsage).toHaveBeenCalledWith(1, {
        action: 'settle',
        note: 'Confirmed by provider request ID',
      })
    )
    expect(
      screen.getByRole('alertdialog', { name: 'Settle this vSearch request?' })
    ).toBeInTheDocument()
    expect(noteInput).toHaveValue('Confirmed by provider request ID')
    await waitFor(() => {
      expect(fetchAdminSearchUsageLogs).toHaveBeenCalledTimes(2)
      expect(fetchAdminSearchUsageStats).toHaveBeenCalledTimes(2)
    })
  })

  test('can refund an unresolved reserved request from the mobile action set', async () => {
    const user = userEvent.setup()
    renderWithQuery(<SearchAdminUsageLogsPage />)

    const desktopLogs = await screen.findByTestId('desktop-usage-logs')
    const mobileLogs = screen.getByTestId('mobile-usage-logs')
    expect(desktopLogs).toHaveClass('hidden', 'md:block')
    expect(mobileLogs).toHaveClass('md:hidden')
    expect(
      within(desktopLogs).getByRole('button', { name: 'Refund reservation' })
    ).toBeInTheDocument()
    const refundButton = within(mobileLogs).getByRole('button', {
      name: 'Refund reservation',
    })
    await user.click(refundButton)
    await user.type(
      screen.getByRole('textbox', { name: 'Operator note' }),
      'No completion record was found upstream'
    )
    await user.click(screen.getByRole('button', { name: 'Confirm refund' }))

    await waitFor(() =>
      expect(reconcileAdminSearchUsage).toHaveBeenCalledWith(1, {
        action: 'refund',
        note: 'No completion record was found upstream',
      })
    )
  })

  test('only retries the original financial action while reconciliation is incomplete', async () => {
    const user = userEvent.setup()
    vi.mocked(fetchAdminSearchUsageLogs).mockResolvedValueOnce({
      items: [
        {
          id: 2,
          created_at: '2026-08-28T10:30:00Z',
          service: 'Brave Search',
          endpoint: '/v1/search',
          status: 'indeterminate',
          execution_phase: 'dispatching',
          billing_state: 'log_pending',
          reconciliation_action: 'settle',
          reconciliation_note: 'Confirmed upstream completion',
          latency_ms: 420,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })

    renderWithQuery(<SearchAdminUsageLogsPage />)

    const retryButtons = await screen.findAllByRole('button', {
      name: 'Retry settlement',
    })
    expect(retryButtons).toHaveLength(2)
    expect(
      screen.queryByRole('button', { name: 'Refund reservation' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Retry refund' })
    ).not.toBeInTheDocument()

    await user.click(retryButtons[0])
    expect(screen.getByRole('textbox', { name: 'Operator note' })).toHaveValue(
      'Confirmed upstream completion'
    )
  })

  test('shows terminal manual outcomes and exposes settled financial amounts', async () => {
    vi.mocked(fetchAdminSearchUsageLogs).mockResolvedValueOnce({
      items: [
        {
          id: 3,
          created_at: '2026-08-28T10:30:00Z',
          service: 'Brave Search',
          endpoint: '/v1/search',
          status: 'indeterminate',
          execution_phase: 'dispatching',
          billing_state: 'committed',
          reconciliation_action: 'settle',
          reconciliation_note: 'Confirmed upstream completion',
          charge_micros: 10,
          latency_ms: 420,
        },
        {
          id: 4,
          created_at: '2026-08-28T10:31:00Z',
          service: 'Firecrawl',
          endpoint: '/v1/scrape',
          status: 'indeterminate',
          execution_phase: 'dispatching',
          billing_state: 'refunded',
          reconciliation_action: 'refund',
          reconciliation_note: 'No upstream completion evidence',
          charge_micros: 20,
          latency_ms: 500,
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
    })

    renderWithQuery(<SearchAdminUsageLogsPage />)

    expect(await screen.findAllByText('Settled by admin')).toHaveLength(2)
    expect(screen.getAllByText('Refunded by admin')).toHaveLength(2)
    expect(
      screen.getAllByText((content) => content.includes('0.00001')).length
    ).toBeGreaterThan(0)
    expect(
      screen.queryByRole('button', { name: 'Settle charge' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Refund reservation' })
    ).not.toBeInTheDocument()
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
