/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BarChart3,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  CircleDollarSign,
  Download,
  FileText,
  RefreshCw,
  Search,
  TrendingUp,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import {
  exportAdminSearchUsageLogs,
  fetchAdminSearchUsageLogs,
  fetchAdminSearchUsageStats,
  reconcileAdminSearchUsage,
  type SearchUsageLog,
  type SearchUsageReconciliationAction,
} from '../api'
import {
  usageReconciliationSchema,
  type UsageReconciliationFormValues,
} from '../lib/usage-reconciliation-form'
import { formatCnyMoney } from '../money'
import { SearchAdminShell } from './search-admin-shell'

const LOGS_PAGE_SIZE = 20
const RANGE_OPTIONS = [7, 30, 90] as const

export function SearchAdminUsageLogsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [range, setRange] = useState<(typeof RANGE_OPTIONS)[number]>(30)
  const [queryInput, setQueryInput] = useState('')
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('all')
  const [page, setPage] = useState(1)
  const [reconciliation, setReconciliation] = useState<{
    log: SearchUsageLog
    action: SearchUsageReconciliationAction
  } | null>(null)

  useEffect(() => {
    const timeoutId = setTimeout(() => setQuery(queryInput.trim()), 300)
    return () => clearTimeout(timeoutId)
  }, [queryInput])
  useEffect(() => setPage(1), [query, range, status])

  const requestParams = {
    page,
    page_size: LOGS_PAGE_SIZE,
    range,
    query: query || undefined,
    status: status === 'all' ? undefined : status,
  }
  const statParams = {
    range,
    query: query || undefined,
    status: status === 'all' ? undefined : status,
  }
  const logsQuery = useQuery({
    queryKey: ['search-admin-usage-logs', requestParams],
    queryFn: () => fetchAdminSearchUsageLogs(requestParams),
    placeholderData: (previousData) => previousData,
  })
  const statsQuery = useQuery({
    queryKey: ['search-admin-usage-stats', statParams],
    queryFn: () => fetchAdminSearchUsageStats(statParams),
  })
  const exportMutation = useMutation({
    mutationFn: () => exportAdminSearchUsageLogs(statParams),
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `vsearch-usage-${range}-days.csv`
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success(t('CSV export ready'))
    },
    onError: handleServerError,
  })
  const reconciliationMutation = useMutation({
    mutationFn: (input: {
      id: number
      action: SearchUsageReconciliationAction
      note: string
    }) =>
      reconcileAdminSearchUsage(input.id, {
        action: input.action,
        note: input.note,
      }),
    onSuccess: () => {
      setReconciliation(null)
      toast.success(t('vSearch request reconciled'))
    },
    onSettled: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['search-admin-usage-logs'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['search-admin-usage-stats'],
        }),
      ])
    },
    onError: handleServerError,
  })

  const openReconciliation = (
    log: SearchUsageLog,
    action: SearchUsageReconciliationAction
  ) => {
    setReconciliation({ log, action })
  }

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
          <EmptyTitle>{t('Failed to load vSearch usage logs')}</EmptyTitle>
          <EmptyDescription>
            {t('Check administrator permissions and try again.')}
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
          <EmptyTitle>{t('No vSearch usage logs')}</EmptyTitle>
          <EmptyDescription>
            {t('No records match the selected range and filters.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <>
        <div
          className='hidden overflow-x-auto md:block'
          data-testid='desktop-usage-logs'
        >
          <AdminLogTable
            logs={logs}
            onReconcile={openReconciliation}
            isReconciling={reconciliationMutation.isPending}
          />
        </div>
        <div className='divide-y md:hidden' data-testid='mobile-usage-logs'>
          {logs.map((log) => (
            <AdminLogCard
              key={log.id}
              log={log}
              onReconcile={openReconciliation}
              isReconciling={reconciliationMutation.isPending}
            />
          ))}
        </div>
      </>
    )
  }

  return (
    <SearchAdminShell
      title={t('vSearch log management')}
      description={t(
        'Track every vSearch request, upstream cost, user revenue, and gross profit in one platform-wide view.'
      )}
      action={
        <div className='flex flex-wrap gap-2'>
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
          <Button
            variant='outline'
            onClick={() => exportMutation.mutate()}
            disabled={exportMutation.isPending}
          >
            <Download data-icon='inline-start' aria-hidden='true' />
            {exportMutation.isPending ? t('Exporting…') : t('Export CSV')}
          </Button>
        </div>
      }
    >
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <UsageMetric
          icon={BarChart3}
          label={t('Requests')}
          value={formatNumberMetric(statsQuery, stats?.total_requests)}
        />
        <UsageMetric
          icon={CircleDollarSign}
          label={t('Upstream cost')}
          value={formatMoneyMetric(statsQuery, {
            micros: stats?.upstream_cost_micros,
            amount: stats?.upstream_cost,
          })}
        />
        <UsageMetric
          icon={TrendingUp}
          label={t('User revenue')}
          value={formatMoneyMetric(statsQuery, {
            micros: stats?.revenue_micros,
            amount: stats?.revenue,
          })}
        />
        <UsageMetric
          icon={TrendingUp}
          label={t('Gross profit')}
          value={formatMoneyMetric(statsQuery, {
            micros: stats?.profit_micros,
            amount: stats?.profit,
          })}
          accent
        />
      </div>

      <Card>
        <CardContent className='space-y-4 p-4 sm:p-5'>
          <div className='flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between'>
            <label
              className='relative w-full xl:max-w-lg'
              htmlFor='search-admin-logs-query'
            >
              <span className='text-muted-foreground text-xs font-medium'>
                {t('Search usage logs')}
              </span>
              <Search
                className='text-muted-foreground absolute bottom-2.5 left-3 size-4'
                aria-hidden='true'
              />
              <Input
                id='search-admin-logs-query'
                className='mt-2 h-9 pl-9'
                value={queryInput}
                onChange={(event) => setQueryInput(event.target.value)}
                placeholder={t(
                  'Search user, service, endpoint, or vSearch key'
                )}
              />
            </label>
            <div className='flex flex-col gap-4 sm:flex-row'>
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
                    if (RANGE_OPTIONS.includes(nextRange as 7 | 30 | 90)) {
                      setRange(nextRange as 7 | 30 | 90)
                    }
                  }}
                >
                  {RANGE_OPTIONS.map((days) => (
                    <ToggleGroupItem key={days} value={String(days)}>
                      {t('{{count}} days', { count: days })}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              </div>
              <div>
                <span className='text-muted-foreground text-xs font-medium'>
                  {t('Status')}
                </span>
                <ToggleGroup
                  className='mt-2'
                  variant='outline'
                  size='sm'
                  spacing={1}
                  value={[status]}
                  onValueChange={(value) => value[0] && setStatus(value[0])}
                >
                  <ToggleGroupItem value='all'>{t('All')}</ToggleGroupItem>
                  <ToggleGroupItem value='success'>
                    {t('Success')}
                  </ToggleGroupItem>
                  <ToggleGroupItem value='error'>{t('Error')}</ToggleGroupItem>
                  <ToggleGroupItem value='pending'>
                    {t('Pending')}
                  </ToggleGroupItem>
                  <ToggleGroupItem value='indeterminate'>
                    {t('Indeterminate')}
                  </ToggleGroupItem>
                </ToggleGroup>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

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

      <ReconciliationDialog
        reconciliation={reconciliation}
        isPending={reconciliationMutation.isPending}
        onOpenChange={(open) => {
          if (!open && !reconciliationMutation.isPending) {
            setReconciliation(null)
          }
        }}
        onConfirm={(note) => {
          if (!reconciliation) return
          reconciliationMutation.mutate({
            id: Number(reconciliation.log.id),
            action: reconciliation.action,
            note,
          })
        }}
      />
    </SearchAdminShell>
  )
}

function UsageMetric(props: {
  icon: typeof BarChart3
  label: string
  value: string
  accent?: boolean
}) {
  const Icon = props.icon
  return (
    <Card
      className={props.accent ? 'border-primary/30 bg-primary/5' : undefined}
    >
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

function AdminLogTable(props: {
  logs: SearchUsageLog[]
  onReconcile: (
    log: SearchUsageLog,
    action: SearchUsageReconciliationAction
  ) => void
  isReconciling: boolean
}) {
  const { t } = useTranslation()
  return (
    <table className='w-full min-w-[1540px] text-sm'>
      <thead className='bg-muted/50 text-muted-foreground text-left text-xs'>
        <tr>
          <th className='px-5 py-3 font-medium'>{t('Time')}</th>
          <th className='px-5 py-3 font-medium'>{t('Enterprise / User')}</th>
          <th className='px-5 py-3 font-medium'>{t('Service')}</th>
          <th className='px-5 py-3 font-medium'>{t('Endpoint')}</th>
          <th className='px-5 py-3 font-medium'>{t('Request ID')}</th>
          <th className='px-5 py-3 font-medium'>{t('Error code')}</th>
          <th className='px-5 py-3 font-medium'>{t('Upstream account')}</th>
          <th className='px-5 py-3 font-medium'>{t('Status')}</th>
          <th className='px-5 py-3 text-right font-medium'>{t('Latency')}</th>
          <th className='px-5 py-3 text-right font-medium'>
            {t('Upstream cost')}
          </th>
          <th className='px-5 py-3 text-right font-medium'>
            {t('User revenue')}
          </th>
          <th className='px-5 py-3 text-right font-medium'>
            {t('Gross profit')}
          </th>
          <th className='px-5 py-3 text-right font-medium'>{t('Actions')}</th>
        </tr>
      </thead>
      <tbody className='divide-y'>
        {props.logs.map((log) => (
          <tr key={log.id}>
            <td className='text-muted-foreground px-5 py-3 font-mono text-xs whitespace-nowrap'>
              {formatTimestamp(log.created_at)}
            </td>
            <td className='min-w-44 px-5 py-3'>
              <div className='font-medium'>{log.enterprise_name || '—'}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {log.user_name || '—'}
              </div>
            </td>
            <td className='max-w-48 truncate px-5 py-3 font-medium'>
              {log.service || '—'}
            </td>
            <td className='text-muted-foreground max-w-56 truncate px-5 py-3 font-mono text-xs'>
              {log.endpoint || '—'}
            </td>
            <td className='text-muted-foreground max-w-48 truncate px-5 py-3 font-mono text-xs'>
              {log.request_id || '—'}
            </td>
            <td className='text-muted-foreground max-w-44 truncate px-5 py-3 font-mono text-xs'>
              {log.error_code || '—'}
            </td>
            <td className='text-muted-foreground max-w-48 truncate px-5 py-3 text-xs'>
              {log.account || '—'}
            </td>
            <td className='px-5 py-3'>
              <LogStatus status={log.status} />
            </td>
            <td className='text-muted-foreground px-5 py-3 text-right font-mono text-xs'>
              {formatLatency(log.latency_ms)}
            </td>
            <td className='px-5 py-3 text-right font-mono text-xs'>
              {formatUsageMoney(log, {
                micros: log.upstream_cost_micros,
                amount: log.upstream_cost,
              })}
            </td>
            <td className='px-5 py-3 text-right font-mono text-xs'>
              {formatUsageMoney(log, {
                micros: log.charge_micros,
                amount: log.charge,
              })}
            </td>
            <td className='px-5 py-3 text-right font-mono text-xs'>
              {formatUsageMoney(log, {
                micros: log.profit_micros,
                amount: log.profit,
              })}
            </td>
            <td className='px-5 py-3'>
              <AdminLogActions
                log={log}
                onReconcile={props.onReconcile}
                disabled={props.isReconciling}
                justifyEnd
              />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function AdminLogCard(props: {
  log: SearchUsageLog
  onReconcile: (
    log: SearchUsageLog,
    action: SearchUsageReconciliationAction
  ) => void
  isReconciling: boolean
}) {
  const { t } = useTranslation()
  return (
    <article className='space-y-3 p-4'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate font-medium'>{props.log.service || '—'}</p>
          <p className='text-muted-foreground mt-1 truncate text-xs'>
            {props.log.enterprise_name || '—'} · {props.log.user_name || '—'}
          </p>
        </div>
        <LogStatus status={props.log.status} />
      </div>
      <div className='text-muted-foreground grid grid-cols-2 gap-2 text-xs'>
        <span>{formatTimestamp(props.log.created_at)}</span>
        <span className='text-right'>
          {formatLatency(props.log.latency_ms)}
        </span>
        <span>{t('Endpoint')}</span>
        <span className='truncate text-right font-mono'>
          {props.log.endpoint || '—'}
        </span>
        <span>{t('Request ID')}</span>
        <span className='truncate text-right font-mono'>
          {props.log.request_id || '—'}
        </span>
        <span>{t('Error code')}</span>
        <span className='truncate text-right font-mono'>
          {props.log.error_code || '—'}
        </span>
        <span>{t('Upstream account')}</span>
        <span className='truncate text-right'>{props.log.account || '—'}</span>
        <span>{t('Upstream cost')}</span>
        <span className='text-right'>
          {formatUsageMoney(props.log, {
            micros: props.log.upstream_cost_micros,
            amount: props.log.upstream_cost,
          })}
        </span>
        <span>{t('User revenue')}</span>
        <span className='text-right'>
          {formatUsageMoney(props.log, {
            micros: props.log.charge_micros,
            amount: props.log.charge,
          })}
        </span>
        <span>{t('Gross profit')}</span>
        <span className='text-right'>
          {formatUsageMoney(props.log, {
            micros: props.log.profit_micros,
            amount: props.log.profit,
          })}
        </span>
      </div>
      <AdminLogActions
        log={props.log}
        onReconcile={props.onReconcile}
        disabled={props.isReconciling}
      />
    </article>
  )
}

function AdminLogActions(props: {
  log: SearchUsageLog
  onReconcile: (
    log: SearchUsageLog,
    action: SearchUsageReconciliationAction
  ) => void
  disabled: boolean
  justifyEnd?: boolean
}) {
  const { t } = useTranslation()
  const actions = getReconciliationActions(props.log)
  const terminalLabelKey = getTerminalReconciliationLabelKey(props.log)
  const terminalLabel = terminalLabelKey ? t(terminalLabelKey) : ''
  if (actions.length === 0 && !terminalLabel) return null

  return (
    <div
      className={cn('flex flex-wrap gap-2', props.justifyEnd && 'justify-end')}
    >
      {terminalLabel && (
        <span
          className='text-muted-foreground text-xs'
          title={props.log.reconciliation_note}
        >
          {terminalLabel}
        </span>
      )}
      {actions.map((action) => {
        const retry = props.log.reconciliation_action === action
        let label = t('Refund reservation')
        if (action === 'settle') {
          label = retry ? t('Retry settlement') : t('Settle charge')
        } else if (retry) {
          label = t('Retry refund')
        }
        return (
          <Button
            key={action}
            variant={action === 'settle' ? 'outline' : 'destructive'}
            size='xs'
            disabled={props.disabled}
            onClick={() => props.onReconcile(props.log, action)}
          >
            {label}
          </Button>
        )
      })}
    </div>
  )
}

function getTerminalReconciliationLabelKey(log: SearchUsageLog) {
  const billingState = log.billing_state?.toLocaleLowerCase()
  if (log.reconciliation_action === 'settle' && billingState === 'committed') {
    return 'Settled by admin'
  }
  if (log.reconciliation_action === 'refund' && billingState === 'refunded') {
    return 'Refunded by admin'
  }
  return ''
}

function ReconciliationDialog(props: {
  reconciliation: {
    log: SearchUsageLog
    action: SearchUsageReconciliationAction
  } | null
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (note: string) => void
}) {
  const { t } = useTranslation()
  const form = useForm<UsageReconciliationFormValues>({
    resolver: zodResolver(usageReconciliationSchema),
    defaultValues: { note: '' },
  })
  const isSettlement = props.reconciliation?.action === 'settle'
  const noteError = form.formState.errors.note
  const noteDescriptionID = 'search-usage-reconciliation-note-description'
  const noteErrorID = 'search-usage-reconciliation-note-error'

  useEffect(() => {
    const existingNote =
      props.reconciliation &&
      props.reconciliation.log.reconciliation_action ===
        props.reconciliation.action
        ? props.reconciliation.log.reconciliation_note || ''
        : ''
    form.reset({ note: existingNote })
  }, [form, props.reconciliation])

  return (
    <AlertDialog
      open={props.reconciliation !== null}
      onOpenChange={props.onOpenChange}
    >
      <AlertDialogContent>
        <form
          className='space-y-4'
          noValidate
          onSubmit={form.handleSubmit((values) =>
            props.onConfirm(values.note.trim())
          )}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>
              {isSettlement
                ? t('Settle this vSearch request?')
                : t('Refund this vSearch reservation?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {isSettlement
                ? t(
                    'Confirm the upstream request completed and charge the reserved amount.'
                  )
                : t(
                    'Confirm the upstream request did not complete and return the reserved amount.'
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <Field data-invalid={Boolean(noteError)}>
            <FieldLabel htmlFor='search-usage-reconciliation-note'>
              {t('Operator note')}
            </FieldLabel>
            <Textarea
              id='search-usage-reconciliation-note'
              maxLength={255}
              disabled={props.isPending}
              placeholder={t('Describe the evidence used for this decision.')}
              aria-invalid={Boolean(noteError)}
              aria-describedby={noteError ? noteErrorID : noteDescriptionID}
              {...form.register('note')}
            />
            <FieldDescription id={noteDescriptionID}>
              {t('An audit note is required for this financial action.')}
            </FieldDescription>
            <FieldError
              id={noteErrorID}
              errors={
                noteError
                  ? [{ message: t(String(noteError.message)) }]
                  : undefined
              }
            />
          </Field>
          <AlertDialogFooter>
            <AlertDialogCancel type='button' disabled={props.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <Button
              type='submit'
              variant={isSettlement ? 'default' : 'destructive'}
              disabled={props.isPending}
            >
              {isSettlement ? t('Confirm settlement') : t('Confirm refund')}
            </Button>
          </AlertDialogFooter>
        </form>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function getReconciliationActions(
  log: SearchUsageLog
): SearchUsageReconciliationAction[] {
  if (
    log.status.toLocaleLowerCase() !== 'indeterminate' ||
    log.execution_phase?.toLocaleLowerCase() !== 'dispatching'
  ) {
    return []
  }

  const billingState = log.billing_state?.toLocaleLowerCase()
  if (!log.reconciliation_action) {
    return billingState === 'reserved' ? ['settle', 'refund'] : []
  }
  if (
    log.reconciliation_action === 'settle' &&
    ['reserved', 'commit_pending', 'log_pending', 'log_writing'].includes(
      billingState || ''
    )
  ) {
    return ['settle']
  }
  if (
    log.reconciliation_action === 'refund' &&
    ['reserved', 'refund_pending', 'refund_failed'].includes(billingState || '')
  ) {
    return ['refund']
  }
  return []
}

function LogStatus(props: { status: string }) {
  const { t } = useTranslation()
  const status = props.status.toLocaleLowerCase()
  if (status === 'pending') {
    return (
      <StatusBadge label={t('Pending')} variant='warning' copyable={false} />
    )
  }
  if (status === 'indeterminate') {
    return (
      <StatusBadge
        label={t('Indeterminate')}
        variant='neutral'
        copyable={false}
      />
    )
  }
  const success = status === 'success'
  return (
    <StatusBadge
      label={success ? t('Success') : t('Error')}
      variant={success ? 'success' : 'danger'}
      copyable={false}
    />
  )
}

function LogsSkeleton() {
  return (
    <div className='space-y-4 p-5' aria-hidden='true'>
      {Array.from({ length: 6 }, (_, index) => (
        <Skeleton key={index} className='h-9 w-full' />
      ))}
    </div>
  )
}

function formatNumberMetric(
  query: { isLoading: boolean; isError: boolean },
  value: number | undefined
) {
  return query.isLoading || query.isError || value === undefined
    ? '—'
    : value.toLocaleString()
}

function formatMoneyMetric(
  query: { isLoading: boolean; isError: boolean },
  value: { micros?: number; amount?: number }
) {
  return query.isLoading || query.isError ? '—' : formatCnyMoney(value)
}

function formatUsageMoney(
  log: SearchUsageLog,
  value: { micros?: number; amount?: number }
) {
  const status = log.status.toLocaleLowerCase()
  if (status === 'pending') return '—'
  if (
    status === 'indeterminate' &&
    log.billing_state?.toLocaleLowerCase() !== 'committed'
  ) {
    return '—'
  }
  return formatCnyMoney(value)
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
