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
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { StatusBadge } from '@/components/status-badge'
import { formatNumber } from '@/lib/format'

import type { EnterpriseMemberRanking } from '../types'

export function MemberRankingTable(props: {
  data: EnterpriseMemberRanking[]
  compact?: boolean
}) {
  const { t } = useTranslation()
  const columns: StaticDataTableColumn<EnterpriseMemberRanking>[] = [
    {
      id: 'rank',
      header: t('Rank'),
      className: 'w-16',
      cell: (row) => <span className='font-mono tabular-nums'>{row.rank}</span>,
    },
    {
      id: 'member',
      header: t('Member'),
      cell: (row) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{row.username}</p>
          {!props.compact && (
            <p className='text-muted-foreground mt-0.5 font-mono text-xs'>
              ID {row.user_id}
            </p>
          )}
        </div>
      ),
    },
    {
      id: 'net-quota',
      header: t('Net consumption'),
      className: 'text-right',
      cellClassName: 'text-right font-mono tabular-nums',
      cell: (row) => formatNumber(row.net_quota),
    },
    {
      id: 'tokens',
      header: t('Tokens'),
      className: 'hidden text-right sm:table-cell',
      cellClassName: 'hidden text-right font-mono tabular-nums sm:table-cell',
      cell: (row) => formatNumber(row.total_tokens),
    },
    {
      id: 'requests',
      header: t('Requests'),
      className: 'hidden text-right md:table-cell',
      cellClassName: 'hidden text-right font-mono tabular-nums md:table-cell',
      cell: (row) => formatNumber(row.request_count),
    },
    {
      id: 'growth',
      header: t('Growth'),
      className: 'text-right',
      cellClassName: 'text-right',
      cell: (row) => (
        <StatusBadge
          label={`${row.growth_pct >= 0 ? '+' : ''}${row.growth_pct.toFixed(1)}%`}
          variant={row.growth_pct >= 0 ? 'success' : 'danger'}
          copyable={false}
        />
      ),
    },
    {
      id: 'details',
      header: '',
      className: 'w-28 text-right',
      cellClassName: 'text-right',
      cell: (row) => (
        <a
          href={`/enterprise/audit?username=${encodeURIComponent(row.username)}`}
          className='text-primary hover:text-primary/80 text-xs font-medium'
        >
          {t('View details')}
        </a>
      ),
    },
  ]

  return (
    <StaticDataTable
      columns={columns}
      data={props.data}
      getRowKey={(row) => row.user_id}
      tableClassName='min-w-[620px]'
      className='-mx-2 sm:mx-0'
      emptyContent={
        <EmptyState
          className='min-h-[220px]'
          title={t('No Data')}
          description={t('No enterprise activity yet.')}
        />
      }
    />
  )
}
