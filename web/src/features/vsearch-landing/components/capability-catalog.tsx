/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { ArrowRight01Icon, ReloadIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatVSearchPlatformLabel } from '@/features/search/lib/platform-label'
import { formatCnyMoney } from '@/features/search/money'

import {
  getCallableInterfaceCount,
  getCatalogItemState,
  type CatalogGroup,
  type CatalogItemState,
  type CatalogViewModel,
} from '../lib/catalog-view-model'
import type { PublicSearchCatalogItem } from '../types'
import { VSearchAccessLink } from './vsearch-access-link'

interface CapabilityCatalogProps {
  viewModel: CatalogViewModel
  isAuthenticated: boolean
  isLoading: boolean
  isError: boolean
  onRetry: () => void
}

const STATE_STYLES: Record<CatalogItemState, string> = {
  available:
    'border-[#155eef] text-[#155eef] dark:border-[#78a7ff] dark:text-[#78a7ff]',
  catalog:
    'border-[#6f8ebc] text-[#536f98] dark:border-[#7896c2] dark:text-[#91abd2]',
  unavailable:
    'border-[#7d8ca1] text-[#637187] dark:border-white/30 dark:text-white/50',
  disabled:
    'border-[#b6c0ce] text-[#8a96a8] dark:border-white/18 dark:text-white/32',
}

function getStatusLabelKey(state: CatalogItemState) {
  if (state === 'available') return 'Available'
  if (state === 'catalog') return 'Cataloged'
  if (state === 'disabled') return 'Disabled'
  return 'Unavailable'
}

function getPriceLabel(item: PublicSearchCatalogItem) {
  const minimumMicros = item.price_min_micros ?? item.price_max_micros
  const maximumMicros = item.price_max_micros ?? item.price_min_micros
  const validMinimum = isDisplayablePrice(minimumMicros)
  const validMaximum = isDisplayablePrice(maximumMicros)
  if (validMinimum && validMaximum) {
    const minimum = formatCnyMoney({ micros: minimumMicros })
    if (minimumMicros === maximumMicros) return minimum
    return `${minimum} - ${formatCnyMoney({ micros: maximumMicros })}`
  }
  return item.cost_label?.trim() || '-'
}

function isDisplayablePrice(value: number | undefined): value is number {
  return (
    value !== undefined &&
    Number.isSafeInteger(value) &&
    value >= 0 &&
    value <= 9_000_000_000_000_000
  )
}

function getPlatformLabel(item: PublicSearchCatalogItem) {
  const platforms = item.supported_platforms || []
  if (platforms.length === 0) return '-'

  const visiblePlatforms = platforms
    .slice(0, 3)
    .map(formatVSearchPlatformLabel)
    .join(', ')
  if (platforms.length <= 3) return visiblePlatforms
  return `${visiblePlatforms}, +${platforms.length - 3}`
}

function CapabilityStatus(props: { state: CatalogItemState }) {
  const { t } = useTranslation()
  return (
    <span
      className={`inline-flex border-l-2 pl-2 text-[10px] font-semibold tracking-[0.1em] uppercase ${STATE_STYLES[props.state]}`}
    >
      {t(getStatusLabelKey(props.state))}
    </span>
  )
}

function CapabilityRow(props: { item: PublicSearchCatalogItem }) {
  const { t } = useTranslation()
  const state = getCatalogItemState(props.item)
  const callableInterfaces = getCallableInterfaceCount(props.item)

  return (
    <article className='grid grid-cols-2 gap-5 border-t border-[#c5d2e3] py-6 first:border-t-0 lg:grid-cols-[minmax(12rem,1.3fr)_minmax(12rem,1.45fr)_minmax(8rem,0.8fr)_minmax(5rem,0.6fr)_minmax(6rem,0.65fr)_minmax(5.5rem,0.55fr)] lg:items-center lg:gap-6 dark:border-white/10'>
      <div className='col-span-2 min-w-0 lg:col-span-1'>
        <div className='flex items-start justify-between gap-4 lg:block'>
          <h3 className='text-lg leading-6 font-semibold tracking-[-0.02em] text-[#0b1324] dark:text-white/90'>
            {props.item.name}
          </h3>
          <span className='shrink-0 lg:hidden'>
            <CapabilityStatus state={state} />
          </span>
        </div>
      </div>
      <p className='col-span-2 text-sm leading-6 text-[#586b86] lg:col-span-1 dark:text-white/48'>
        {props.item.description}
      </p>
      <CatalogDatum
        label={t('Platform')}
        value={getPlatformLabel(props.item)}
        className='col-span-2 lg:col-span-1'
      />
      <CatalogDatum
        label={t('Interfaces')}
        value={`${callableInterfaces}/${Math.max(0, props.item.interface_count)}`}
        mono
      />
      <CatalogDatum label={t('Cost')} value={getPriceLabel(props.item)} mono />
      <div className='hidden lg:block'>
        <CapabilityStatus state={state} />
      </div>
    </article>
  )
}

