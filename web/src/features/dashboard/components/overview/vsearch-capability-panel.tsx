/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowRight, DatabaseZap, RefreshCw } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  fetchSearchCatalog,
  type SearchCatalogItem,
} from '@/features/search/api'
import { formatVSearchPlatformLabel } from '@/features/search/lib/platform-label'

const EMPTY_CAPABILITIES: SearchCatalogItem[] = []

export function VSearchCapabilityPanel() {
  const { t } = useTranslation()
  const catalogQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'vsearch-catalog'],
    queryFn: fetchSearchCatalog,
    staleTime: 5 * 60 * 1000,
  })
  const catalog = catalogQuery.data ?? EMPTY_CAPABILITIES
  const featured = useMemo(() => catalog.slice(0, 4), [catalog])
  const availableCount = catalog.filter(
    (item) => item.status === 'available' && item.enabled
  ).length
  let catalogContent = (
    <div className='grid gap-3 lg:grid-cols-2'>
      {featured.map((item) => (
        <CapabilityPreview key={item.id} item={item} />
      ))}
    </div>
  )

  if (catalogQuery.isLoading) {
    catalogContent = (
      <div className='grid gap-3 lg:grid-cols-2' aria-hidden='true'>
        {Array.from({ length: 4 }, (_, index) => (
          <Skeleton key={index} className='h-28 rounded-xl' />
        ))}
      </div>
    )
  } else if (catalogQuery.isError) {
    catalogContent = (
      <div className='flex min-h-28 flex-col items-center justify-center gap-3 text-center'>
        <p className='text-muted-foreground text-sm'>
          {t('Failed to load capability catalog')}
        </p>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void catalogQuery.refetch()}
        >
          <RefreshCw data-icon='inline-start' aria-hidden='true' />
          {t('Retry')}
        </Button>
      </div>
    )
  } else if (featured.length === 0) {
    catalogContent = (
      <p className='text-muted-foreground py-8 text-center text-sm'>
        {t('Capability catalog is being prepared.')}
      </p>
    )
  }

  return (
    <section
      aria-labelledby='dashboard-vsearch-capabilities-title'
      className='bg-card overflow-hidden rounded-2xl border shadow-xs'
    >
      <div className='flex flex-col gap-4 border-b p-4 sm:flex-row sm:items-start sm:justify-between sm:p-5'>
        <div className='flex min-w-0 items-start gap-3'>
          <span className='bg-primary/10 text-primary flex size-10 shrink-0 items-center justify-center rounded-xl'>
            <DatabaseZap className='size-5' aria-hidden='true' />
          </span>
          <div className='min-w-0'>
            <h2
              id='dashboard-vsearch-capabilities-title'
              className='text-lg font-semibold tracking-tight'
            >
              {t('vSearch data capabilities')}
            </h2>
            <p className='text-muted-foreground mt-1 text-sm leading-6'>
              {t(
                'Search public social, commerce, and trend data through standard operations.'
              )}
            </p>
          </div>
        </div>
        <Button
          variant='outline'
          size='sm'
          render={<Link to='/search/catalog' />}
        >
          {t('Explore vSearch')}
          <ArrowRight data-icon='inline-end' aria-hidden='true' />
        </Button>
      </div>

      <div className='bg-border grid gap-px sm:grid-cols-2'>
        <CapabilityMetric
          label={t('{{count}} standard capabilities', {
            count: catalog.length,
          })}
          description={t('Published product coverage')}
          loading={catalogQuery.isLoading}
        />
        <CapabilityMetric
          label={t('{{count}} ready to call', { count: availableCount })}
          description={t('Currently verified and billable')}
          loading={catalogQuery.isLoading}
        />
      </div>

      <div className='p-4 sm:p-5'>{catalogContent}</div>
    </section>
  )
}

function CapabilityMetric(props: {
  label: string
  description: string
  loading: boolean
}) {
  return (
    <div className='bg-card px-4 py-3 sm:px-5'>
      {props.loading ? (
        <Skeleton className='h-5 w-36' />
      ) : (
        <div className='font-semibold tabular-nums'>{props.label}</div>
      )}
      <div className='text-muted-foreground mt-0.5 text-xs'>
        {props.description}
      </div>
    </div>
  )
}

function CapabilityPreview(props: { item: SearchCatalogItem }) {
  const { t } = useTranslation()
  const available =
    props.item.status === 'available' && props.item.enabled === true
  const platforms = props.item.supported_platforms ?? []

  return (
    <article className='bg-muted/25 rounded-xl border p-4'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <h3 className='truncate text-sm font-semibold'>{props.item.name}</h3>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(props.item.category)}
          </p>
        </div>
        <span
          className={
            available
              ? 'bg-success/10 text-success rounded-full px-2 py-1 text-xs'
              : 'bg-muted text-muted-foreground rounded-full px-2 py-1 text-xs'
          }
        >
          {available ? t('Ready') : t('Preparing')}
        </span>
      </div>
      {platforms.length > 0 && (
        <div className='mt-3 flex flex-wrap gap-1.5'>
          {platforms.slice(0, 4).map((platform) => (
            <span
              key={platform}
              className='bg-background text-muted-foreground rounded-md border px-2 py-1 text-[11px]'
            >
              {formatVSearchPlatformLabel(platform)}
            </span>
          ))}
        </div>
      )}
    </article>
  )
}
