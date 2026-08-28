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
import { Download } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'
import { useAuthStore } from '@/stores/auth-store'

import { useEnterpriseRankings } from '../hooks/use-enterprise-rankings'
import { downloadCsv } from '../lib/ranking-export'
import { getCustomRankingRange } from '../lib/ranking-range'
import type { EnterpriseRankingPeriod } from '../types'
import { EnterpriseShell } from './enterprise-shell'
import { MemberRankingTable } from './member-ranking-table'
import { RankingPeriodControls } from './ranking-period-controls'
import { RankingRangeSummary } from './ranking-range-summary'

export function EnterpriseRankingsPage() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<EnterpriseRankingPeriod>('month')
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')
  const customRange = getCustomRankingRange(customStart, customEnd)
  const customRangeReady = period !== 'custom' || Boolean(customRange)
  const user = useAuthStore((state) => state.auth.user)
  const query = useEnterpriseRankings(
    user?.enterprise?.id,
    period,
    customRange?.start,
    customRange?.end,
    Boolean(user?.enterprise?.id) && customRangeReady
  )
  const data = query.data?.data
  const rows = data?.members ?? []

  function handleExport() {
    downloadCsv(`member-ranking-${period}.csv`, [
      [
        t('Rank'),
        t('Member'),
        t('Net consumption'),
        t('Tokens'),
        t('Requests'),
        t('Growth'),
      ],
      ...rows.map((row) => [
        row.rank,
        row.display_name || row.username,
        row.net_quota,
        row.total_tokens,
        row.request_count,
        `${row.growth_pct.toFixed(1)}%`,
      ]),
    ])
  }

  return (
    <EnterpriseShell
      section='rankings'
      title={t('Member consumption rankings')}
      description={t('Compare member usage for the selected period.')}
    >
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
        title={t('Member consumption rankings')}
        description={t('Net consumption, tokens, requests and growth.')}
        action={
          <Button
            variant='outline'
            size='sm'
            onClick={handleExport}
            disabled={!rows.length || query.isLoading}
          >
            <Download aria-hidden='true' />
            {t('Export CSV')}
          </Button>
        }
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
          <MemberRankingTable data={rows} />
        )}
      </TitledCard>
    </EnterpriseShell>
  )
}