function CatalogDatum(props: {
  label: string
  value: string
  mono?: boolean
  className?: string
}) {
  return (
    <div className={`min-w-0 ${props.className || ''}`}>
      <span className='mb-1 block text-[9px] font-semibold tracking-[0.1em] text-[#7b899e] uppercase lg:hidden dark:text-white/30'>
        {props.label}
      </span>
      <span
        className={`block truncate text-xs leading-5 text-[#40536f] dark:text-white/52 ${props.mono ? 'font-mono tabular-nums' : ''}`}
      >
        {props.value}
      </span>
    </div>
  )
}

function CapabilityGroupSection(props: { group: CatalogGroup }) {
  const { t } = useTranslation()
  return (
    <section className='border-t border-[#8fa5c2] pt-6 dark:border-white/22'>
      <div className='flex items-baseline justify-between gap-6 pb-5'>
        <h3 className='text-xl font-semibold tracking-[-0.025em] text-[#17345f] dark:text-[#a8c5f7]'>
          {t(props.group.category)}
        </h3>
        <p className='font-mono text-[10px] text-[#718099] dark:text-white/34'>
          {props.group.items.length} {t('contracts')}
        </p>
      </div>
      <div>
        {props.group.items.map((item) => (
          <CapabilityRow key={item.id} item={item} />
        ))}
      </div>
    </section>
  )
}

export function CapabilityCatalog(props: CapabilityCatalogProps) {
  const { t } = useTranslation()
  const catalogAction = props.isAuthenticated
    ? t('Open vSearch catalog')
    : t('Start with vSearch')

  return (
    <section
      id='capabilities'
      className='scroll-mt-28 border-b border-[#b9c9df] bg-[#f4f7fb] px-4 py-20 sm:px-6 md:py-28 dark:border-white/14 dark:bg-[#050b16]'
    >
      <div className='mx-auto max-w-7xl'>
        <div className='max-w-4xl pb-12'>
          <h2 className='font-sans text-4xl leading-[0.98] font-semibold tracking-[-0.045em] text-[#0b1324] sm:text-5xl md:text-6xl dark:text-[#f1f6ff]'>
            {t('Know exactly what vSearch can do today.')}
          </h2>
          <p className='mt-6 max-w-2xl text-sm leading-7 text-[#586b86] dark:text-white/50'>
            {t(
              'Availability reflects published provider bindings, not a static marketing list.'
            )}
          </p>
        </div>

        <div className='border-y border-[#8fa5c2] dark:border-white/22'>
          <div className='hidden grid-cols-[minmax(12rem,1.3fr)_minmax(12rem,1.45fr)_minmax(8rem,0.8fr)_minmax(5rem,0.6fr)_minmax(6rem,0.65fr)_minmax(5.5rem,0.55fr)] gap-6 py-3 text-[9px] font-semibold tracking-[0.12em] text-[#718099] uppercase lg:grid dark:text-white/34'>
            <span>{t('Capability')}</span>
            <span>{t('Standard contract')}</span>
            <span>{t('Platform')}</span>
            <span>{t('Interfaces')}</span>
            <span>{t('Cost')}</span>
            <span>{t('Status')}</span>
          </div>

          {props.isLoading ? <CatalogLoading /> : null}

          {props.isError ? <CatalogError onRetry={props.onRetry} /> : null}

          {!props.isLoading &&
          !props.isError &&
          props.viewModel.groups.length === 0 ? (
            <CatalogEmpty />
          ) : null}

          {!props.isLoading &&
          !props.isError &&
          props.viewModel.groups.length > 0 ? (
            <div className='space-y-12 py-10 md:py-12'>
              {props.viewModel.groups.map((group) => (
                <CapabilityGroupSection key={group.category} group={group} />
              ))}
            </div>
          ) : null}
        </div>

        <div className='mt-8 flex justify-end'>
          <VSearchAccessLink
            isAuthenticated={props.isAuthenticated}
            className='group inline-flex h-11 w-full items-center justify-center rounded-md bg-[#155eef] px-5 text-sm font-semibold whitespace-nowrap text-white transition-[transform,background-color] hover:-translate-y-0.5 hover:bg-[#0b4bc4] focus-visible:ring-3 focus-visible:ring-[#155eef]/35 focus-visible:outline-none active:translate-y-px sm:w-fit dark:bg-[#78a7ff] dark:text-[#050b16] dark:hover:bg-[#9cbdff]'
          >
            {catalogAction}
            <HugeiconsIcon
              icon={ArrowRight01Icon}
              className='ml-2 size-4 transition-transform group-hover:translate-x-0.5'
              strokeWidth={2}
              aria-hidden='true'
            />
          </VSearchAccessLink>
        </div>
      </div>
    </section>
  )
}

