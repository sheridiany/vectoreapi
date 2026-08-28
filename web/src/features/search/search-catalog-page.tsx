/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  BookOpen,
  CircleAlert,
  Gauge,
  RefreshCw,
  Search,
  Sparkles,
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
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

import { fetchSearchCatalog, type SearchCatalogItem } from './api'
import { SearchShell } from './components/search-shell'
import { formatCnyMoney } from './money'

const ALL_CATEGORIES = 'all'
const EMPTY_CATALOG: SearchCatalogItem[] = []

export function SearchCatalogPage() {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState(ALL_CATEGORIES)
  const catalogQuery = useQuery({
    queryKey: ['search-catalog'],
    queryFn: fetchSearchCatalog,
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
  const availableCount = catalog.filter(
    (item) => item.status === 'available' && item.enabled
  ).length
  const interfaceCount = catalog.reduce(
    (total, item) => total + item.interface_count,
    0
  )

  let catalogContent: ReactNode
  if (catalogQuery.isLoading) {
    catalogContent = <CatalogSkeleton />
  } else if (catalogQuery.isError) {
    catalogContent = (
      <Empty className='bg-card min-h-64 border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <CircleAlert aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('Failed to load capability catalog')}</EmptyTitle>
          <EmptyDescription>
            {t('Check your connection and try again.')}
          </EmptyDescription>
        </EmptyHeader>
        <Button variant='outline' onClick={() => void catalogQuery.refetch()}>
          {t('Retry')}
        </Button>
      </Empty>
    )
  } else if (filteredCatalog.length === 0) {
    catalogContent = (
      <Empty className='bg-card min-h-64 border'>
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
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
        {filteredCatalog.map((item) => (
          <CapabilityCard key={item.id} item={item} t={t} />
        ))}
      </div>
    )
  }

  return (
    <SearchShell
      title={t('vSearch capabilities')}
      description={t(
        'Browse the live vSearch catalog, availability, interface counts, and recent latency.'
      )}
      action={
        <Button render={<Link to='/search/keys' />}>
          {t('Create vSearch key')}
          <ArrowRight data-icon='inline-end' aria-hidden='true' />
        </Button>
      }
    >
      <section className='bg-card rounded-xl border p-5 shadow-sm sm:p-7'>
        <div className='flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between'>
          <div className='max-w-3xl'>
            <p className='text-muted-foreground text-xs font-semibold tracking-[0.18em] uppercase'>
              <span translate='no'>{t('VECTOR EPOCH SEARCH')}</span>
            </p>
            <h2 className='mt-2 text-2xl font-semibold tracking-tight sm:text-3xl'>
              {t('Find the right capability for an Agent task')}
            </h2>
            <p className='text-muted-foreground mt-3 text-sm leading-6 sm:text-base'>
              {t(
                'Search by service or category, then create a vSearch key with the capabilities your Agent needs.'
              )}
            </p>
          </div>
          <StatusBadge
            label={t('Live capability catalog')}
            icon={BookOpen}
            variant='info'
            copyable={false}
            size='lg'
          />
        </div>

        <div className='mt-6 grid gap-3 sm:grid-cols-3'>
          <CatalogMetric
            label={t('Available capabilities')}
            value={
              catalogQuery.isLoading ? '—' : availableCount.toLocaleString()
            }
          />
          <CatalogMetric
            label={t('Callable interfaces')}
            value={
              catalogQuery.isLoading ? '—' : interfaceCount.toLocaleString()
            }
          />
          <CatalogMetric
            label={t('Catalog capabilities')}
            value={
              catalogQuery.isLoading ? '—' : catalog.length.toLocaleString()
            }
          />
        </div>
      </section>

      <Card>
        <CardContent className='space-y-4 p-4 sm:p-5'>
          <label className='relative block' htmlFor='search-catalog-query'>
            <Search
              className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2'
              aria-hidden='true'
            />
            <Input
              id='search-catalog-query'
              aria-label={t('Search capability catalog')}
              className='h-10 pl-9'
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('Search capability catalog')}
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

      <section aria-labelledby='search-catalog-list-title'>
        <div className='mb-3 flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2
              id='search-catalog-list-title'
              className='text-lg font-semibold'
            >
              {t('Available capabilities')}
            </h2>
            {!catalogQuery.isLoading && (
              <p className='text-muted-foreground mt-1 text-sm'>
                {filteredCatalog.length.toLocaleString()} /{' '}
                {catalog.length.toLocaleString()}
              </p>
            )}
          </div>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void catalogQuery.refetch()}
            disabled={catalogQuery.isFetching}
          >
            <RefreshCw
              data-icon='inline-start'
              className={catalogQuery.isFetching ? 'animate-spin' : undefined}
              aria-hidden='true'
            />
            {t('Refresh')}
          </Button>
        </div>

        {catalogContent}
      </section>
    </SearchShell>
  )
}

