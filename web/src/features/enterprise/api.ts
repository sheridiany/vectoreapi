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

import type {
  Enterprise,
  EnterpriseAuditLog,
  EnterpriseAnalyticsResponse,
  EnterpriseBudgetStatus,
  EnterpriseInvitation,
  EnterpriseKeyPolicyOperation,
  EnterpriseKeyPolicySummary,
  EnterpriseMemberCandidate,
  EnterpriseMembership,
  EnterprisePage,
  EnterpriseRankingPeriod,
  EnterpriseRankingsResponse,
} from './types'

interface EnterpriseRankingParams {
  enterpriseId?: number
  period: EnterpriseRankingPeriod
  start?: number
  end?: number
}

export async function getEnterpriseRankings(
  params: EnterpriseRankingParams
): Promise<{ success: boolean; data: EnterpriseRankingsResponse }> {
  const path = params.enterpriseId
    ? `/api/enterprise/${params.enterpriseId}/rankings`
    : '/api/enterprise/admin/rankings'
  const res = await api.get(path, {
    params: {
      period: params.period,
      start: params.start,
      end: params.end,
    },
  })
  return res.data
}

export async function getEnterpriseAnalytics(
  params: EnterpriseRankingParams
): Promise<{
  success: boolean
  data?: EnterpriseAnalyticsResponse
  message?: string
}> {
  if (!params.enterpriseId) {
    return { success: false, message: 'Enterprise is required' }
  }
  const res = await api.get(
    `/api/enterprise/${params.enterpriseId}/analytics`,
    {
      params: { period: params.period, start: params.start, end: params.end },
    }
  )
  return res.data
}

export async function getEnterpriseBudget(enterpriseId: number): Promise<{
  success: boolean
  data?: EnterpriseBudgetStatus
  message?: string
}> {
  const res = await api.get(`/api/enterprise/${enterpriseId}/budget`)
  return res.data
}

export async function updateEnterpriseBudget(
  enterpriseId: number,
  data: { budget_quota: number; alert_threshold: number }
): Promise<{ success: boolean; message?: string }> {
  const res = await api.put(`/api/enterprise/${enterpriseId}/budget`, data)
  return res.data
}

function enterpriseAdminPath(enterpriseId?: number) {
  return enterpriseId
    ? `/api/enterprise/${enterpriseId}`
    : '/api/enterprise/admin'
}

export async function getEnterprises(): Promise<EnterprisePage<Enterprise>> {
  const res = await api.get('/api/enterprise/admin/', {
    params: { p: 1, page_size: 100 },
  })
  return res.data
}

export async function createEnterprise(data: {
  name: string
  code: string
}): Promise<{ success: boolean; data?: Enterprise; message?: string }> {
  const res = await api.post('/api/enterprise/admin/', data)
  return res.data
}

export async function updateEnterprise(
  enterpriseId: number,
  data: Partial<
    Pick<
      Enterprise,
      'name' | 'code' | 'status' | 'registration_enabled' | 'registration_mode'
    >
  >
): Promise<{ success: boolean; data?: Enterprise; message?: string }> {
  const res = await api.put(`/api/enterprise/admin/${enterpriseId}`, data)
  return res.data
}

export async function getEnterprise(
  enterpriseId: number
): Promise<{ success: boolean; data?: Enterprise; message?: string }> {
  const res = await api.get(`${enterpriseAdminPath(enterpriseId)}`)
  return res.data
}

export async function updateEnterpriseRegistration(
  enterpriseId: number,
  data: Pick<Enterprise, 'registration_enabled' | 'registration_mode'>
): Promise<{ success: boolean; data?: Enterprise; message?: string }> {
  const res = await api.put(`${enterpriseAdminPath(enterpriseId)}`, data)
  return res.data
}

export async function getEnterpriseKeyPolicy(enterpriseId: number): Promise<{
  success: boolean
  data?: EnterpriseKeyPolicySummary
  message?: string
}> {
  const res = await api.get(`/api/enterprise/${enterpriseId}/key-policy`)
  return res.data
}

