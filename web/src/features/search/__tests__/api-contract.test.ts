/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  createSearchAgentKey,
  createSearchInstallToken,
  createSearchUpstreamAccount,
  deleteSearchUpstreamAccount,
  fetchSearchAgentKeys,
  fetchSearchCapabilityEnterpriseGrants,
  fetchSearchGrantEnterprises,
  fetchSearchUsageLogs,
  reconcileAdminSearchUsage,
  revokeSearchAgentKey,
  updateAdminSearchCatalogItem,
  updateSearchCapabilityEnterpriseGrants,
  updateSearchUpstreamAccount,
} from '../api'

vi.mock('@/lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('vSearch frontend API contract', () => {
  beforeEach(() => vi.clearAllMocks())

  test('maps account form fields to the backend secret contract', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, data: { id: 7 } },
    })

    await createSearchUpstreamAccount({
      provider: 'justoneapi_rest',
      name: 'Primary',
      api_key: 'ak_live_secret',
      base_url: 'https://api.justoneapi.com',
      pool_id: 3,
      weight: 5,
      priority: 1,
      status: 'standby',
    })

    expect(api.post).toHaveBeenCalledWith(
      '/api/search/admin/upstream-accounts',
      {
        provider: 'justoneapi_rest',
        name: 'Primary',
        base_url: 'https://api.justoneapi.com',
        secret: 'ak_live_secret',
        pool_id: 3,
        weight: 5,
        priority: 1,
        status: 'standby',
      }
    )
  })

  test('uses the dedicated user vSearch key routes', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, data: [] },
    })
    vi.mocked(api.post)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: { id: 9, user_id: 3, secret: 'vr_live_secret' },
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: { token: 'vr_search_install_test', expires_at: 2 },
        },
      })
    vi.mocked(api.delete).mockResolvedValue({ data: { success: true } })

    await fetchSearchAgentKeys()
    await createSearchAgentKey('research-bot', ['web-search'])
    await createSearchInstallToken(9)
    await revokeSearchAgentKey(9)

    expect(api.get).toHaveBeenCalledWith('/api/search/keys')
    expect(api.post).toHaveBeenNthCalledWith(1, '/api/search/keys', {
      name: 'research-bot',
      scopes: ['web-search'],
    })
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/search/keys/9/install-token'
    )
    expect(api.delete).toHaveBeenCalledWith('/api/search/keys/9')
  })

  test('maps dashboard pagination and range to p and days query fields', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: { items: [], total: 0, page: 2, page_size: 20 },
      },
    })

    await fetchSearchUsageLogs({
      page: 2,
      page_size: 20,
      range: 30,
      query: 'brave',
      status: 'success',
    })

    expect(api.get).toHaveBeenCalledWith('/api/search/logs', {
      params: {
        p: 2,
        page_size: 20,
        days: 30,
        query: 'brave',
        status: 'success',
      },
    })
  })

  test('sends an audited manual reconciliation to the exact admin request route', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: {
        success: true,
        data: {
          id: 17,
          request_id: 'request-17',
          status: 'indeterminate',
          billing_state: 'refunded',
          reconciliation_action: 'refund',
          reconciliation_note: 'No upstream completion evidence',
          reconciled_by: 9,
          reconciled_at: 1_788_000_000,
          started: true,
        },
      },
    })

    const result = await reconcileAdminSearchUsage(17, {
      action: 'refund',
      note: 'No upstream completion evidence',
    })

    expect(api.post).toHaveBeenCalledWith(
      '/api/search/admin/usage-logs/17/reconcile',
      {
        action: 'refund',
        note: 'No upstream completion evidence',
      }
    )
    expect(result).toEqual(
      expect.objectContaining({
        id: 17,
        reconciliation_action: 'refund',
        started: true,
      })
    )
  })

  test('updates catalog prices through the exact micros field', async () => {
    vi.mocked(api.patch).mockResolvedValue({
      data: { success: true, data: { id: 'brave', price_micros: 1 } },
    })

    await updateAdminSearchCatalogItem('brave/search', { price_micros: 1 })

    expect(api.patch).toHaveBeenCalledWith(
      '/api/search/admin/catalog/brave%2Fsearch',
      { price_micros: 1 },
      { skipErrorHandler: true, skipBusinessError: true }
    )
  })

  test('updates the complete account record and deletes by numeric id', async () => {
    vi.mocked(api.patch).mockResolvedValue({
      data: { success: true, data: { id: 7 } },
    })
    vi.mocked(api.delete).mockResolvedValue({ data: { success: true } })

    await updateSearchUpstreamAccount({
      id: 7,
      provider: 'justoneapi_rest',
      name: 'Primary',
      base_url: 'https://api.justoneapi.com',
      pool_id: 3,
      weight: 5,
      priority: 1,
      status: 'paused',
    })
    await deleteSearchUpstreamAccount(7)

    expect(api.patch).toHaveBeenCalledWith(
      '/api/search/admin/upstream-accounts/7',
      {
        provider: 'justoneapi_rest',
        name: 'Primary',
        base_url: 'https://api.justoneapi.com',
        secret: '',
        pool_id: 3,
        weight: 5,
        priority: 1,
        status: 'paused',
      }
    )
    expect(api.delete).toHaveBeenCalledWith(
      '/api/search/admin/upstream-accounts/7'
    )
  })

  test('sends a replacement secret only when the edit form provides one', async () => {
    vi.mocked(api.patch).mockResolvedValue({
      data: { success: true, data: { id: 7 } },
    })

    await updateSearchUpstreamAccount({
      id: 7,
      provider: 'tikhub_rest',
      name: 'Primary',
      api_key: '  ak_live_replacement  ',
      base_url: 'https://api.tikhub.io',
      pool_id: 3,
      weight: 5,
      priority: 1,
      status: 'healthy',
    })

    expect(api.patch).toHaveBeenCalledWith(
      '/api/search/admin/upstream-accounts/7',
      expect.objectContaining({ secret: 'ak_live_replacement' })
    )
  })

  test('reads and replaces enterprise capability grants with an empty-list global mode', async () => {
    vi.mocked(api.get)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            capability_id: 'vr_svc_1',
            access_mode: 'selected_enterprises',
            enterprise_ids: [11],
          },
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            items: [
              { id: 11, name: 'Northstar', code: 'northstar', status: 1 },
            ],
            total: 1,
            page: 1,
            page_size: 100,
          },
        },
      })
    vi.mocked(api.put).mockResolvedValue({
      data: {
        success: true,
        data: {
          capability_id: 'vr_svc_1',
          access_mode: 'all_enterprises',
          enterprise_ids: [],
        },
      },
    })

    await fetchSearchCapabilityEnterpriseGrants('vr_svc_1')
    const enterprises = await fetchSearchGrantEnterprises()
    await updateSearchCapabilityEnterpriseGrants('vr_svc_1', [])

    expect(api.get).toHaveBeenNthCalledWith(
      1,
      '/api/search/admin/catalog/vr_svc_1/grants'
    )
    expect(api.get).toHaveBeenNthCalledWith(2, '/api/enterprise/admin/', {
      params: { p: 1, page_size: 100 },
    })
    expect(enterprises).toHaveLength(1)
    expect(api.put).toHaveBeenCalledWith(
      '/api/search/admin/catalog/vr_svc_1/grants',
      { enterprise_ids: [] }
    )
  })
})