function CatalogLoading() {
  const { t } = useTranslation()
  return (
    <div
      className='space-y-8 py-10'
      aria-label={t('Loading vSearch capability catalog')}
    >
      {[0, 1, 2, 3].map((index) => (
        <div
          key={index}
          className='grid grid-cols-2 gap-4 border-t border-[#c5d2e3] pt-6 first:border-t-0 lg:grid-cols-[minmax(12rem,1.3fr)_minmax(12rem,1.45fr)_minmax(8rem,0.8fr)_minmax(5rem,0.6fr)_minmax(6rem,0.65fr)_minmax(5.5rem,0.55fr)] lg:gap-6 dark:border-white/10'
        >
          <Skeleton className='h-5 w-40 rounded-none bg-[#c6d4e7] dark:bg-white/10' />
          <Skeleton className='h-5 w-full rounded-none bg-[#d7e0ec] dark:bg-white/7' />
          <Skeleton className='h-4 w-24 rounded-none bg-[#d7e0ec] dark:bg-white/7' />
          <Skeleton className='h-4 w-12 rounded-none bg-[#d7e0ec] dark:bg-white/7' />
          <Skeleton className='h-4 w-16 rounded-none bg-[#d7e0ec] dark:bg-white/7' />
          <Skeleton className='h-4 w-20 rounded-none bg-[#d7e0ec] dark:bg-white/7' />
        </div>
      ))}
    </div>
  )
}

function CatalogError(props: { onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <div className='py-14 md:py-20'>
      <p className='font-sans text-3xl font-semibold tracking-[-0.03em] text-[#0b1324] dark:text-[#f1f6ff]'>
        {t('The live catalog could not be reached.')}
      </p>
      <p className='mt-3 max-w-xl text-sm leading-7 text-[#586b86] dark:text-white/50'>
        {t(
          'The product page is still available. Retry to load current capability and provider readiness.'
        )}
      </p>
      <Button
        type='button'
        variant='outline'
        className='mt-6 rounded-md border-[#9fb3d0] bg-transparent text-[#274466] hover:border-[#155eef] hover:bg-[#e6eef9] hover:text-[#155eef] dark:border-white/18 dark:text-white/70 dark:hover:border-[#78a7ff] dark:hover:bg-white/[0.045] dark:hover:text-[#78a7ff]'
        onClick={props.onRetry}
      >
        <HugeiconsIcon
          icon={ReloadIcon}
          data-icon='inline-start'
          strokeWidth={2}
          aria-hidden='true'
        />
        {t('Retry live catalog')}
      </Button>
    </div>
  )
}

function CatalogEmpty() {
  const { t } = useTranslation()
  return (
    <div className='py-14 md:py-20'>
      <p className='font-sans text-3xl font-semibold tracking-[-0.03em] text-[#0b1324] dark:text-[#f1f6ff]'>
        {t('The standard catalog is being prepared.')}
      </p>
      <p className='mt-3 max-w-xl text-sm leading-7 text-[#586b86] dark:text-white/50'>
        {t(
          'Capability contracts will appear here as soon as they are published.'
        )}
      </p>
    </div>
  )
}
