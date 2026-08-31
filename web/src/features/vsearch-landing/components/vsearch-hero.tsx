/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import type { CatalogMetrics } from '../lib/catalog-view-model'
import { LiveCapabilityIndex } from './live-capability-index'
import { VSearchAccessLink } from './vsearch-access-link'

interface VSearchHeroProps {
  metrics: CatalogMetrics
  isAuthenticated: boolean
  isLoading: boolean
  isError: boolean
}

export function VSearchHero(props: VSearchHeroProps) {
  const { t } = useTranslation()
  const primaryLabel = props.isAuthenticated
    ? t('Open vSearch catalog')
    : t('Start with vSearch')

  return (
    <section className='relative overflow-hidden border-b border-[#c6d7ef] bg-[#eef5ff] px-4 pt-20 pb-16 sm:px-6 sm:pt-24 dark:border-white/12 dark:bg-[#050b16]'>
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 opacity-70 dark:opacity-20'
        style={{
          backgroundImage:
            'radial-gradient(circle at 76% 18%, rgba(21,94,239,0.18), transparent 30%), radial-gradient(circle, rgba(21,94,239,0.12) 1px, transparent 1.2px)',
          backgroundSize: '100% 100%, 18px 18px',
        }}
      />
      <div className='relative mx-auto grid max-w-[88rem] gap-12 lg:grid-cols-[minmax(0,0.95fr)_minmax(34rem,1.05fr)] lg:items-center lg:gap-12'>
        <div className='max-w-4xl'>
          <p className='font-mono text-[10px] font-semibold tracking-[0.18em] text-[#155eef] uppercase dark:text-[#78a7ff]'>
            {t('Real-world data, standardized')}
          </p>
          <h1
            className='mt-6 max-w-4xl font-sans text-[clamp(3.25rem,5vw,5.4rem)] leading-[0.9] font-semibold tracking-[-0.065em] text-balance text-[#0b1324] dark:text-[#f1f6ff]'
            aria-label={`${t('Turn the real world,')} ${t('into callable capabilities.')}`}
          >
            <span className='block'>{t('Turn the real world,')}</span>
            <span className='block'>{t('into callable capabilities.')}</span>
          </h1>
          <p className='mt-7 max-w-2xl text-base leading-7 text-[#53647d] sm:text-lg sm:leading-8 dark:text-white/55'>
            {t(
              'Search, extract, and understand public web and social data while your product contract stays stable.'
            )}
          </p>

          <div className='mt-10 flex flex-col gap-3 sm:flex-row sm:items-center'>
            <VSearchAccessLink
              isAuthenticated={props.isAuthenticated}
              className='group inline-flex h-12 items-center justify-center rounded-md bg-[#155eef] px-6 text-sm font-semibold whitespace-nowrap text-white transition-[transform,background-color] hover:-translate-y-0.5 hover:bg-[#0b4bc4] focus-visible:ring-3 focus-visible:ring-[#155eef]/35 focus-visible:outline-none active:translate-y-px dark:bg-[#78a7ff] dark:text-[#050b16] dark:hover:bg-[#9cbdff]'
            >
              {primaryLabel}
              <HugeiconsIcon
                icon={ArrowRight01Icon}
                className='ml-2 size-4 transition-transform group-hover:translate-x-0.5'
                strokeWidth={2}
                aria-hidden='true'
              />
            </VSearchAccessLink>
            <a
              href='#contract'
              className='inline-flex h-12 items-center justify-center rounded-md border border-[#9fb3d0] px-6 text-sm font-semibold whitespace-nowrap text-[#274466] transition-[transform,border-color,color] hover:-translate-y-0.5 hover:border-[#155eef] hover:text-[#155eef] focus-visible:ring-3 focus-visible:ring-[#155eef]/25 focus-visible:outline-none active:translate-y-px dark:border-white/18 dark:text-white/72 dark:hover:border-[#78a7ff] dark:hover:text-[#78a7ff]'
            >
              {t('View standard contract')}
            </a>
          </div>

          <div className='mt-14 max-w-md rounded-2xl border border-[#b9c9df] bg-white/72 px-5 py-4 dark:border-white/14 dark:bg-white/[0.025]'>
            <p className='text-[9px] font-semibold tracking-[0.12em] text-[#718099] uppercase dark:text-white/36'>
              {t('Standard layer')}
            </p>
            <div className='mt-4 grid grid-cols-2 divide-x divide-[#b9c9df] dark:divide-white/14'>
              <HeroMetric
                label={t('Standard capabilities')}
                value={
                  props.isLoading || props.isError
                    ? '-'
                    : props.metrics.capabilityCount
                }
              />
              <HeroMetric
                label={t('Mapped interfaces')}
                value={
                  props.isLoading || props.isError
                    ? '-'
                    : props.metrics.interfaceCount
                }
                className='pl-5'
              />
            </div>
          </div>
        </div>

        <LiveCapabilityIndex
          metrics={props.metrics}
          isLoading={props.isLoading}
          isError={props.isError}
        />
      </div>
    </section>
  )
}

function HeroMetric(props: {
  label: string
  value: string | number
  className?: string
}) {
  return (
    <div className={`pr-3 ${props.className || ''}`}>
      <span className='block font-mono text-2xl font-semibold text-[#0b1324] tabular-nums dark:text-[#f1f6ff]'>
        {props.value}
      </span>
      <span className='mt-1 block text-[9px] font-semibold tracking-[0.12em] text-[#718099] uppercase dark:text-white/36'>
        {props.label}
      </span>
    </div>
  )
}
