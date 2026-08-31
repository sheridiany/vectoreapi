/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

export type SearchAgentKeyApiRecord = {
  id: number
  user_id: number
  enterprise_id: number
  label: string
  prefix: string
  owner?: string
  status: 'active' | 'disabled' | 'revoked'
  scopes: string[]
  created_at: number
  last_used_at?: number
  expires_at?: number
}

export type CreatedSearchAgentKey = SearchAgentKeyApiRecord & {
  secret: string
}

type ApiResponse<T> = { success: boolean; message?: string; data?: T }

export type SearchCatalogStatus =
  | 'available'
  | 'unavailable'
  | 'catalog'
  | 'disabled'

export type SearchCatalogItem = {
  id: string
  name: string
  category: string
  description: string
  status: SearchCatalogStatus
  schema_status?: 'available' | 'unavailable'
  contract_status: 'verified' | 'unverified'
  enabled: boolean
  interface_count: number
  available_interface_count?: number
  supported_platforms?: string[]
  request_parameters?: string[]
  information_fields?: string[]
  healthy_route_count?: number
  cost_label?: string
  recent_latency_ms?: number | null
  last_synced_at?: number | null
  price?: number | null
  price_micros?: number
  price_min_micros?: number
  price_max_micros?: number
}

export type SearchUsageLog = {
  id: number | string
  created_at: number | string
  service: string
  endpoint: string
  status: 'success' | 'error' | string
  latency_ms: number
  key_name?: string
  request_id?: string
  charge?: number
  charge_micros?: number
  enterprise_name?: string
  user_name?: string
  account?: string
  upstream_cost?: number
  upstream_cost_micros?: number
  profit?: number
  profit_micros?: number
  error_code?: string
  execution_phase?: string
  billing_state?: string
  reconciliation_action?: SearchUsageReconciliationAction
  reconciliation_note?: string
  reconciled_by?: number
  reconciled_at?: number | string
}

export type SearchUsageReconciliationAction = 'settle' | 'refund'

export type SearchUsageReconciliationInput = {
  action: SearchUsageReconciliationAction
  note: string
}

export type SearchUsageReconciliationResult = {
  id: number
  request_id: string
  status: string
  billing_state: string
  reconciliation_action: SearchUsageReconciliationAction
  reconciliation_note: string
  reconciled_by: number
  reconciled_at: number
  started: boolean
}

export type SearchUsageStats = {
  total_requests: number
  success_requests: number
  error_requests: number
  pending_requests?: number
  indeterminate_requests?: number
  success_rate: number
  average_latency_ms: number
  quota?: number
  quota_micros?: number
  upstream_cost?: number
  upstream_cost_micros?: number
  revenue?: number
  revenue_micros?: number
  profit?: number
  profit_micros?: number
}

export type SearchPage<T> = {
  items: T[]
  total: number
  page: number
  page_size: number
}

export type SearchLogParams = {
  page: number
  page_size: number
  range: number
  query?: string
  status?: string
}

export type SearchUpstreamProvider = 'justoneapi_rest' | 'tikhub_rest'

export type SearchUpstreamAccount = {
  id: number
  name: string
  provider: SearchUpstreamProvider
  base_url: string
  key_prefix: string
  plan: string
  balance: number
  balance_micros: number
  balance_currency: string
  weight: number
  priority: number
  pool: string
  pool_id: number
  status: 'healthy' | 'warning' | 'standby' | 'paused'
  last_check: number
  last_error?: string | null
}

export type CreateSearchUpstreamAccount = {
  provider: SearchUpstreamProvider
  name: string
  api_key: string
  base_url: string
  pool_id: number
  weight: number
  priority: number
  status: 'healthy' | 'warning' | 'standby' | 'paused'
}

export type UpdateSearchUpstreamAccount = Omit<
  CreateSearchUpstreamAccount,
  'api_key'
> & { id: number; api_key?: string }

export type SearchAdminCatalogItem = SearchCatalogItem & {
  upstream_cost?: number | null
  upstream_cost_micros?: number | null
  price?: number | null
}

export type SearchCatalogSyncResult = {
  synced: number
  published: number
  skipped: number
  failures: string[]
  synced_service_ids: string[]
}

export type SearchCapabilityEnterpriseGrants = {
  capability_id: string
  access_mode: 'all_enterprises' | 'selected_enterprises'
  enterprise_ids: number[]
}

export type SearchGrantEnterprise = {
  id: number
  name: string
  code: string
  status: number
}

