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
import { useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { Users, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { IconBadge } from '@/components/ui/icon-badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useTheme } from '@/context/theme-provider'
import { getUserQuotaDataByUsers } from '@/features/dashboard/api'
import {
  TIME_GRANULARITY_OPTIONS,
  TIME_RANGE_PRESETS,
} from '@/features/dashboard/constants'
import {
  getDefaultDays,
  saveGranularity,
  processUserChartData,
} from '@/features/dashboard/lib'
import type {
  ProcessedUserChartData,
  UserChartsFilters,
} from '@/features/dashboard/types'
import { getEnterprises } from '@/features/enterprise/api'
import { formatQuota } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { getRollingDateRange, type TimeGranularity } from '@/lib/time'
import { VCHART_OPTION } from '@/lib/vchart'
import { useAuthStore } from '@/stores/auth-store'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

const USER_CHARTS: {
  value: string
  labelKey: string
  specKey: keyof ProcessedUserChartData
}[] = [
  {
    value: 'rank',
    labelKey: 'User Consumption Ranking',
    specKey: 'spec_user_rank',
  },
  {
    value: 'trend',
    labelKey: 'User Consumption Trend',
    specKey: 'spec_user_trend',
  },
]

const TOP_USER_LIMIT_OPTIONS = [5, 10, 20, 50]
const RANK_TONE_CLASSES = [
  'bg-amber-500/15 text-amber-600 dark:text-amber-300',
  'bg-slate-500/15 text-slate-600 dark:text-slate-300',
  'bg-orange-500/15 text-orange-600 dark:text-orange-300',
  'bg-muted text-muted-foreground',
]

interface UserChartsProps {
  filters: UserChartsFilters
  onFiltersChange: (filters: UserChartsFilters) => void
}

interface UserRanking {
  rank: number
  userId?: number
  displayName: string
  quota: number
}

interface UserRankingListProps {
  rankings: UserRanking[]
  total: number
  isLoading: boolean
}

function UserRankingList(props: UserRankingListProps) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return (
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
        {['first', 'second', 'third'].map((skeletonKey) => (
          <Skeleton key={skeletonKey} className='h-28 w-full rounded-xl' />
        ))}
      </div>
    )
  }

  if (props.rankings.length === 0) {
    return (
      <div className='text-muted-foreground flex min-h-28 items-center justify-center rounded-xl border border-dashed text-sm'>
        {t('No data available')}
      </div>
    )
  }

  return (
    <div
      aria-label={t('User Consumption Ranking')}
      className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'
      role='list'
    >
      {props.rankings.map((ranking) => {
        const share = props.total > 0 ? (ranking.quota / props.total) * 100 : 0
        const initials = ranking.displayName
          .trim()
          .split(/\s+/)
          .map((part) => part[0])
          .join('')
          .slice(0, 2)
          .toUpperCase()

        return (
          <div
            key={ranking.userId ?? ranking.displayName}
            className='bg-card/60 hover:bg-muted/40 flex min-h-28 items-center gap-3 rounded-xl border p-3 transition-colors sm:p-4'
            role='listitem'
          >
            <div
              aria-label={`${t('Rankings')} ${ranking.rank}`}
              className={`flex size-8 shrink-0 items-center justify-center rounded-full text-sm font-semibold tabular-nums ${RANK_TONE_CLASSES[Math.min(ranking.rank - 1, RANK_TONE_CLASSES.length - 1)]}`}
            >
              {ranking.rank}
            </div>
            <Avatar size='sm' className='shrink-0'>
              <AvatarFallback>{initials || '?'}</AvatarFallback>
            </Avatar>
            <div className='min-w-0 flex-1'>
              <div className='truncate text-sm font-medium'>
                {ranking.displayName}
              </div>
              <div className='mt-1 flex items-center justify-between gap-2'>
                <span className='text-muted-foreground text-xs'>
                  {share.toFixed(1)}% {t('Share')}
                </span>
                <span className='text-sm font-semibold tabular-nums'>
                  {formatQuota(ranking.quota)}
                </span>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export function UserCharts(props: UserChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const isRoot = userRole === ROLE.SUPER_ADMIN
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  // The selection is owned by the dashboard parent so it persists across
  // sub-section switches; the rolling window is derived from the chosen range.
  const timeGranularity = props.filters.timeGranularity
  const selectedRange = props.filters.selectedRange
  const topUserLimit = props.filters.topUserLimit
  const enterpriseId = props.filters.enterpriseId
  const onFiltersChange = props.onFiltersChange

  const timeRange = useMemo(() => {
    const { start, end } = getRollingDateRange(selectedRange)
    return {
      start_timestamp: Math.floor(start.getTime() / 1000),
      end_timestamp: Math.floor(end.getTime() / 1000),
    }
  }, [selectedRange])

  const handleRangeChange = useCallback(
    (days: number) => {
      onFiltersChange({ ...props.filters, selectedRange: days })
    },
    [onFiltersChange, props.filters]
  )

  const handleGranularityChange = useCallback(
    (g: TimeGranularity) => {
      saveGranularity(g)
      onFiltersChange({
        ...props.filters,
        timeGranularity: g,
        selectedRange: getDefaultDays(g),
      })
    },
    [onFiltersChange, props.filters]
  )

  const handleTopUserLimitChange = useCallback(
    (limit: number) => {
      onFiltersChange({ ...props.filters, topUserLimit: limit })
    },
    [onFiltersChange, props.filters]
  )

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      themeManagerRef.current = ThemeManager
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    updateTheme()
  }, [resolvedTheme])

  const { data: userData, isLoading } = useQuery({
    queryKey: ['dashboard', 'user-quota', timeRange, enterpriseId ?? 'all'],
    queryFn: () =>
      getUserQuotaDataByUsers({
        ...timeRange,
        enterprise_id: enterpriseId,
      }),
    select: (res) => (res.success ? res.data : []),
    staleTime: 60_000,
  })

  const enterprisesQuery = useQuery({
    queryKey: ['enterprise-admin', 'enterprises'],
    queryFn: async () => {
      const response = await getEnterprises()
      if (!response.success) {
        throw new Error(response.message || t('Unable to load enterprises'))
      }
      return response.data?.items ?? []
    },
    enabled: isRoot,
    staleTime: 60_000,
  })

  const handleEnterpriseChange = useCallback(
    (value: string | null) => {
      onFiltersChange({
        ...props.filters,
        enterpriseId: value && value !== 'all' ? Number(value) : undefined,
      })
    },
    [onFiltersChange, props.filters]
  )

  const selectedEnterprise = enterprisesQuery.data?.find(
    (enterprise) => enterprise.id === enterpriseId
  )
  const enterpriseTriggerLabel =
    enterpriseId === undefined
      ? t('All')
      : (selectedEnterprise?.name ?? String(enterpriseId))

  const chartData = useMemo(
    () =>
      processUserChartData(
        isLoading ? [] : (userData ?? []),
        timeGranularity,
        t,
        topUserLimit
      ),
    [userData, isLoading, timeGranularity, t, topUserLimit]
  )

  const userRankings = useMemo(() => {
    const totals = new Map<
      string,
      { userId?: number; displayName: string; quota: number }
    >()

    for (const item of userData ?? []) {
      const userId = item.user_id && item.user_id > 0 ? item.user_id : undefined
      const key = userId ? `id:${userId}` : `name:${item.username || 'unknown'}`
      const current = totals.get(key)
      const displayName =
        item.display_name?.trim() || item.username || 'unknown'
      totals.set(key, {
        userId,
        displayName: current?.displayName || displayName,
        quota: (current?.quota ?? 0) + (Number(item.quota) || 0),
      })
    }

    return [...totals.values()]
      .sort((a, b) => b.quota - a.quota)
      .slice(0, topUserLimit)
      .map((ranking, index) => ({
        rank: index + 1,
        ...ranking,
      }))
  }, [topUserLimit, userData])

  const userRankingsTotal = userRankings.reduce(
    (total, ranking) => total + ranking.quota,
    0
  )

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-center gap-1.5 pb-1 sm:gap-2'>
        <Tabs
          value={String(selectedRange)}
          onValueChange={(value) => handleRangeChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            {TIME_RANGE_PRESETS.map((preset) => (
              <TabsTrigger
                key={preset.days}
                value={String(preset.days)}
                className='px-2.5 text-xs'
              >
                {t(preset.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Tabs
          value={timeGranularity}
          onValueChange={(value) =>
            handleGranularityChange(value as TimeGranularity)
          }
          className='shrink-0'
        >
          <TabsList>
            {TIME_GRANULARITY_OPTIONS.map((opt) => (
              <TabsTrigger
                key={opt.value}
                value={opt.value}
                className='px-2.5 text-xs'
              >
                {t(opt.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Tabs
          value={String(topUserLimit)}
          onValueChange={(value) => handleTopUserLimitChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            <span className='text-muted-foreground px-2 text-xs font-medium whitespace-nowrap'>
              {t('Top Users')}
            </span>
            {TOP_USER_LIMIT_OPTIONS.map((limit) => (
              <TabsTrigger
                key={limit}
                value={String(limit)}
                className='px-2.5 text-xs'
              >
                {t('Top {{count}}', { count: limit })}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {isRoot && (
          <div className='ml-auto flex shrink-0 items-center gap-2'>
            <span className='text-muted-foreground text-xs font-medium whitespace-nowrap'>
              {t('Enterprise filter')}
            </span>
            <Select
              value={enterpriseId === undefined ? 'all' : String(enterpriseId)}
              onValueChange={handleEnterpriseChange}
            >
              <SelectTrigger
                aria-label={t('Enterprise filter')}
                className='w-[180px]'
                disabled={
                  enterprisesQuery.isLoading || enterprisesQuery.isError
                }
              >
                <SelectValue placeholder={t('All')}>
                  {enterpriseTriggerLabel}
                </SelectValue>
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectItem value='all'>{t('All')}</SelectItem>
                {enterprisesQuery.data?.map((enterprise) => (
                  <SelectItem key={enterprise.id} value={String(enterprise.id)}>
                    {enterprise.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        {isLoading && (
          <Loader2 className='text-muted-foreground size-4 animate-spin' />
        )}
      </div>

      <div className='grid gap-3'>
        {USER_CHARTS.map((chart) => {
          const spec = chartData[chart.specKey]

          return (
            <div
              key={chart.value}
              className='overflow-hidden rounded-lg border'
            >
              <div className='flex w-full items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
                <IconBadge tone='info' size='sm'>
                  <Users />
                </IconBadge>
                <div className='text-sm font-semibold'>{t(chart.labelKey)}</div>
              </div>

              {chart.value === 'rank' ? (
                <div className='space-y-4 p-3 sm:p-5'>
                  <div className='flex items-end justify-between gap-3'>
                    <div>
                      <div className='text-base font-semibold'>
                        {t('User Consumption Ranking')}
                      </div>
                      <div className='text-muted-foreground mt-1 text-sm'>
                        {t('Total:')} {formatQuota(userRankingsTotal)}
                      </div>
                    </div>
                    {userRankings.length > 0 && (
                      <Badge
                        variant='outline'
                        className='shrink-0 tabular-nums'
                      >
                        {userRankings.length}
                      </Badge>
                    )}
                  </div>

                  <UserRankingList
                    rankings={userRankings}
                    total={userRankingsTotal}
                    isLoading={isLoading}
                  />
                </div>
              ) : (
                <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
                  {isLoading ? (
                    <Skeleton className='h-full w-full' />
                  ) : (
                    themeReady &&
                    spec && (
                      <VChart
                        key={`user-${chart.value}-${topUserLimit}-${resolvedTheme}`}
                        spec={{
                          ...spec,
                          theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                          background: 'transparent',
                        }}
                        option={VCHART_OPTION}
                      />
                    )
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
