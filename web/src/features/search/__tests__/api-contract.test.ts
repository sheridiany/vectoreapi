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
  createAdminSearchAgentKey,
  createAdminSearchInstallToken,
  createSearchUpstreamAccount,
  deleteSearchUpstreamAccount,
  fetchAdminSearchAgentKeys,
  fetchSearchCapabilityEnterpriseGrants,
  fetchSearchAgentKeyOwnerCandidates,
  fetchSearchGrantEnterprises,
  fetchSearchUsageLogs,
  revokeAdminSearchAgentKey,
  publishAdminSearchCatalog,
  syncAdminSearchCatalog,
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
      name: 'Primary',
      api_key: 'ak_live_secret',
      base_url: 'https://api.agentkey.app/v1/mcp',
      pool_id: 3,
      weight: 5,
      priority: 1,
      status: 'standby',
    })

    expect(api.post).toHaveBeenCalledWith(
      '/api/search/admin/upstream-accounts',
      {
        name: 'Primary',
        base_url: 'https://api.agentkey.app/v1/mcp',
        secret: 'ak_live_secret',
        pool_id: 3,
        weight: 5,
        priority: 1,
        status: 'standby',
      }
    )
  })

  test('uses the managed AgentKey API instead of upstream-account routes', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, data: [] },
    })
    vi.mocked(api.post)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: { id: 7, user_id: 3, secret: 'vr_live_secret' },
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: { token: 'vr_search_install_test', expires_at: 2 },
        },
      })
    vi.mocked(api.delete).mockResolvedValue({ data: { success: true } })

    await fetchAdminSearchAgentKeys()
    await createAdminSearchAgentKey({
      user_id: 3,
      name: 'ops-bot',
      scopes: ['web-search'],
    })
    await createAdminSearchInstallToken(7)
    await revokeAdminSearchAgentKey(7)

    expect(api.get).toHaveBeenCalledWith('/api/search/admin/agent-keys')
    expect(api.post).toHaveBeenNthCalledWith(
      1,
      '/api/search/admin/agent-keys',
      {
        user_id: 3,
        name: 'ops-bot',
        scopes: ['web-search'],
      }
    )
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/search/admin/agent-keys/7/install-token'
    )
    expect(api.delete).toHaveBeenCalledWith('/api/search/admin/agent-keys/7')
  })

  test('scopes AgentKey owner candidates to enterprise members', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          items: [
            {
              user_id: 3,
              status: 1,
              user: {
                id: 3,
                username: 'alice',
                display_name: 'Alice',
              },
            },
            {
              user_id: 4,
              status: 2,
              user: { id: 4, username: 'disabled' },
            },
          ],
          total: 2,
          page: 1,
          page_size: 100,
        },
      },
    })

    const candidates = await fetchSearchAgentKeyOwnerCandidates(11)

    expect(api.get).toHaveBeenCalledWith('/api/enterprise/11/members', {
      params: { p: 1, page_size: 100 },
    })
    expect(candidates).toEqual([
      { id: 3, username: 'alice', display_name: 'Alice' },
    ])
  })

  test('loads platform AgentKey owner candidates from enabled users for root', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          items: [
            {
              id: 1,
              status: 1,
              username: 'root',
              display_name: 'Root',
            },
            { id: 2, status: 2, username: 'disabled' },
          ],
          total: 2,
          page: 1,
          page_size: 100,
        },
      },
    })

    const candidates = await fetchSearchAgentKeyOwnerCandidates()

    expect(api.get).toHaveBeenCalledWith('/api/user/', {
      params: {
        p: 1,
        page_size: 100,
        sort_by: 'username',
        sort_order: 'asc',
      },
    })
    expect(candidates).toEqual([
      { id: 1, username: 'root', display_name: 'Root' },
    ])
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

  test('updates catalog prices through the exact micros field', async () => {
    vi.mocked(api.patch).mockResolvedValue({
      data: { success: true, data: { id: 'brave', price_micros: 1 } },
    })

    await updateAdminSearchCatalogItem('brave/search', { price_micros: 1 })

    expect(api.patch).toHaveBeenCalledWith(
      '/api/search/admin/catalog/brave%2Fsearch',
      { price_micros: 1 }
    )
  })

  test('publishes only the service ids returned by the latest catalog sync', async () => {
    vi.mocked(api.post)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            synced: 2,
            synced_service_ids: ['vr_svc_brave', 'vr_svc_firecrawl'],
          },
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            published: 1,
            skipped: 1,
            published_service_ids: ['vr_svc_brave'],
            skipped_services: [
              {
                service_id: 'vr_svc_firecrawl',
                reason: 'schema_unavailable',
              },
            ],
          },
        },
      })

    const syncResult = await syncAdminSearchCatalog()
    const publishResult = await publishAdminSearchCatalog({
      service_ids: syncResult.synced_service_ids,
      access_mode: 'all_enterprises',
    })

    expect(api.post).toHaveBeenNthCalledWith(
      1,
      '/api/search/admin/catalog/sync'
    )
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/search/admin/catalog/publish',
      {
        service_ids: ['vr_svc_brave', 'vr_svc_firecrawl'],
        access_mode: 'all_enterprises',
      }
    )
    expect(publishResult).toEqual(
      expect.objectContaining({ published: 1, skipped: 1 })
    )
  })

  test('updates the complete account record and deletes by numeric id', async () => {
    vi.mocked(api.patch).mockResolvedValue({
      data: { success: true, data: { id: 7 } },
    })
    vi.mocked(api.delete).mockResolvedValue({ data: { success: true } })

    await updateSearchUpstreamAccount({
      id: 7,
      name: 'Primary',
      base_url: 'https://api.agentkey.app/v1/mcp',
      pool_id: 3,
      weight: 5,
      priority: 1,
      status: 'paused',
    })
    await deleteSearchUpstreamAccount(7)

    expect(api.patch).toHaveBeenCalledWith(
      '/api/search/admin/upstream-accounts/7',
      {
        name: 'Primary',
        base_url: 'https://api.agentkey.app/v1/mcp',
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
      name: 'Primary',
      api_key: '  ak_live_replacement  ',
      base_url: 'https://api.agentkey.app/v1/mcp',
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
