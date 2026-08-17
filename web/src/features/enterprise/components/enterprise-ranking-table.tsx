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

import type { EnterpriseRanking } from '../types'

export function EnterpriseRankingTable(props: { data: EnterpriseRanking[] }) {
  const { t } = useTranslation()
  const columns: StaticDataTableColumn<EnterpriseRanking>[] = [
    {
      id: 'rank',
      header: t('Rank'),
      className: 'w-16',
      cell: (row) => <span className='font-mono tabular-nums'>{row.rank}</span>,
    },
    {
      id: 'enterprise',
      header: t('Enterprise'),
      cell: (row) => (
        <div className='min-w-0'>
          <p className='truncate font-medium'>{row.name}</p>
          <p className='text-muted-foreground mt-0.5 font-mono text-xs'>
            ID {row.enterprise_id}
          </p>
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
      id: 'active-users',
      header: t('Active members'),
      className: 'hidden text-right lg:table-cell',
      cellClassName: 'hidden text-right font-mono tabular-nums lg:table-cell',
      cell: (row) => formatNumber(row.active_users),
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
  ]

  return (
    <StaticDataTable
      columns={columns}
      data={props.data}
      getRowKey={(row) => row.enterprise_id}
      tableClassName='min-w-[760px]'
      className='-mx-2 sm:mx-0'
      emptyContent={
        <EmptyState
          className='min-h-[220px]'
          title={t('No Data')}
          description={t('No enterprise activity in this period.')}
        />
      }
    />
  )
}
