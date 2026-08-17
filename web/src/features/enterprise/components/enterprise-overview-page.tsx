/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Building2,
  KeyRound,
  MessageSquare,
  Save,
  Users,
  WalletCards,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { TitledCard } from '@/components/ui/titled-card'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { formatNumber } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  getEnterpriseAnalytics,
  getEnterpriseBudget,
  getEnterpriseMembers,
  updateEnterpriseBudget,
} from '../api'
import { useEnterpriseRankings } from '../hooks/use-enterprise-rankings'
import { EnterpriseShell } from './enterprise-shell'
import { EnterpriseUsageChart } from './enterprise-usage-chart'
import { MemberRankingTable } from './member-ranking-table'

export function EnterpriseOverviewPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const enterpriseId = user?.enterprise?.id
  const canManage =
    user?.enterprise?.role === 'owner' || user?.enterprise?.role === 'admin'
  const [budgetQuota, setBudgetQuota] = useState('')
  const [alertThreshold, setAlertThreshold] = useState('80')
  const rankingQuery = useEnterpriseRankings(
    enterpriseId,
    'month',
    undefined,
    undefined,
    Boolean(enterpriseId)
  )
  const analyticsQuery = useQuery({
    queryKey: ['enterprise', enterpriseId, 'analytics', 'month'],
    queryFn: async () => {
      if (!enterpriseId) return undefined
      const response = await getEnterpriseAnalytics({
        enterpriseId,
        period: 'month',
      })
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load enterprise management data')
        )
      }
      return response.data
    },
    enabled: Boolean(enterpriseId),
  })
  const budgetQuery = useQuery({
    queryKey: ['enterprise', enterpriseId, 'budget'],
    queryFn: async () => {
      if (!enterpriseId) return undefined
      const response = await getEnterpriseBudget(enterpriseId)
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load enterprise management data')
        )
      }
      return response.data
    },
    enabled: Boolean(enterpriseId),
  })
  const membersQuery = useQuery({
    queryKey: ['enterprise', enterpriseId, 'members'],
    queryFn: async () => {
      if (!enterpriseId) return []
      const response = await getEnterpriseMembers(enterpriseId)
      if (!response.success) {
        throw new Error(
          response.message || t('Unable to load enterprise management data')
        )
      }
      return response.data?.items ?? []
    },
    enabled: Boolean(enterpriseId),
  })

  useEffect(() => {
    if (!budgetQuery.data) return
    setBudgetQuota(String(budgetQuery.data.budget_quota))
    setAlertThreshold(String(budgetQuery.data.alert_threshold))
  }, [budgetQuery.data])

  const budgetMutation = useMutation({
    mutationFn: async () => {
      if (!enterpriseId) throw new Error(t('Enterprise is required'))
      const quota = Number(budgetQuota)
      const threshold = Number(alertThreshold)
      if (!Number.isInteger(quota) || quota < 0) {
        throw new Error(t('Enter a valid monthly budget'))
      }
      if (!Number.isInteger(threshold) || threshold < 0 || threshold > 100) {
        throw new Error(t('Enter a valid alert threshold'))
      }
      const response = await updateEnterpriseBudget(enterpriseId, {
        budget_quota: quota,
        alert_threshold: threshold,
      })
      if (!response.success) {
        throw new Error(response.message || t('Unable to save budget settings'))
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['enterprise', enterpriseId, 'budget'],
      })
      toast.success(t('Saved successfully'))
    },
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : t('Unable to save budget settings')
      ),
  })

  if (
    rankingQuery.isLoading ||
    membersQuery.isLoading ||
    analyticsQuery.isLoading ||
    budgetQuery.isLoading
  ) {
    return (
      <EnterpriseShell
        section='overview'
        title={t('Enterprise')}
        description={t('Enterprise overview')}
      >
        <LoadingState message={t('Loading...')} />
      </EnterpriseShell>
    )
  }

  if (
    rankingQuery.isError ||
    membersQuery.isError ||
    analyticsQuery.isError ||
    budgetQuery.isError
  ) {
    return (
      <EnterpriseShell section='overview' title={t('Enterprise')}>
        <ErrorState
          title={t('Unable to load enterprise management data')}
          description={t('Please try again later.')}
          onRetry={() => {
            void rankingQuery.refetch()
            void membersQuery.refetch()
          }}
        />
      </EnterpriseShell>
    )
  }

  const enterprise = rankingQuery.data?.data?.enterprise
  const members = rankingQuery.data?.data?.members ?? []
  const analytics = analyticsQuery.data
  const budget = budgetQuery.data
  const activeMembers =
    membersQuery.data?.filter((member) => member.status === 1).length ?? 0
  const periodLabel = t('This month')

  return (
    <EnterpriseShell
      section='overview'
      title={t('Enterprise')}
      description={t('Enterprise overview')}
    >
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <Card>
          <CardContent className='p-4'>
            <StatCard
              title={t('Net consumption')}
              value={formatNumber(enterprise?.net_quota ?? 0)}
              description={periodLabel}
              icon={Activity}
              tone='accent-1'
            />
          </CardContent>
        </Card>
        <Card>
          <CardContent className='p-4'>
            <StatCard
              title={t('Tokens')}
              value={formatNumber(enterprise?.total_tokens ?? 0)}
              description={periodLabel}
              icon={MessageSquare}
              tone='accent-2'
            />
          </CardContent>
        </Card>
        <Card>
          <CardContent className='p-4'>
            <StatCard
              title={t('Requests')}
              value={formatNumber(enterprise?.request_count ?? 0)}
              description={periodLabel}
              icon={KeyRound}
              tone='accent-3'
            />
          </CardContent>
        </Card>
        <Card>
          <CardContent className='p-4'>
            <StatCard
              title={t('Active members')}
              value={formatNumber(activeMembers)}
              description={t('Current status')}
              icon={Users}
              tone='accent-1'
              details={[
                {
                  label: t('Total members'),
                  value: formatNumber(membersQuery.data?.length ?? 0),
                },
                {
                  label: t('Growth'),
                  value: formatGrowth(enterprise?.growth_pct ?? 0),
                  tone: 'success',
                },
              ]}
            />
          </CardContent>
        </Card>
      </div>

      <div className='grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]'>
        <TitledCard
          title={t('Member consumption rankings')}
          description={t('This month')}
          icon={<Users className='size-4' aria-hidden='true' />}
          action={
            <span className='text-muted-foreground text-xs'>
              {t('Top members')}
            </span>
          }
        >
          {members.length > 0 ? (
            <MemberRankingTable data={members.slice(0, 5)} compact />
          ) : (
            <EmptyState
              className='min-h-[220px]'
              title={t('No Data')}
              description={t('No enterprise activity yet.')}
            />
          )}
        </TitledCard>

        <TitledCard
          title={t('Enterprise status')}
          description={t('Current status')}
          icon={<Building2 className='size-4' aria-hidden='true' />}
        >
          <div className='space-y-4'>
            <div className='bg-muted/35 rounded-lg border px-3 py-3'>
              <p className='text-muted-foreground text-xs'>{t('Enterprise')}</p>
              <p className='mt-1 truncate text-sm font-semibold'>
                {user?.enterprise?.name || t('Enterprise')}
              </p>
              <p className='text-muted-foreground mt-1 font-mono text-xs'>
                {user?.enterprise?.code || '--'}
              </p>
            </div>
            <div className='grid grid-cols-2 gap-3 text-sm'>
              <div>
                <p className='text-muted-foreground text-xs'>{t('Role')}</p>
                <p className='mt-1 font-medium'>
                  {t(getEnterpriseRoleLabel(user?.enterprise?.role))}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground text-xs'>{t('Status')}</p>
                <p className='text-success mt-1 font-medium'>{t('Active')}</p>
              </div>
            </div>
          </div>
        </TitledCard>
      </div>

      <div className='grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]'>
        <TitledCard
          title={t('Usage trend')}
          description={t('Daily net consumption for this month.')}
          icon={<BarChart3 className='size-4' aria-hidden='true' />}
        >
          {analytics?.daily.length ? (
            <EnterpriseUsageChart data={analytics.daily} />
          ) : (
            <EmptyState
              className='min-h-[260px]'
              title={t('No Data')}
              description={t('No enterprise activity yet.')}
            />
          )}
        </TitledCard>

        <TitledCard
          title={t('Model consumption')}
          description={t('Models ranked by net consumption.')}
          icon={<WalletCards className='size-4' aria-hidden='true' />}
        >
          {analytics?.models.length ? (
            <div className='space-y-3'>
              {analytics.models.slice(0, 6).map((item) => (
                <div
                  key={item.model_name || 'unknown'}
                  className='flex items-center justify-between gap-3 text-sm'
                >
                  <span className='min-w-0 truncate font-mono text-xs'>
                    {item.model_name || t('Unknown model')}
                  </span>
                  <span className='shrink-0 font-mono tabular-nums'>
                    {formatNumber(item.net_quota)}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState
              className='min-h-[220px]'
              title={t('No Data')}
              description={t('No enterprise activity yet.')}
            />
          )}
        </TitledCard>
      </div>

      <TitledCard
        title={t('Budget & alerts')}
        description={t(
          'Track this month consumption and configure a warning threshold.'
        )}
        icon={<AlertTriangle className='size-4' aria-hidden='true' />}
      >
        <div className='space-y-4'>
          <div className='grid gap-3 sm:grid-cols-3'>
            <div className='bg-muted/35 rounded-lg border px-3 py-3'>
              <p className='text-muted-foreground text-xs'>
                {t('Monthly budget')}
              </p>
              <p className='mt-1 font-mono text-lg font-semibold'>
                {budget?.budget_quota
                  ? formatNumber(budget.budget_quota)
                  : t('Not configured')}
              </p>
            </div>
            <div className='bg-muted/35 rounded-lg border px-3 py-3'>
              <p className='text-muted-foreground text-xs'>
                {t('Consumed this month')}
              </p>
              <p className='mt-1 font-mono text-lg font-semibold'>
                {formatNumber(budget?.consumed_quota ?? 0)}
              </p>
            </div>
            <div className='bg-muted/35 rounded-lg border px-3 py-3'>
              <p className='text-muted-foreground text-xs'>
                {t('Budget status')}
              </p>
              <div className='mt-1'>
                <StatusBadge
                  label={t(getBudgetStatusLabel(budget?.alert_level))}
                  variant={getBudgetStatusVariant(budget?.alert_level)}
                  copyable={false}
                />
              </div>
            </div>
          </div>
          {budget?.budget_quota ? (
            <Progress
              value={Math.min(100, budget.usage_percentage)}
              aria-label={t('Budget usage')}
            />
          ) : null}
          {canManage && (
            <form
              className='grid gap-3 border-t pt-4 sm:grid-cols-[1fr_180px_auto] sm:items-end'
              onSubmit={(event) => {
                event.preventDefault()
                budgetMutation.mutate()
              }}
            >
              <label className='space-y-1.5 text-sm'>
                <span className='text-muted-foreground text-xs'>
                  {t('Monthly budget')}
                </span>
                <Input
                  type='number'
                  min='0'
                  value={budgetQuota}
                  onChange={(event) => setBudgetQuota(event.target.value)}
                  placeholder='0'
                />
              </label>
              <label className='space-y-1.5 text-sm'>
                <span className='text-muted-foreground text-xs'>
                  {t('Alert threshold (%)')}
                </span>
                <Input
                  type='number'
                  min='0'
                  max='100'
                  value={alertThreshold}
                  onChange={(event) => setAlertThreshold(event.target.value)}
                />
              </label>
              <Button type='submit' disabled={budgetMutation.isPending}>
                <Save aria-hidden='true' />
                {t('Save settings')}
              </Button>
            </form>
          )}
        </div>
      </TitledCard>
    </EnterpriseShell>
  )
}

function formatGrowth(value: number) {
  return `${value >= 0 ? '+' : ''}${value.toFixed(1)}%`
}

function getEnterpriseRoleLabel(role?: string) {
  if (role === 'owner') return 'Owner'
  if (role === 'admin') return 'Admin'
  if (role === 'auditor') return 'Auditor'
  return 'Member'
}

function getBudgetStatusLabel(level?: string) {
  if (level === 'exceeded') return 'Exceeded'
  if (level === 'warning') return 'Warning'
  return 'Within budget'
}

function getBudgetStatusVariant(level?: string) {
  if (level === 'exceeded') return 'danger' as const
  if (level === 'warning') return 'warning' as const
  return 'success' as const
}