function CapabilityCard(props: {
  item: SearchCatalogItem
  t: (key: string, options?: Record<string, unknown>) => string
}) {
  const { item, t } = props
  const available = item.status === 'available' && item.enabled
  return (
    <Card size='sm'>
      <CardContent className='flex h-full flex-col p-4'>
        <div className='flex items-start justify-between gap-3'>
          <div className='bg-primary/10 text-primary flex size-10 shrink-0 items-center justify-center rounded-lg'>
            <Sparkles className='size-5' aria-hidden='true' />
          </div>
          <StatusBadge
            label={available ? t('Available') : t('Unavailable')}
            variant={available ? 'success' : 'neutral'}
            copyable={false}
          />
        </div>
        <h3 className='mt-4 font-semibold'>{item.name}</h3>
        <p className='text-muted-foreground mt-1 text-xs'>{t(item.category)}</p>
        <p className='text-muted-foreground mt-3 flex-1 text-sm leading-6'>
          {item.description}
        </p>
        <div className='text-muted-foreground mt-4 flex flex-wrap gap-x-4 gap-y-2 border-t pt-3 text-xs'>
          <span>
            {t('{{count}} interfaces', { count: item.interface_count })}
          </span>
          {item.recent_latency_ms != null && (
            <span className='inline-flex items-center gap-1'>
              <Gauge className='size-3.5' aria-hidden='true' />
              {item.recent_latency_ms.toLocaleString()} ms
            </span>
          )}
          {catalogPriceLabel(item) && <span>{catalogPriceLabel(item)}</span>}
        </div>
      </CardContent>
    </Card>
  )
}

function catalogPriceLabel(item: SearchCatalogItem) {
  if (
    item.price_min_micros === undefined ||
    item.price_max_micros === undefined ||
    (item.price_min_micros === 0 && item.price_max_micros === 0)
  ) {
    return item.cost_label || ''
  }
  const minimum = formatCnyMoney({ micros: item.price_min_micros })
  if (item.price_min_micros === item.price_max_micros) return minimum
  return `${minimum} – ${formatCnyMoney({ micros: item.price_max_micros })}`
}

function CatalogMetric(props: { label: string; value: string }) {
  return (
    <div className='bg-muted/50 rounded-lg p-4'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 text-xl font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function CategoryButton(props: {
  active: boolean
  label: string
  onClick: () => void
}) {
  return (
    <Button
      type='button'
      variant={props.active ? 'default' : 'outline'}
      size='sm'
      aria-pressed={props.active}
      className='shrink-0'
      onClick={props.onClick}
    >
      {props.label}
    </Button>
  )
}

function CatalogSkeleton() {
  return (
    <div
      className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'
      aria-hidden='true'
    >
      {Array.from({ length: 6 }, (_, index) => (
        <Card key={index} size='sm'>
          <CardContent className='space-y-4 p-4'>
            <Skeleton className='size-10 rounded-lg' />
            <Skeleton className='h-5 w-1/2' />
            <Skeleton className='h-16 w-full' />
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