export async function applyEnterpriseKeyPolicy(enterpriseId: number): Promise<{
  success: boolean
  data?: EnterpriseKeyPolicyOperation
  message?: string
}> {
  const res = await api.post(`/api/enterprise/${enterpriseId}/key-policy/apply`)
  return res.data
}

export async function rollbackEnterpriseKeyPolicy(
  enterpriseId: number,
  operationId: number
): Promise<{
  success: boolean
  data?: EnterpriseKeyPolicyOperation
  message?: string
}> {
  const res = await api.post(
    `/api/enterprise/${enterpriseId}/key-policy/rollback/${operationId}`
  )
  return res.data
}

export async function getEnterpriseMembers(
  enterpriseId: number
): Promise<EnterprisePage<EnterpriseMembership>> {
  const res = await api.get(`${enterpriseAdminPath(enterpriseId)}/members`, {
    params: { p: 1, page_size: 100 },
  })
  return res.data
}

export async function getEnterpriseLogs(
  enterpriseId: number,
  params: {
    page: number
    pageSize: number
    type?: number
    username?: string
    modelName?: string
    group?: string
    startTimestamp?: number
    endTimestamp?: number
  }
): Promise<EnterprisePage<EnterpriseAuditLog>> {
  const res = await api.get(`/api/enterprise/${enterpriseId}/logs`, {
    params: {
      p: params.page,
      page_size: params.pageSize,
      type: params.type || undefined,
      username: params.username || undefined,
      model_name: params.modelName || undefined,
      group: params.group || undefined,
      start_timestamp: params.startTimestamp || undefined,
      end_timestamp: params.endTimestamp || undefined,
    },
  })
  return res.data
}

export async function getEnterpriseMemberCandidates(
  enterpriseId: number,
  keyword: string
): Promise<{
  success: boolean
  data?: EnterpriseMemberCandidate[]
  message?: string
}> {
  const res = await api.get(
    `${enterpriseAdminPath(enterpriseId)}/member-candidates`,
    { params: { keyword } }
  )
  return res.data
}

export async function assignEnterpriseMember(
  enterpriseId: number,
  data: { user_id: number; role: string }
): Promise<{
  success: boolean
  data?: EnterpriseMembership
  message?: string
}> {
  const res = await api.post(
    `${enterpriseAdminPath(enterpriseId)}/members`,
    data
  )
  return res.data
}

export async function updateEnterpriseMember(
  enterpriseId: number,
  userId: number,
  data: { role: string; status: number }
): Promise<{
  success: boolean
  data?: EnterpriseMembership
  message?: string
}> {
  const res = await api.put(
    `${enterpriseAdminPath(enterpriseId)}/members/${userId}`,
    data
  )
  return res.data
}

export async function getEnterpriseInvitations(
  enterpriseId: number
): Promise<EnterprisePage<EnterpriseInvitation>> {
  const res = await api.get(
    `${enterpriseAdminPath(enterpriseId)}/invitations`,
    {
      params: { p: 1, page_size: 100 },
    }
  )
  return res.data
}

export async function createEnterpriseInvitation(
  enterpriseId: number,
  data: { expires_at: number; max_uses: number }
): Promise<{
  success: boolean
  data?: { code: string; invitation: EnterpriseInvitation }
  message?: string
}> {
  const res = await api.post(
    `${enterpriseAdminPath(enterpriseId)}/invitations`,
    data
  )
  return res.data
}

export async function updateEnterpriseInvitation(
  enterpriseId: number,
  invitationId: number,
  status: number
): Promise<{
  success: boolean
  data?: EnterpriseInvitation
  message?: string
}> {
  const res = await api.patch(
    `${enterpriseAdminPath(enterpriseId)}/invitations/${invitationId}`,
    { status }
  )
  return res.data
}
