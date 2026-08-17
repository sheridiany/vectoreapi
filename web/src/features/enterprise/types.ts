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
export type EnterpriseRankingPeriod = 'today' | 'week' | 'month' | 'custom'

export interface Enterprise {
  id: number
  name: string
  code: string
  status: number
  registration_enabled: boolean
  registration_mode: string
}

export interface EnterpriseMembership {
  id: number
  enterprise_id: number
  user_id: number
  role: string
  status: number
  invited_by: number
  joined_at: number
  updated_at?: number
  user?: {
    id: number
    username: string
    display_name: string
    email: string
  }
}

export interface EnterpriseMemberCandidate {
  id: number
  username: string
  display_name: string
  email: string
}

export interface EnterpriseInvitation {
  id: number
  enterprise_id: number
  status: number
  expires_at: number
  max_uses: number
  used_count: number
  created_by: number
  created_at: number
}

export interface EnterprisePage<T> {
  success: boolean
  data?: {
    items: T[]
    total: number
    page: number
    page_size: number
  }
  message?: string
}

export interface EnterpriseRanking {
  rank: number
  enterprise_id: number
  name: string
  net_quota: number
  total_tokens: number
  request_count: number
  active_users: number
  growth_pct: number
}

export interface EnterpriseMemberRanking {
  rank: number
  user_id: number
  username: string
  net_quota: number
  total_tokens: number
  request_count: number
  growth_pct: number
}

export interface EnterpriseRankingsResponse {
  enterprise_id?: number
  period: EnterpriseRankingPeriod
  start_at: number
  end_at: number
  enterprises: EnterpriseRanking[]
  enterprise?: EnterpriseRanking
  members?: EnterpriseMemberRanking[]
}

export interface EnterpriseUsageDaily {
  start_at: number
  end_at: number
  net_quota: number
  total_tokens: number
  request_count: number
}

export interface EnterpriseUsageModel {
  model_name: string
  net_quota: number
  total_tokens: number
  request_count: number
}

export interface EnterpriseAnalyticsResponse {
  enterprise_id: number
  period: EnterpriseRankingPeriod
  start_at: number
  end_at: number
  daily: EnterpriseUsageDaily[]
  models: EnterpriseUsageModel[]
}

export interface EnterpriseBudgetStatus {
  enterprise_id: number
  budget_quota: number
  alert_threshold: number
  consumed_quota: number
  remaining_quota: number
  usage_percentage: number
  alert_level: 'none' | 'warning' | 'exceeded'
  period_start: number
  period_end: number
}

export interface EnterpriseKeyPolicyOperation {
  id: number
  enterprise_id: number
  initiated_by: number
  status: 'applied' | 'rolled_back'
  changed_count: number
  rollback_skipped_count: number
  created_at: number
  rolled_back_at: number
}

export interface EnterpriseKeyPolicySummary {
  enterprise_id: number
  token_group_policy: 'auto'
  active_member_count: number
  members_with_keys: number
  total_key_count: number
  auto_key_count: number
  convertible_key_count: number
  last_operation?: EnterpriseKeyPolicyOperation
}

export interface EnterpriseAuditLog {
  id: number
  user_id: number
  created_at: number
  type: number
  username: string
  model_name: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  group: string
  request_id: string
  other: string
}