function unwrapResponse<T>(response: ApiResponse<T>, fallback: string): T {
  if (!response.success || response.data === undefined) {
    throw new Error(response.message || fallback)
  }
  return response.data
}

export async function fetchSearchAgentKeys(): Promise<
  SearchAgentKeyApiRecord[]
> {
  const response =
    await api.get<ApiResponse<SearchAgentKeyApiRecord[]>>('/api/search/keys')
  if (!response.data.success) {
    throw new Error(response.data.message || 'Request failed')
  }
  return response.data.data || []
}

export async function createSearchAgentKey(
  name: string,
  scopes: string[]
): Promise<CreatedSearchAgentKey> {
  const response = await api.post<ApiResponse<CreatedSearchAgentKey>>(
    '/api/search/keys',
    { name, scopes }
  )
  if (!response.data.success || !response.data.data) {
    throw new Error(response.data.message || 'Request failed')
  }
  return response.data.data
}

export async function revokeSearchAgentKey(id: number): Promise<void> {
  const response = await api.delete<ApiResponse<null>>(`/api/search/keys/${id}`)
  if (!response.data.success) {
    throw new Error(response.data.message || 'Request failed')
  }
}

export async function fetchSearchCatalog(): Promise<SearchCatalogItem[]> {
  const response = await api.get<ApiResponse<SearchCatalogItem[]>>(
    '/api/search/catalog'
  )
  return unwrapResponse(response.data, 'Failed to load capability catalog')
}

export async function fetchSearchUsageLogs(
  params: SearchLogParams
): Promise<SearchPage<SearchUsageLog>> {
  const response = await api.get<ApiResponse<SearchPage<SearchUsageLog>>>(
    '/api/search/logs',
    { params: toSearchUsageParams(params) }
  )
  return unwrapResponse(response.data, 'Failed to load usage logs')
}

export async function fetchSearchUsageStats(
  params: Omit<SearchLogParams, 'page' | 'page_size'>
): Promise<SearchUsageStats> {
  const response = await api.get<ApiResponse<SearchUsageStats>>(
    '/api/search/logs/stat',
    { params: toSearchUsageParams(params) }
  )
  return unwrapResponse(response.data, 'Failed to load usage statistics')
}

export async function fetchSearchUpstreamAccounts(): Promise<
  SearchUpstreamAccount[]
> {
  const response = await api.get<ApiResponse<SearchUpstreamAccount[]>>(
    '/api/search/admin/upstream-accounts'
  )
  return unwrapResponse(response.data, 'Failed to load upstream accounts')
}

export async function createSearchUpstreamAccount(
  input: CreateSearchUpstreamAccount
): Promise<SearchUpstreamAccount> {
  const response = await api.post<ApiResponse<SearchUpstreamAccount>>(
    '/api/search/admin/upstream-accounts',
    {
      provider: input.provider,
      name: input.name,
      base_url: input.base_url,
      secret: input.api_key,
      pool_id: input.pool_id,
      weight: input.weight,
      priority: input.priority,
      status: input.status,
    }
  )
  return unwrapResponse(response.data, 'Failed to connect upstream account')
}

export async function updateSearchUpstreamAccount(
  input: UpdateSearchUpstreamAccount
): Promise<SearchUpstreamAccount> {
  const response = await api.patch<ApiResponse<SearchUpstreamAccount>>(
    `/api/search/admin/upstream-accounts/${input.id}`,
    {
      provider: input.provider,
      name: input.name,
      base_url: input.base_url,
      secret: input.api_key?.trim() || '',
      pool_id: input.pool_id,
      weight: input.weight,
      priority: input.priority,
      status: input.status,
    }
  )
  return unwrapResponse(response.data, 'Failed to update upstream account')
}

export async function deleteSearchUpstreamAccount(id: number): Promise<void> {
  const response = await api.delete<ApiResponse<null>>(
    `/api/search/admin/upstream-accounts/${id}`
  )
  if (!response.data.success) {
    throw new Error(
      response.data.message || 'Failed to delete upstream account'
    )
  }
}

export async function testSearchUpstreamAccount(
  id: number
): Promise<SearchUpstreamAccount> {
  const response = await api.post<ApiResponse<SearchUpstreamAccount>>(
    `/api/search/admin/upstream-accounts/${id}/test`
  )
  return unwrapResponse(response.data, 'Upstream account test failed')
}

export async function fetchAdminSearchCatalog(): Promise<
  SearchAdminCatalogItem[]
