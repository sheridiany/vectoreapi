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
*/
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef, PaginationState } from '@tanstack/react-table'
import { ClipboardList, Download, RotateCcw, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TitledCard } from '@/components/ui/titled-card'
import { formatNumber, formatTimestamp } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import { getEnterpriseLogs } from '../api'
import { downloadCsv } from '../lib/ranking-export'
import type { EnterpriseAuditLog } from '../types'
import { EnterpriseShell } from './enterprise-shell'

type AuditFilters = {
  type: number
  username: string
  modelName: string
  group: string
  startDate: string
  endDate: string
}

const EMPTY_FILTERS: AuditFilters = {
  type: 0,
  username: '',
  modelName: '',
  group: '',
  startDate: '',
  endDate: '',
}

const LOG_TYPES = [
  { value: 0, label: 'All Types' },
  { value: 1, label: 'Top-up' },
  { value: 2, label: 'Consume' },
  { value: 3, label: 'Manage' },
  { value: 4, label: 'System' },
  { value: 5, label: 'Error' },
  { value: 6, label: 'Refund' },
  { value: 7, label: 'Login' },
] as const

export function EnterpriseAuditPage() {
  const { t } = useTranslation()
  const enterpriseId = useAuthStore((state) => state.auth.user?.enterprise?.id)
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const initialUsername =
    new URLSearchParams(window.location.search).get('username') ?? ''
  const [draftFilters, setDraftFilters] = useState<AuditFilters>(() => ({
    ...EMPTY_FILTERS,
    username: initialUsername,
  }))
  const [filters, setFilters] = useState<AuditFilters>(() => ({
    ...EMPTY_FILTERS,
    username: initialUsername,
  }))
  const [isExporting, setIsExporting] = useState(false)

  const query = useQuery({
    queryKey: ['enterprise-audit', enterpriseId, pagination, filters],
    queryFn: async () => {
      if (!enterpriseId) return { success: true, data: undefined }
      return getEnterpriseLogs(enterpriseId, {
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        type: filters.type,
        username: filters.username,
        modelName: filters.modelName,
        group: filters.group,
        startTimestamp: toTimestamp(filters.startDate),
        endTimestamp: toTimestamp(filters.endDate, true),
      })
    },
    enabled: Boolean(enterpriseId),
    placeholderData: (previousData) => previousData,
  })

  const logs = query.data?.data?.items ?? []
  const columns = useMemo<ColumnDef<EnterpriseAuditLog>[]>(
    () => [
      {
        accessorKey: 'created_at',
        header: t('Time'),
        cell: ({ row }) => (
          <span className='whitespace-nowrap'>
            {formatTimestamp(row.original.created_at)}
          </span>
        ),
      },
      {
        accessorKey: 'username',
        header: t('Member'),
        cell: ({ row }) => (
          <div className='min-w-0'>
            <p className='truncate font-medium'>
              {row.original.username || '-'}
            </p>
            <p className='text-muted-foreground mt-0.5 font-mono text-xs'>
              ID {row.original.user_id}
            </p>
          </div>
        ),
      },
      {
        accessorKey: 'type',
        header: t('Event'),
        cell: ({ row }) => (
          <StatusBadge
            label={t(getLogTypeLabel(row.original.type))}
            variant={getLogTypeVariant(row.original.type)}
            copyable={false}
          />
        ),
      },
      {
        accessorKey: 'model_name',
        header: t('Model'),
        cell: ({ row }) => (
          <span className='max-w-[220px] truncate font-mono text-xs'>
            {row.original.model_name || '-'}
          </span>
        ),
      },
      {
        accessorKey: 'group',
        header: t('Group'),
        cell: ({ row }) => row.original.group || '-',
      },
      {
        id: 'tokens',
        header: t('Tokens'),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(
              row.original.prompt_tokens + row.original.completion_tokens
            )}
          </span>
        ),
      },
      {
        accessorKey: 'quota',
        header: t('Quota'),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(row.original.quota)}
          </span>
        ),
      },
      {
        accessorKey: 'request_id',
        header: t('Request ID'),
        cell: ({ row }) => (
          <span className='text-muted-foreground max-w-[150px] truncate font-mono text-xs'>
            {row.original.request_id || '-'}
          </span>
        ),
      },
    ],
    [t]
  )

  const { table } = useDataTable({
    data: logs,
    columns,
    pagination,
    onPaginationChange: setPagination,
    manualPagination: true,
    totalCount: query.data?.data?.total ?? 0,
    enableRowSelection: false,
  })

  function applyFilters() {
    setFilters({
      ...draftFilters,
      username: draftFilters.username.trim(),
      modelName: draftFilters.modelName.trim(),
      group: draftFilters.group.trim(),
    })
    setPagination((previous) => ({ ...previous, pageIndex: 0 }))
  }

  function resetFilters() {
    setDraftFilters(EMPTY_FILTERS)
    setFilters(EMPTY_FILTERS)
    setPagination((previous) => ({ ...previous, pageIndex: 0 }))
  }

  async function exportLogs() {
    if (!enterpriseId || isExporting) return
    setIsExporting(true)
    try {
      const pageSize = 100
      const exportedLogs: EnterpriseAuditLog[] = []
      let page = 1
      let total = 0
      do {
        const response = await getEnterpriseLogs(enterpriseId, {
          page,
          pageSize,
          type: filters.type,
          username: filters.username,
          modelName: filters.modelName,
          group: filters.group,
          startTimestamp: toTimestamp(filters.startDate),
          endTimestamp: toTimestamp(filters.endDate, true),
        })
        if (!response.success) {
          throw new Error(
            response.message || t('Unable to export enterprise audit logs')
          )
        }
        const items = response.data?.items ?? []
        total = response.data?.total ?? items.length
        exportedLogs.push(...items)
        if (items.length < pageSize) break
        page += 1
      } while (exportedLogs.length < total && page <= 10)

      downloadCsv(
        `enterprise-audit-${new Date().toISOString().slice(0, 10)}.csv`,
        [
          [
            t('Time'),
            t('Member'),
            t('Event'),
            t('Model'),
            t('Group'),
            t('Tokens'),
            t('Quota'),
            t('Request ID'),
          ],
          ...exportedLogs.map((log) => [
            formatTimestamp(log.created_at),
            log.username || '-',
            t(getLogTypeLabel(log.type)),
            log.model_name || '-',
            log.group || '-',
            log.prompt_tokens + log.completion_tokens,
            log.quota,
            log.request_id || '-',
          ]),
        ]
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Unable to export enterprise audit logs')
      )
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <EnterpriseShell
      section='audit'
      title={t('Audit logs')}
      description={t(
        'Review enterprise activity with member-level filters. Secret keys, IP addresses and request payloads are not exposed here.'
      )}
    >
      {query.isError ? (
        <ErrorState
          title={t('Unable to load enterprise audit logs')}
          description={t('Please try again later.')}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <TitledCard
          title={t('Enterprise activity')}
          description={t(
            'Usage, access and management events for this enterprise.'
          )}
          icon={<ClipboardList className='size-4' aria-hidden='true' />}
          action={
            <Button
              variant='outline'
              size='sm'
              onClick={() => void exportLogs()}
              disabled={isExporting || query.isLoading || !enterpriseId}
            >
              <Download aria-hidden='true' />
              {t('Export CSV')}
            </Button>
          }
        >
          <DataTablePage
            table={table}
            columns={columns}
            isLoading={query.isLoading}
            isFetching={query.isFetching}
            fixedHeight={false}
            paginationInFooter={false}
            emptyTitle={t('No audit logs found')}
            emptyDescription={t(
              'No enterprise activity matches the current filters.'
            )}
            toolbar={
              <form
                className='grid gap-2 rounded-lg border p-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7'
                onSubmit={(event) => {
                  event.preventDefault()
                  applyFilters()
                }}
              >
                <Select
                  items={LOG_TYPES.map((item) => ({
                    value: String(item.value),
                    label: t(item.label),
                  }))}
                  value={String(draftFilters.type)}
                  onValueChange={(value) =>
                    setDraftFilters((previous) => ({
                      ...previous,
                      type: Number(value),
                    }))
                  }
                >
                  <SelectTrigger aria-label={t('Event type')}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    {LOG_TYPES.map((item) => (
                      <SelectItem key={item.value} value={String(item.value)}>
                        {t(item.label)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  value={draftFilters.username}
                  onChange={(event) =>
                    setDraftFilters((previous) => ({
                      ...previous,
                      username: event.target.value,
                    }))
                  }
                  placeholder={t('Filter by member')}
                  aria-label={t('Filter by member')}
                />
                <Input
                  value={draftFilters.modelName}
                  onChange={(event) =>
                    setDraftFilters((previous) => ({
                      ...previous,
                      modelName: event.target.value,
                    }))
                  }
                  placeholder={t('Filter by model')}
                  aria-label={t('Filter by model')}
                />
                <Input
                  value={draftFilters.group}
                  onChange={(event) =>
                    setDraftFilters((previous) => ({
                      ...previous,
                      group: event.target.value,
                    }))
                  }
                  placeholder={t('Filter by group')}
                  aria-label={t('Filter by group')}
                />
                <Input
                  type='date'
                  value={draftFilters.startDate}
                  onChange={(event) =>
                    setDraftFilters((previous) => ({
                      ...previous,
                      startDate: event.target.value,
                    }))
                  }
                  aria-label={t('Start date')}
                />
                <Input
                  type='date'
                  value={draftFilters.endDate}
                  onChange={(event) =>
                    setDraftFilters((previous) => ({
                      ...previous,
                      endDate: event.target.value,
                    }))
                  }
                  aria-label={t('End date')}
                />
                <div className='flex gap-2'>
                  <Button type='submit' className='min-w-0 flex-1'>
                    <Search aria-hidden='true' />
                    {t('Apply')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    onClick={resetFilters}
                    aria-label={t('Reset filters')}
                  >
                    <RotateCcw aria-hidden='true' />
                  </Button>
                </div>
              </form>
            }
            tableClassName='[&_[data-slot=table]]:text-[13px]'
          />
        </TitledCard>
      )}
    </EnterpriseShell>
  )
}

function toTimestamp(value: string, endOfDay = false) {
  if (!value) return undefined
  const date = new Date(`${value}T${endOfDay ? '23:59:59' : '00:00:00'}`)
  const timestamp = Math.floor(date.getTime() / 1000)
  return Number.isFinite(timestamp) ? timestamp : undefined
}

function getLogTypeLabel(type: number) {
  return LOG_TYPES.find((item) => item.value === type)?.label ?? 'Unknown'
}

function getLogTypeVariant(type: number): StatusVariant {
  switch (type) {
    case 1:
      return 'cyan'
    case 2:
      return 'success'
    case 3:
      return 'warning'
    case 4:
      return 'purple'
    case 5:
      return 'danger'
    case 6:
      return 'info'
    case 7:
      return 'teal'
    default:
      return 'neutral'
  }
}
