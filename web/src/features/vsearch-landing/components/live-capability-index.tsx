/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import {
  ArrowRight01Icon,
  File02Icon,
  Globe02Icon,
  Message01Icon,
  PlayIcon,
  UserIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import type { CatalogMetrics } from '../lib/catalog-view-model'

interface LiveCapabilityIndexProps {
  metrics: CatalogMetrics
  isLoading: boolean
  isError: boolean
}

const SOURCES = [
  { label: 'Web', icon: Globe02Icon, color: '#1769ff', className: 'top-[4%]' },
  {
    label: 'Social',
    icon: Message01Icon,
    color: '#ff5c8a',
    className: 'top-[22%]',
  },
  {
    label: 'Video',
    icon: PlayIcon,
    color: '#ffb62e',
    className: 'top-[40%]',
  },
  {
    label: 'Documents',
    icon: File02Icon,
    color: '#28c68b',
    className: 'top-[58%]',
  },
  {
    label: 'Forums',
    icon: UserIcon,
    color: '#8c6ff7',
    className: 'top-[76%]',
  },
] as const

const OPERATIONS = [
  { name: 'vSearch.query', color: '#1769ff', className: 'top-[8%]' },
  { name: 'vSearch.extract', color: '#28c68b', className: 'top-[40%]' },
  { name: 'vSearch.creator', color: '#8c6ff7', className: 'top-[72%]' },
] as const

export function LiveCapabilityIndex(props: LiveCapabilityIndexProps) {
  const { t } = useTranslation()
  const synchronized = !props.isLoading && !props.isError
  let statusLabel = t('Live sources')
  if (props.isLoading) statusLabel = t('Reading live capability index')
  if (props.isError) statusLabel = t('Live index unavailable')

  return (
    <aside className='overflow-hidden rounded-[1.75rem] border border-[#bcd2f4] bg-white/90 shadow-[0_30px_90px_-64px_rgba(23,105,255,0.75)] dark:border-white/12 dark:bg-[#0b1728]/95'>
      <div className='flex items-center justify-between gap-4 border-b border-[#d7e2f3] px-5 py-4 dark:border-white/10'>
        <span className='text-[10px] font-semibold tracking-[0.14em] text-[#667b9d] uppercase dark:text-white/42'>
          {t('World to standard capability')}
        </span>
        <span className='flex items-center gap-2 text-[9px] font-semibold tracking-[0.1em] text-[#39734f] uppercase dark:text-[#76ddb8]'>
          <span
            className={`size-1.5 rounded-full ${synchronized ? 'bg-[#28c68b]' : 'bg-[#8a98ad]'}`}
          />
          {statusLabel}
        </span>
      </div>

      <div className='mx-5 mt-5 space-y-2 rounded-2xl bg-[#f5f8fd] p-3 sm:hidden dark:bg-white/[0.025]'>
        {SOURCES.map((source, index) => {
          const operation = OPERATIONS[index % OPERATIONS.length]
          return (
            <div
              key={source.label}
              className='grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1.15fr)] items-center gap-2 rounded-xl bg-white px-3 py-2.5 shadow-sm dark:bg-[#111f34]'
            >
              <span className='flex min-w-0 items-center gap-2 text-[8px] font-semibold tracking-[0.07em] text-[#526785] uppercase dark:text-white/60'>
                <span
                  className='flex size-6 shrink-0 items-center justify-center rounded-full text-white'
                  style={{ backgroundColor: source.color }}
                >
                  <HugeiconsIcon
                    icon={source.icon}
                    className='size-3.5'
                    strokeWidth={2}
                    aria-hidden='true'
                  />
                </span>
                <span className='truncate'>{t(source.label)}</span>
              </span>
              <HugeiconsIcon
                icon={ArrowRight01Icon}
                className='size-3.5 text-[#84a3d1]'
                strokeWidth={2}
                aria-hidden='true'
              />
              <span className='truncate rounded-lg border border-[#c8d9f3] bg-[#f6f9fe] px-2 py-2 font-mono text-[8px] font-semibold text-[#173b73] dark:border-white/12 dark:bg-white/[0.035] dark:text-white/72'>
                {operation.name}
              </span>
            </div>
          )
        })}
      </div>

      <div className='relative mx-6 mt-5 hidden h-[21rem] overflow-hidden rounded-2xl bg-[#f5f8fd] sm:block dark:bg-white/[0.025]'>
        <svg
          viewBox='0 0 700 360'
          preserveAspectRatio='none'
          className='pointer-events-none absolute inset-0 size-full'
          aria-hidden='true'
        >
          <path
            d='M126 42 C310 42 320 80 528 68'
            fill='none'
            stroke='#1769ff'
            strokeWidth='3'
            opacity='.72'
          />
          <path
            d='M126 106 C300 106 332 150 528 126'
            fill='none'
            stroke='#78b7ff'
            strokeWidth='4'
            opacity='.72'
          />
          <path
            d='M126 172 C310 172 330 210 528 192'
            fill='none'
            stroke='#8c6ff7'
            strokeWidth='3'
            opacity='.66'
          />
          <path
            d='M126 238 C310 238 350 280 528 252'
            fill='none'
            stroke='#45cff2'
            strokeWidth='4'
            opacity='.58'
          />
          <path
            d='M126 304 C300 304 360 326 528 314'
            fill='none'
            stroke='#8c6ff7'
            strokeWidth='3'
            opacity='.62'
          />
        </svg>

        {SOURCES.map((source) => (
          <div
            key={source.label}
            className={`absolute left-3 flex w-[7.2rem] items-center gap-2 rounded-xl bg-white px-3 py-2.5 text-[9px] font-semibold tracking-[0.08em] text-[#526785] uppercase shadow-sm sm:left-5 dark:bg-[#111f34] dark:text-white/60 ${source.className}`}
          >
            <span
              className='flex size-6 items-center justify-center rounded-full text-white'
              style={{ backgroundColor: source.color }}
            >
              <HugeiconsIcon
                icon={source.icon}
                className='size-3.5'
                strokeWidth={2}
                aria-hidden='true'
              />
            </span>
            {t(source.label)}
          </div>
        ))}

        {OPERATIONS.map((operation) => (
          <div
            key={operation.name}
            className={`absolute right-3 w-[10.2rem] rounded-xl border border-[#c8d9f3] bg-white px-3 py-3 shadow-sm sm:right-5 dark:border-white/12 dark:bg-[#111f34] ${operation.className}`}
          >
            <p className='text-[8px] font-semibold tracking-[0.1em] text-[#7183a1] uppercase dark:text-white/36'>
              {t('Operation')}
            </p>
            <p className='mt-1 text-xs font-semibold text-[#173b73] dark:text-white/78'>
              {operation.name}
            </p>
            <span
              className='absolute top-1/2 -left-1.5 size-3 -translate-y-1/2 rounded-full border-2 border-white dark:border-[#111f34]'
              style={{ backgroundColor: operation.color }}
            />
          </div>
        ))}
      </div>

      <div className='mx-5 mt-5 mb-5 flex items-center gap-4 rounded-2xl bg-[#edf4ff] px-4 py-4 sm:mx-6 sm:mb-6 dark:bg-white/[0.045]'>
        <div className='min-w-0 flex-1'>
          <p className='text-[8px] font-semibold tracking-[0.12em] text-[#7183a1] uppercase dark:text-white/36'>
            {t('Query')}
          </p>
          <p className='mt-1 truncate text-sm font-semibold text-[#263f66] dark:text-white/72'>
            {t('Where are large models being deployed?')}
          </p>
        </div>
        <span className='flex size-9 shrink-0 items-center justify-center rounded-full bg-[#1769ff] text-white dark:bg-[#83b2ff] dark:text-[#07101e]'>
          <HugeiconsIcon
            icon={ArrowRight01Icon}
            className='size-4'
            strokeWidth={2}
            aria-hidden='true'
          />
        </span>
      </div>

      <div className='grid grid-cols-3 border-t border-[#d7e2f3] dark:border-white/10'>
        <IndexMetric
          label={t('Capabilities')}
          value={metricValue(props, props.metrics.capabilityCount)}
        />
        <IndexMetric
          label={t('Mapped interfaces')}
          value={metricValue(props, props.metrics.interfaceCount)}
        />
        <IndexMetric
          label={t('Callable now')}
          value={metricValue(props, props.metrics.callableInterfaceCount)}
        />
      </div>
    </aside>
  )
}

function metricValue(
  props: Pick<LiveCapabilityIndexProps, 'isLoading' | 'isError'>,
  value: number
) {
  return props.isLoading || props.isError ? '—' : value
}

function IndexMetric(props: { label: string; value: string | number }) {
  return (
    <div className='border-r border-[#d7e2f3] px-3 py-3 text-center last:border-r-0 dark:border-white/10'>
      <p className='text-lg font-semibold text-[#1769ff] tabular-nums dark:text-[#83b2ff]'>
        {props.value}
      </p>
      <p className='mt-1 text-[8px] font-semibold tracking-[0.08em] text-[#7183a1] uppercase dark:text-white/34'>
        {props.label}
      </p>
    </div>
  )
}
