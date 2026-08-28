/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import {
  BarChart3,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  Clock3,
  FileText,
  RefreshCw,
  Search,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import {
  fetchSearchUsageLogs,
  fetchSearchUsageStats,
  type SearchUsageLog,
} from './api'
import { SearchShell } from './components/search-shell'

const LOGS_PAGE_SIZE = 20
const RANGE_OPTIONS = [1, 7, 30] as const

export function SearchLogsPage() {
  const { t } = useTranslation()
  const [range, setRange] = useState<(typeof RANGE_OPTIONS)[number]>(30)
  const [queryInput, setQueryInput] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)

  useEffect(() => {
    const timeoutId = setTimeout(() => setQuery(queryInput.trim()), 300)
    return () => clearTimeout(timeoutId)
  }, [queryInput])

  useEffect(() => setPage(1), [query, range])

  const requestParams = {
    page,
    page_size: LOGS_PAGE_SIZE,
    range,
    query: query || undefined,
  }
  const logsQuery = useQuery({
    queryKey: ['search-usage-logs', requestParams],
    queryFn: () => fetchSearchUsageLogs(requestParams),
    placeholderData: (previousData) => previousData,
  })
  const statsQuery = useQuery({
    queryKey: ['search-usage-stats', range, query],
    queryFn: () =>
      fetchSearchUsageStats({
        range,
        query: query || undefined,
      }),
  })

  const logs = logsQuery.data?.items || []
  const total = logsQuery.data?.total || 0
  const totalPages = Math.max(1, Math.ceil(total / LOGS_PAGE_SIZE))
  const stats = statsQuery.data
  const isFetching = logsQuery.isFetching || statsQuery.isFetching

  let content: ReactNode
  if (logsQuery.isLoading) {
    content = <LogsSkeleton />
  } else if (logsQuery.isError) {
    content = (
      <Empty className='min-h-56 rounded-none border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <CircleAlert aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('Failed to load vSearch logs')}</EmptyTitle>
          <EmptyDescription>
            {t('Check your connection and try again.')}
          </EmptyDescription>
        </EmptyHeader>
        <Button variant='outline' onClick={() => void logsQuery.refetch()}>
          {t('Retry')}
        </Button>
      </Empty>
    )
  } else if (logs.length === 0) {
    content = (
      <Empty className='min-h-56 rounded-none border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <FileText aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('No vSearch logs')}</EmptyTitle>
          <EmptyDescription>
            {t('No requests match the selected range and search term.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <>
        <div className='hidden overflow-x-auto sm:block'>
          <LogTable logs={logs} t={t} />
        </div>
        <div className='divide-y sm:hidden'>
          {logs.map((log) => (
            <LogMobileCard key={log.id} log={log} t={t} />
          ))}
        </div>
      </>
    )
  }

  return (
    <SearchShell
      title={t('vSearch logs')}
      description={t(
        'Review only your vSearch requests, response time, status, and vSearch key usage.'
      )}
      action={
        <Button
          variant='outline'
          onClick={() => {
            void logsQuery.refetch()
            void statsQuery.refetch()
          }}
          disabled={isFetching}
        >
          <RefreshCw
            data-icon='inline-start'
            className={isFetching ? 'animate-spin' : undefined}
            aria-hidden='true'
          />
          {t('Refresh')}
        </Button>
      }
    >
      <Card>
        <CardContent className='flex flex-col gap-4 p-4 sm:p-5 lg:flex-row lg:items-end lg:justify-between'>
          <div>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('Date range')}
            </span>
            <ToggleGroup
              className='mt-2'
              variant='outline'
              size='sm'
              spacing={1}
              value={[String(range)]}
              onValueChange={(value) => {
                const nextRange = Number(value[0])
                if (RANGE_OPTIONS.includes(nextRange as 1 | 7 | 30)) {
                  setRange(nextRange as 1 | 7 | 30)
                }
              }}
            >
              {RANGE_OPTIONS.map((days) => (
                <ToggleGroupItem key={days} value={String(days)}>
                  {days === 1
                    ? t('24 hours')
                    : t('{{count}} days', { count: days })}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <label
            className='relative w-full lg:max-w-sm'
            htmlFor='search-logs-query'
          >
            <span className='text-muted-foreground text-xs font-medium'>
              {t('Search logs')}
            </span>
            <Search
              className='text-muted-foreground absolute bottom-2.5 left-3 size-4'
              aria-hidden='true'
            />
            <Input
              id='search-logs-query'
              className='mt-2 h-9 pl-9'
              value={queryInput}
              onChange={(event) => setQueryInput(event.target.value)}
              placeholder={t('Search service, endpoint, or vSearch key')}
            />
          </label>
        </CardContent>
      </Card>

      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <LogMetric
          icon={FileText}
          label={t('Total requests')}
          value={formatMetric(statsQuery, stats?.total_requests)}
        />
        <LogMetric
          icon={CheckCircle2}
          label={t('Success rate')}
          value={
            statsQuery.isLoading || !stats
              ? '—'
              : `${stats.success_rate.toFixed(1)}%`
          }
        />
        <LogMetric
          icon={Clock3}
          label={t('Average response time')}
          value={
            statsQuery.isLoading || !stats
              ? '—'
              : formatLatency(stats.average_latency_ms)
          }
        />
        <LogMetric
          icon={BarChart3}
          label={t('Failed requests')}
          value={formatMetric(statsQuery, stats?.error_requests)}
        />
      </div>

      <Card>
        <CardContent className='p-0'>
          <div className='border-b p-4 sm:p-5'>
            <h2 className='font-semibold'>{t('Request details')}</h2>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('{{count}} matching requests', { count: total })}
            </p>
          </div>
          {content}
          {!logsQuery.isLoading && !logsQuery.isError && total > 0 && (
            <div className='flex items-center justify-between gap-3 border-t p-3 sm:p-4'>
              <span className='text-muted-foreground text-xs tabular-nums'>
                {t('Page {{page}} of {{total}}', { page, total: totalPages })}
              </span>
              <div className='flex gap-2'>
                <Button
                  variant='outline'
                  size='icon-sm'
                  aria-label={t('Previous page')}
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                  disabled={page <= 1 || logsQuery.isFetching}
                >
                  <ChevronLeft aria-hidden='true' />
                </Button>
                <Button
                  variant='outline'
                  size='icon-sm'
                  aria-label={t('Next page')}
                  onClick={() =>
                    setPage((current) => Math.min(totalPages, current + 1))
                  }
                  disabled={page >= totalPages || logsQuery.isFetching}
                >
                  <ChevronRight aria-hidden='true' />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </SearchShell>
  )
}

function LogMetric(props: {
  icon: typeof FileText
  label: string
  value: string
}) {
  const Icon = props.icon
  return (
    <Card>
      <CardContent className='p-4'>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-xs font-medium'>
            {props.label}
          </span>
          <span className='bg-primary/10 text-primary flex size-8 items-center justify-center rounded-lg'>
            <Icon className='size-4' aria-hidden='true' />
          </span>
        </div>
        <div className='mt-4 font-mono text-2xl font-semibold tabular-nums'>
          {props.value}
        </div>
      </CardContent>
    </Card>
  )
}

function LogTable(props: {
  logs: SearchUsageLog[]
  t: (key: string) => string
}) {
  return (
    <table className='w-full min-w-[760px] text-sm'>
      <thead className='bg-muted/50 text-muted-foreground text-left text-xs'>
        <tr>
          <th className='px-5 py-3 font-medium'>{props.t('Time')}</th>
          <th className='px-5 py-3 font-medium'>{props.t('Service')}</th>
          <th className='px-5 py-3 font-medium'>{props.t('Endpoint')}</th>
          <th className='px-5 py-3 font-medium'>{props.t('AgentKey')}</th>
          <th className='px-5 py-3 font-medium'>{props.t('Status')}</th>
          <th className='px-5 py-3 text-right font-medium'>
            {props.t('Latency')}
          </th>
        </tr>
      </thead>
      <tbody className='divide-y'>
        {props.logs.map((log) => (
          <tr key={log.id}>
            <td className='text-muted-foreground px-5 py-3 font-mono text-xs whitespace-nowrap'>
              {formatTimestamp(log.created_at)}
            </td>
            <td className='max-w-48 truncate px-5 py-3 font-medium'>
              {log.service || '—'}
            </td>
            <td className='text-muted-foreground max-w-56 truncate px-5 py-3 font-mono text-xs'>
              {log.endpoint || '—'}
            </td>
            <td className='text-muted-foreground max-w-40 truncate px-5 py-3'>
              {log.agent_key_name || '—'}
            </td>
            <td className='px-5 py-3'>
              <LogStatus status={log.status} t={props.t} />
            </td>
            <td className='text-muted-foreground px-5 py-3 text-right font-mono text-xs'>
              {formatLatency(log.latency_ms)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function LogMobileCard(props: {
  log: SearchUsageLog
  t: (key: string) => string
}) {
  const { log, t } = props
  return (
    <article className='space-y-3 p-4'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate font-medium'>{log.service || '—'}</p>
          <p className='text-muted-foreground mt-1 truncate font-mono text-xs'>
            {log.endpoint || '—'}
          </p>
        </div>
        <LogStatus status={log.status} t={t} />
      </div>
      <div className='text-muted-foreground flex flex-wrap justify-between gap-2 text-xs'>
        <span>{formatTimestamp(log.created_at)}</span>
        <span className='font-mono'>{formatLatency(log.latency_ms)}</span>
      </div>
      {log.agent_key_name && (
        <p className='text-muted-foreground text-xs'>{log.agent_key_name}</p>
      )}
    </article>
  )
}

function LogStatus(props: { status: string; t: (key: string) => string }) {
  const status = props.status.toLocaleLowerCase()
  if (status === 'pending') {
    return (
      <StatusBadge
        label={props.t('Pending')}
        variant='warning'
        copyable={false}
      />
    )
  }
  if (status === 'indeterminate') {
    return (
      <StatusBadge
        label={props.t('Indeterminate')}
        variant='neutral'
        copyable={false}
      />
    )
  }
  const success = status === 'success'
  return (
    <StatusBadge
      label={success ? props.t('Success') : props.t('Error')}
      variant={success ? 'success' : 'danger'}
      copyable={false}
    />
  )
}

function LogsSkeleton() {
  return (
    <div className='space-y-4 p-5' aria-hidden='true'>
      {Array.from({ length: 5 }, (_, index) => (
        <Skeleton key={index} className='h-8 w-full' />
      ))}
    </div>
  )
}

function formatMetric(
  query: { isLoading: boolean; isError: boolean },
  value: number | undefined
) {
  return query.isLoading || query.isError || value === undefined
    ? '—'
    : value.toLocaleString()
}

function formatLatency(milliseconds: number) {
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return '—'
  return milliseconds >= 1000
    ? `${(milliseconds / 1000).toFixed(2)}s`
    : `${Math.round(milliseconds)}ms`
}

function formatTimestamp(value: number | string) {
  let timestamp = Date.parse(String(value))
  if (typeof value === 'number') {
    timestamp = value < 10_000_000_000 ? value * 1000 : value
  }
  if (!Number.isFinite(timestamp)) return '—'
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(timestamp)
}
