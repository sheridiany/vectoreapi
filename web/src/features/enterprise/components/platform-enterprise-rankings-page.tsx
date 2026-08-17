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
import { Link } from '@tanstack/react-router'
import { ArrowLeft, Download } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'

import { useEnterpriseRankings } from '../hooks/use-enterprise-rankings'
import { downloadCsv } from '../lib/ranking-export'
import { getCustomRankingRange } from '../lib/ranking-range'
import type { EnterpriseRankingPeriod } from '../types'
import { EnterpriseRankingTable } from './enterprise-ranking-table'
import { RankingPeriodControls } from './ranking-period-controls'
import { RankingRangeSummary } from './ranking-range-summary'

export function PlatformEnterpriseRankingsPage() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<EnterpriseRankingPeriod>('month')
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')
  const customRange = useMemo(
    () => getCustomRankingRange(customStart, customEnd),
    [customEnd, customStart]
  )
  const customRangeReady = period !== 'custom' || Boolean(customRange)
  const query = useEnterpriseRankings(
    undefined,
    period,
    customRange?.start,
    customRange?.end,
    customRangeReady
  )
  const data = query.data?.data
  const rows = data?.enterprises ?? []

  function handleExport() {
    downloadCsv(`enterprise-ranking-${period}.csv`, [
      [
        t('Rank'),
        t('Enterprise'),
        t('Net consumption'),
        t('Tokens'),
        t('Requests'),
        t('Active members'),
        t('Growth'),
      ],
      ...rows.map((row) => [
        row.rank,
        row.name,
        row.net_quota,
        row.total_tokens,
        row.request_count,
        row.active_users,
        `${row.growth_pct.toFixed(1)}%`,
      ]),
    ])
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Enterprise consumption rankings')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          render={<Link to='/enterprise/admin' />}
          nativeButton={false}
        >
          <ArrowLeft aria-hidden='true' />
          {t('Enterprise management')}
        </Button>
        <Button
          variant='outline'
          onClick={handleExport}
          disabled={!rows.length || query.isLoading}
        >
          <Download aria-hidden='true' />
          {t('Export CSV')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-[1440px] space-y-5'>
          <div className='border-border/60 bg-card rounded-xl border p-4 shadow-xs sm:p-5'>
            <h1 className='text-xl font-semibold tracking-tight'>
              {t('Enterprise consumption rankings')}
            </h1>
            <p className='text-muted-foreground mt-1 max-w-3xl text-sm leading-6'>
              {t(
                'Compare enterprise usage for the selected period. Enterprises without activity in the period are not ranked.'
              )}
            </p>
          </div>

          <RankingPeriodControls
            period={period}
            customStart={customStart}
            customEnd={customEnd}
            onPeriodChange={setPeriod}
            onCustomStartChange={setCustomStart}
            onCustomEndChange={setCustomEnd}
          />

          {data && customRangeReady && (
            <RankingRangeSummary startAt={data.start_at} endAt={data.end_at} />
          )}

          <TitledCard
            title={t('Enterprise consumption rankings')}
            description={t('Net consumption, tokens, requests and growth.')}
          >
            {!customRangeReady && (
              <p className='text-muted-foreground px-2 py-10 text-center text-sm'>
                {t('Select a valid custom date range to load rankings.')}
              </p>
            )}
            {customRangeReady && query.isLoading && (
              <LoadingState message={t('Loading rankings')} />
            )}
            {customRangeReady && query.isError && (
              <ErrorState
                title={t('Unable to load enterprise rankings')}
                onRetry={() => void query.refetch()}
              />
            )}
            {customRangeReady && !query.isLoading && !query.isError && (
              <EnterpriseRankingTable data={rows} />
            )}
          </TitledCard>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
