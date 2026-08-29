/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BookOpen,
  Building2,
  CircleAlert,
  RefreshCw,
  Search,
  Upload,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { handleServerError } from '@/lib/handle-server-error'

import {
  fetchAdminSearchCatalog,
  fetchSearchCapabilityEnterpriseGrants,
  fetchSearchGrantEnterprises,
  publishAdminSearchCatalog,
  syncAdminSearchCatalog,
  updateAdminSearchCatalogItem,
  updateSearchCapabilityEnterpriseGrants,
  type SearchAdminCatalogItem,
} from '../api'
import {
  formatCnyMoney,
  formatMicrosForInput,
  parseCnyInputToMicros,
  resolveCnyMicros,
} from '../money'
import { SearchAdminShell } from './search-admin-shell'

const ALL_CATEGORIES = 'all'
const EMPTY_CATALOG: SearchAdminCatalogItem[] = []

export function SearchAdminCatalogPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState(ALL_CATEGORIES)
  const [lastSyncedServiceIDs, setLastSyncedServiceIDs] = useState<string[]>([])
  const [publishDialogOpen, setPublishDialogOpen] = useState(false)
  const catalogQuery = useQuery({
    queryKey: ['search-admin-catalog'],
    queryFn: fetchAdminSearchCatalog,
  })
  const syncMutation = useMutation({
    mutationFn: syncAdminSearchCatalog,
    onSuccess: (result) => {
      setLastSyncedServiceIDs(result.synced_service_ids)
      toast.success(
        t('{{count}} capabilities synchronized', { count: result.synced })
      )
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-catalog'],
      })
    },
    onError: handleServerError,
  })
  const publishMutation = useMutation({
    mutationFn: publishAdminSearchCatalog,
    onSuccess: (result) => {
      toast.success(
        t('{{published}} capabilities published; {{skipped}} skipped', {
          published: result.published,
          skipped: result.skipped,
        })
      )
      setLastSyncedServiceIDs([])
      setPublishDialogOpen(false)
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-catalog'],
      })
    },
    onError: handleServerError,
  })
  const updateMutation = useMutation({
    mutationFn: ({
      id,
      patch,
    }: {
      id: string
      patch: { enabled?: boolean; price_micros?: number }
    }) => updateAdminSearchCatalogItem(id, patch),
    onSuccess: () => {
      toast.success(t('Capability configuration saved'))
      void queryClient.invalidateQueries({
        queryKey: ['search-admin-catalog'],
      })
    },
    onError: handleServerError,
  })

  const catalog = catalogQuery.data || EMPTY_CATALOG
  const categories = useMemo(
    () => [...new Set(catalog.map((item) => item.category))].sort(),
    [catalog]
  )
  const filteredCatalog = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    return catalog.filter((item) => {
      if (category !== ALL_CATEGORIES && item.category !== category) {
        return false
      }
      if (!normalizedQuery) return true
      return `${item.name} ${item.description} ${item.category}`
        .toLocaleLowerCase()
        .includes(normalizedQuery)
    })
  }, [catalog, category, query])
  const enabledCount = catalog.filter((item) => item.enabled).length
  const interfaceCount = catalog.reduce(
    (total, item) => total + item.interface_count,
    0
  )
  const configuredPriceCount = catalog.filter(
    (item) =>
      item.price_micros !== undefined ||
      (item.price !== null && item.price !== undefined)
  ).length

  let catalogContent: ReactNode
  if (catalogQuery.isLoading) {
    catalogContent = <CatalogSkeleton />
  } else if (catalogQuery.isError) {
    catalogContent = (
      <Empty className='min-h-56 rounded-none border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <CircleAlert aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('Failed to load capability catalog')}</EmptyTitle>
          <EmptyDescription>
            {t('Check administrator permissions and try again.')}
          </EmptyDescription>
        </EmptyHeader>
        <Button variant='outline' onClick={() => void catalogQuery.refetch()}>
          {t('Retry')}
        </Button>
      </Empty>
    )
  } else if (filteredCatalog.length === 0) {
    catalogContent = (
      <Empty className='min-h-56 rounded-none border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <Search aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('No matching capabilities')}</EmptyTitle>
          <EmptyDescription>
            {t('Try another search term or category.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    catalogContent = (
      <>
        <div className='hidden overflow-x-auto md:block'>
          <CatalogTable
            items={filteredCatalog}
            pendingId={
              updateMutation.isPending ? updateMutation.variables.id : null
            }
            onUpdate={(id, patch) => updateMutation.mutate({ id, patch })}
            t={t}
          />
        </div>
        <div className='divide-y md:hidden'>
          {filteredCatalog.map((item) => (
            <CatalogCard
              key={item.id}
              item={item}
              pending={
                updateMutation.isPending &&
                updateMutation.variables.id === item.id
              }
              onUpdate={(patch) =>
                updateMutation.mutate({ id: item.id, patch })
              }
              t={t}
            />
          ))}
        </div>
      </>
    )
  }

  return (
    <SearchAdminShell
      title={t('vSearch capability management')}
      description={t(
        'Synchronize the upstream tool catalog, control availability, and set the price exposed to users.'
      )}
      action={
        <div className='flex flex-wrap items-center justify-end gap-2'>
          {lastSyncedServiceIDs.length > 0 && (
            <AlertDialog
              open={publishDialogOpen}
              onOpenChange={(open) => {
                if (!publishMutation.isPending) setPublishDialogOpen(open)
              }}
            >
              <AlertDialogTrigger
                render={<Button variant='outline' />}
                disabled={syncMutation.isPending || publishMutation.isPending}
              >
                <Upload data-icon='inline-start' aria-hidden='true' />
                {t('Publish synchronized capabilities')}
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {t('Publish capabilities?')}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {t(
                      'Only the {{count}} capabilities returned by the latest synchronization with valid parameter schemas and healthy routes will be published to all enterprises.',
                      { count: lastSyncedServiceIDs.length }
                    )}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel disabled={publishMutation.isPending}>
                    {t('Cancel')}
                  </AlertDialogCancel>
                  <AlertDialogAction
                    disabled={publishMutation.isPending}
                    onClick={() => {
                      if (lastSyncedServiceIDs.length === 0) return
                      publishMutation.mutate({
                        service_ids: lastSyncedServiceIDs,
                        access_mode: 'all_enterprises',
                      })
                    }}
                  >
                    {publishMutation.isPending
                      ? t('Publishing…')
                      : t('Publish capabilities')}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
          <Button
            onClick={() => {
              setLastSyncedServiceIDs([])
              syncMutation.mutate()
            }}
            disabled={syncMutation.isPending || publishMutation.isPending}
          >
            <RefreshCw
              data-icon='inline-start'
              className={syncMutation.isPending ? 'animate-spin' : undefined}
              aria-hidden='true'
            />
            {syncMutation.isPending ? t('Synchronizing…') : t('Sync catalog')}
          </Button>
        </div>
      }
    >
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <CatalogMetric
          label={t('Catalog capabilities')}
          value={catalogQuery.isLoading ? '—' : catalog.length.toLocaleString()}
        />
        <CatalogMetric
          label={t('Enabled capabilities')}
          value={catalogQuery.isLoading ? '—' : enabledCount.toLocaleString()}
        />
        <CatalogMetric
          label={t('Callable interfaces')}
          value={catalogQuery.isLoading ? '—' : interfaceCount.toLocaleString()}
        />
        <CatalogMetric
          label={t('Configured prices')}
          value={
            catalogQuery.isLoading ? '—' : configuredPriceCount.toLocaleString()
          }
        />
      </div>

      <Card>
        <CardContent className='space-y-4 p-4 sm:p-5'>
          <label
            className='relative block'
            htmlFor='search-admin-catalog-query'
          >
            <Search
              className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2'
              aria-hidden='true'
            />
            <Input
              id='search-admin-catalog-query'
              aria-label={t('Search service catalog')}
              className='h-10 pl-9'
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('Search service catalog')}
            />
          </label>
          <div
            className='flex gap-2 overflow-x-auto pb-1'
            role='group'
            aria-label={t('Catalog categories')}
          >
            <CategoryButton
              active={category === ALL_CATEGORIES}
              label={t('All')}
              onClick={() => setCategory(ALL_CATEGORIES)}
            />
            {categories.map((item) => (
              <CategoryButton
                key={item}
                active={category === item}
                label={t(item)}
                onClick={() => setCategory(item)}
              />
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className='p-0'>
          <div className='flex flex-col gap-2 border-b p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5'>
            <div>
              <h2 className='font-semibold'>{t('Capability catalog')}</h2>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t('{{count}} matching capabilities', {
                  count: filteredCatalog.length,
                })}
              </p>
            </div>
            <StatusBadge
              label={t('Server-managed catalog')}
              icon={BookOpen}
              variant='info'
              copyable={false}
            />
          </div>
          {catalogContent}
        </CardContent>
      </Card>
    </SearchAdminShell>
  )
}

function CatalogTable(props: {
  items: SearchAdminCatalogItem[]
  pendingId: string | null
  onUpdate: (
    id: string,
    patch: { enabled?: boolean; price_micros?: number }
  ) => void
  t: (key: string, options?: Record<string, unknown>) => string
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{props.t('Capability')}</TableHead>
          <TableHead>{props.t('Interfaces')}</TableHead>
          <TableHead>{props.t('Upstream cost')}</TableHead>
          <TableHead>{props.t('User price')}</TableHead>
          <TableHead>{props.t('Enterprise access')}</TableHead>
          <TableHead>{props.t('Availability')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.items.map((item) => (
          <TableRow key={item.id}>
            <TableCell className='min-w-64'>
              <div className='font-medium'>{item.name}</div>
              <p className='text-muted-foreground mt-1 max-w-md text-xs leading-5'>
                {item.description}
              </p>
              <p className='text-muted-foreground mt-1 text-xs'>
                {props.t(item.category)}
              </p>
            </TableCell>
            <TableCell>{item.interface_count.toLocaleString()}</TableCell>
            <TableCell>
              {formatCnyMoney({
                micros: item.upstream_cost_micros,
                amount: item.upstream_cost,
              })}
            </TableCell>
            <TableCell>
              <PriceEditor
                item={item}
                disabled={props.pendingId !== null}
                onSave={(priceMicros) =>
                  props.onUpdate(item.id, { price_micros: priceMicros })
                }
                t={props.t}
              />
            </TableCell>
            <TableCell>
              <EnterpriseGrantDialog item={item} />
            </TableCell>
            <TableCell>
              <div className='flex items-center gap-3'>
                <Switch
                  aria-label={props.t('Enable {{name}}', { name: item.name })}
                  checked={item.enabled}
                  disabled={
                    props.pendingId !== null ||
                    item.schema_status === 'unavailable'
                  }
                  onCheckedChange={(enabled) =>
                    props.onUpdate(item.id, { enabled })
                  }
                />
                <span className='text-muted-foreground text-xs'>
                  {catalogAvailabilityLabel(item, props.t)}
                </span>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function CatalogCard(props: {
  item: SearchAdminCatalogItem
  pending: boolean
  onUpdate: (patch: { enabled?: boolean; price_micros?: number }) => void
  t: (key: string, options?: Record<string, unknown>) => string
}) {
  const { item, t } = props
  return (
    <article className='space-y-4 p-4'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate font-medium'>{item.name}</p>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(item.category)}
          </p>
        </div>
        <Switch
          aria-label={t('Enable {{name}}', { name: item.name })}
          checked={item.enabled}
          disabled={props.pending || props.item.schema_status === 'unavailable'}
          onCheckedChange={(enabled) => props.onUpdate({ enabled })}
        />
      </div>
      <p
        className={
          item.schema_status === 'unavailable'
            ? 'text-destructive text-xs font-medium'
            : 'text-muted-foreground text-xs'
        }
      >
        {catalogAvailabilityLabel(item, t)}
      </p>
      <p className='text-muted-foreground text-sm leading-6'>
        {item.description}
      </p>
      <div className='text-muted-foreground flex items-center justify-between text-xs'>
        <span>
          {t('{{count}} interfaces', { count: item.interface_count })}
        </span>
        <span>
          {formatCnyMoney({
            micros: item.upstream_cost_micros,
            amount: item.upstream_cost,
          })}
        </span>
      </div>
      <PriceEditor
        item={item}
        disabled={props.pending}
        onSave={(priceMicros) => props.onUpdate({ price_micros: priceMicros })}
        t={t}
      />
      <EnterpriseGrantDialog item={item} />
    </article>
  )
}

function EnterpriseGrantDialog(props: { item: SearchAdminCatalogItem }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [selectedEnterpriseIDs, setSelectedEnterpriseIDs] = useState<
    Set<number>
  >(new Set())
  const grantQueryKey = ['search-capability-enterprise-grants', props.item.id]
  const grantsQuery = useQuery({
    queryKey: grantQueryKey,
    queryFn: () => fetchSearchCapabilityEnterpriseGrants(props.item.id),
    enabled: open,
  })
  const enterprisesQuery = useQuery({
    queryKey: ['search-grant-enterprises'],
    queryFn: fetchSearchGrantEnterprises,
    enabled: open,
  })
  const updateMutation = useMutation({
    mutationFn: () =>
      updateSearchCapabilityEnterpriseGrants(
        props.item.id,
        [...selectedEnterpriseIDs].sort((left, right) => left - right)
      ),
    onSuccess: (result) => {
      queryClient.setQueryData(grantQueryKey, result)
      toast.success(t('Enterprise access saved'))
      setOpen(false)
    },
    onError: handleServerError,
  })

  useEffect(() => {
    if (!open || !grantsQuery.data) return
    setSelectedEnterpriseIDs(new Set(grantsQuery.data.enterprise_ids))
  }, [grantsQuery.data, open])

  const toggleEnterprise = (enterpriseID: number) => {
    setSelectedEnterpriseIDs((current) => {
      const next = new Set(current)
      if (next.has(enterpriseID)) next.delete(enterpriseID)
      else next.add(enterpriseID)
      return next
    })
  }
  const loading = grantsQuery.isLoading || enterprisesQuery.isLoading
  const loadFailed = grantsQuery.isError || enterprisesQuery.isError

  let grantContent: ReactNode
  if (loading) {
    grantContent = (
      <div className='space-y-3 py-2' aria-hidden='true'>
        {Array.from({ length: 3 }, (_, index) => (
          <Skeleton key={index} className='h-10 w-full' />
        ))}
      </div>
    )
  } else if (loadFailed) {
    grantContent = (
      <Empty className='min-h-40 rounded-md border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <CircleAlert aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('Failed to load enterprise access')}</EmptyTitle>
        </EmptyHeader>
        <Button
          variant='outline'
          onClick={() => {
            void grantsQuery.refetch()
            void enterprisesQuery.refetch()
          }}
        >
          {t('Retry')}
        </Button>
      </Empty>
    )
  } else {
    grantContent = (
      <div className='max-h-80 space-y-2 overflow-y-auto py-1'>
        <label className='flex cursor-pointer items-center gap-3 rounded-md border p-3'>
          <Checkbox
            aria-label={t('All enterprises')}
            checked={selectedEnterpriseIDs.size === 0}
            onCheckedChange={() => setSelectedEnterpriseIDs(new Set())}
          />
          <span>
            <span className='block text-sm font-medium'>
              {t('All enterprises')}
            </span>
            <span className='text-muted-foreground block text-xs'>
              {t('Available to every enterprise')}
            </span>
          </span>
        </label>
        {(enterprisesQuery.data || []).map((enterprise) => (
          <label
            key={enterprise.id}
            className='flex cursor-pointer items-center gap-3 rounded-md border p-3'
          >
            <Checkbox
              aria-label={t('Grant {{name}}', { name: enterprise.name })}
              checked={selectedEnterpriseIDs.has(enterprise.id)}
              onCheckedChange={() => toggleEnterprise(enterprise.id)}
            />
            <span className='min-w-0'>
              <span className='block truncate text-sm font-medium'>
                {enterprise.name}
              </span>
              <span className='text-muted-foreground block truncate font-mono text-xs'>
                {enterprise.code}
              </span>
            </span>
          </label>
        ))}
      </div>
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant='outline' size='sm' />}>
        <Building2 aria-hidden='true' />
        {t('Enterprise access')}
      </DialogTrigger>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {t('Enterprise access for {{name}}', { name: props.item.name })}
          </DialogTitle>
          <DialogDescription>
            {t(
              'No enterprise selected means this capability is available to all enterprises. Select one or more enterprises to restrict access.'
            )}
          </DialogDescription>
        </DialogHeader>
        {grantContent}
        <DialogFooter>
          <Button
            onClick={() => updateMutation.mutate()}
            disabled={loading || loadFailed || updateMutation.isPending}
          >
            {updateMutation.isPending ? t('Saving…') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function PriceEditor(props: {
  item: SearchAdminCatalogItem
  disabled: boolean
  onSave: (priceMicros: number) => void
  t: (key: string, options?: Record<string, unknown>) => string
}) {
  const initialMicros = resolveCnyMicros({
    micros: props.item.price_micros,
    amount: props.item.price,
  })
  const [value, setValue] = useState(
    initialMicros === null ? '' : formatMicrosForInput(initialMicros)
  )
  const parsed = value.trim() === '' ? 0 : parseCnyInputToMicros(value)
  const valid = parsed !== null
  return (
    <div className='flex min-w-52 gap-2'>
      <Input
        aria-label={props.t('Price for {{name}}', { name: props.item.name })}
        type='number'
        min={0}
        step='0.000001'
        className='h-8 w-28'
        value={value}
        onChange={(event) => setValue(event.target.value)}
        disabled={props.disabled}
      />
      <Button
        variant='outline'
        size='sm'
        disabled={props.disabled || !valid}
        onClick={() => parsed !== null && props.onSave(parsed)}
      >
        {props.t('Save')}
      </Button>
    </div>
  )
}

function CatalogMetric(props: { label: string; value: string }) {
  return (
    <Card>
      <CardContent className='p-4'>
        <div className='text-muted-foreground text-xs font-medium'>
          {props.label}
        </div>
        <div className='mt-4 font-mono text-2xl font-semibold tabular-nums'>
          {props.value}
        </div>
      </CardContent>
    </Card>
  )
}

function catalogAvailabilityLabel(
  item: SearchAdminCatalogItem,
  t: (key: string, options?: Record<string, unknown>) => string
) {
  if (item.schema_status === 'unavailable') {
    return t('Parameter schema unavailable')
  }
  return item.enabled ? t('Enabled') : t('Disabled')
}

function CategoryButton(props: {
  active: boolean
  label: string
  onClick: () => void
}) {
  return (
    <Button
      variant={props.active ? 'default' : 'outline'}
      size='sm'
      className='shrink-0'
      aria-pressed={props.active}
      onClick={props.onClick}
    >
      {props.label}
    </Button>
  )
}

function CatalogSkeleton() {
  return (
    <div className='space-y-4 p-5' aria-hidden='true'>
      {Array.from({ length: 5 }, (_, index) => (
        <Skeleton key={index} className='h-12 w-full' />
      ))}
    </div>
  )
}
