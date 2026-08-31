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

import { VSearchAccessLink } from './vsearch-access-link'

interface FinalCtaProps {
  isAuthenticated: boolean
}

export function FinalCta(props: FinalCtaProps) {
  const { t } = useTranslation()
  return (
    <section className='relative overflow-hidden border-b border-[#b9c9df] bg-[linear-gradient(110deg,#eef5ff_0%,#f1ecff_100%)] px-4 py-20 text-[#0b1324] sm:px-6 md:py-28 dark:border-white/14 dark:bg-[linear-gradient(110deg,#09162a_0%,#16112d_100%)] dark:text-white'>
      <svg
        aria-hidden='true'
        viewBox='0 0 1440 240'
        preserveAspectRatio='none'
        className='pointer-events-none absolute inset-x-0 bottom-0 h-40 w-full opacity-70 dark:opacity-30'
      >
        <path
          d='M-20 180 C210 18 400 286 650 133 C880 -10 1070 184 1460 35'
          fill='none'
          stroke='#45cff2'
          strokeWidth='7'
        />
        <path
          d='M-20 210 C220 54 432 302 670 160 C900 23 1110 217 1460 78'
          fill='none'
          stroke='#8c6ff7'
          strokeWidth='3'
          opacity='.7'
        />
      </svg>
      <div className='relative mx-auto max-w-[88rem]'>
        <div className='grid gap-10 md:grid-cols-[minmax(0,1fr)_auto] md:items-end md:gap-16'>
          <div className='max-w-4xl'>
            <p className='font-mono text-[10px] font-semibold tracking-[0.16em] text-[#155eef] uppercase dark:text-[#78a7ff]'>
              {t('vSearch by Vector Epoch')}
            </p>
            <h2 className='mt-5 font-sans text-4xl leading-[0.95] font-semibold tracking-[-0.045em] text-balance sm:text-5xl md:text-6xl'>
              {t('Bring real-world data directly into your product.')}
            </h2>
          </div>
          <div>
            <VSearchAccessLink
              isAuthenticated={props.isAuthenticated}
              className='group inline-flex h-12 items-center justify-center rounded-xl bg-[#155eef] px-6 text-sm font-semibold whitespace-nowrap text-white transition-[transform,background-color] hover:-translate-y-0.5 hover:bg-[#0b4bc4] focus-visible:ring-3 focus-visible:ring-[#155eef]/35 focus-visible:outline-none active:translate-y-px dark:bg-[#78a7ff] dark:text-[#050b16] dark:hover:bg-[#9cbdff]'
            >
              {t('View access guide')}
              <HugeiconsIcon
                icon={ArrowRight01Icon}
                className='ml-2 size-4 transition-transform group-hover:translate-x-0.5'
                strokeWidth={2}
                aria-hidden='true'
              />
            </VSearchAccessLink>
          </div>
        </div>
      </div>
    </section>
  )
}