> {
  const response = await api.get<ApiResponse<SearchAdminCatalogItem[]>>(
    '/api/search/admin/catalog'
  )
  return unwrapResponse(response.data, 'Failed to load capability catalog')
}

export async function syncAdminSearchCatalog(): Promise<SearchCatalogSyncResult> {
  const response = await api.post<ApiResponse<SearchCatalogSyncResult>>(
    '/api/search/admin/catalog/sync',
    undefined,
    { skipErrorHandler: true, skipBusinessError: true }
  )
  return unwrapResponse(response.data, 'Catalog synchronization failed')
}

export async function updateAdminSearchCatalogItem(
  id: string,
  patch: { enabled?: boolean; price_micros?: number }
): Promise<SearchAdminCatalogItem> {
  const response = await api.patch<ApiResponse<SearchAdminCatalogItem>>(
    `/api/search/admin/catalog/${encodeURIComponent(id)}`,
    patch,
    { skipErrorHandler: true, skipBusinessError: true }
  )
  return unwrapResponse(response.data, 'Failed to update capability')
}

export async function fetchSearchCapabilityEnterpriseGrants(
  id: string
): Promise<SearchCapabilityEnterpriseGrants> {
  const response = await api.get<ApiResponse<SearchCapabilityEnterpriseGrants>>(
    `/api/search/admin/catalog/${encodeURIComponent(id)}/grants`
  )
  return unwrapResponse(response.data, 'Failed to load enterprise access')
}

export async function updateSearchCapabilityEnterpriseGrants(
  id: string,
  enterpriseIds: number[]
): Promise<SearchCapabilityEnterpriseGrants> {
  const response = await api.put<ApiResponse<SearchCapabilityEnterpriseGrants>>(
    `/api/search/admin/catalog/${encodeURIComponent(id)}/grants`,
    {
      enterprise_ids: enterpriseIds,
    }
  )
  return unwrapResponse(response.data, 'Failed to save enterprise access')
}

export async function fetchSearchGrantEnterprises(): Promise<
  SearchGrantEnterprise[]
> {
  const enterprises: SearchGrantEnterprise[] = []
  let currentPage = 1
  while (true) {
    const response = await api.get<
      ApiResponse<SearchPage<SearchGrantEnterprise>>
    >('/api/enterprise/admin/', {
      params: { p: currentPage, page_size: 100 },
    })
    const page = unwrapResponse(response.data, 'Failed to load enterprises')
    enterprises.push(...(page.items || []))
    if (enterprises.length >= page.total || page.items.length === 0) break
    currentPage += 1
  }
  return enterprises
}

export async function fetchAdminSearchUsageLogs(
  params: SearchLogParams
): Promise<SearchPage<SearchUsageLog>> {
  const response = await api.get<ApiResponse<SearchPage<SearchUsageLog>>>(
    '/api/search/admin/usage-logs',
    { params: toSearchUsageParams(params) }
  )
  return unwrapResponse(response.data, 'Failed to load usage logs')
}

export async function fetchAdminSearchUsageStats(
  params: Omit<SearchLogParams, 'page' | 'page_size'>
): Promise<SearchUsageStats> {
  const response = await api.get<ApiResponse<SearchUsageStats>>(
    '/api/search/admin/usage-logs/stat',
    { params: toSearchUsageParams(params) }
  )
  return unwrapResponse(response.data, 'Failed to load usage statistics')
}

export async function reconcileAdminSearchUsage(
  id: number,
  input: SearchUsageReconciliationInput
): Promise<SearchUsageReconciliationResult> {
  const response = await api.post<ApiResponse<SearchUsageReconciliationResult>>(
    `/api/search/admin/usage-logs/${id}/reconcile`,
    input
  )
  return unwrapResponse(response.data, 'Failed to reconcile usage request')
}

export async function exportAdminSearchUsageLogs(
  params: Omit<SearchLogParams, 'page' | 'page_size'>
): Promise<Blob> {
  const response = await api.get<Blob>('/api/search/admin/usage-logs/export', {
    params: toSearchUsageParams(params),
    responseType: 'blob',
  })
  return response.data
}

function toSearchUsageParams(
  params: SearchLogParams | Omit<SearchLogParams, 'page' | 'page_size'>
) {
  const { range, ...filters } = params
  if ('page' in filters) {
    const { page, ...pagedFilters } = filters
    return { ...pagedFilters, p: page, days: range }
  }
  return { ...filters, days: range }
}
