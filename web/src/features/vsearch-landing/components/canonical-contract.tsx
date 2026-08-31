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

export function CanonicalContract() {
  const { t } = useTranslation()

  return (
    <section
      id='contract'
      className='relative scroll-mt-28 overflow-hidden border-b border-[#c7d8f0] bg-[#edf5ff] px-4 py-20 sm:px-6 md:py-28 dark:border-white/12 dark:bg-[#071427]'
    >
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 opacity-60 dark:opacity-16'
        style={{
          backgroundImage:
            'radial-gradient(circle, rgba(23,105,255,0.14) 1px, transparent 1.2px)',
          backgroundSize: '18px 18px',
        }}
      />
      <div className='relative mx-auto max-w-[88rem]'>
        <div className='max-w-5xl'>
          <p className='text-[10px] font-semibold tracking-[0.18em] text-[#1769ff] uppercase dark:text-[#83b2ff]'>
            {t('Canonical contract')}
          </p>
          <h2 className='mt-6 text-[clamp(2.7rem,5vw,5.5rem)] leading-[0.94] font-semibold tracking-[-0.065em] text-balance text-[#0b1324] dark:text-[#f1f6ff]'>
            {t('One contract connects every public source.')}
          </h2>
          <p className='mt-6 max-w-2xl text-sm leading-7 text-[#586b86] md:text-base dark:text-white/50'>
            {t(
              'Inputs and outputs stay stable whether the result comes from the web, a social platform, or a video index.'
            )}
          </p>
        </div>

        <div className='relative mt-14 grid gap-5 lg:grid-cols-2'>
          <CodePanel label={t('Request')} code={REQUEST_EXAMPLE} />
          <span className='absolute top-1/2 left-1/2 z-10 hidden size-10 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-[#1769ff] text-white shadow-lg lg:flex dark:bg-[#83b2ff] dark:text-[#071427]'>
            <HugeiconsIcon
              icon={ArrowRight01Icon}
              className='size-5'
              strokeWidth={2}
              aria-hidden='true'
            />
          </span>
          <CodePanel label={t('Response')} code={RESPONSE_EXAMPLE} tinted />
        </div>
      </div>
    </section>
  )
}

const REQUEST_EXAMPLE = `POST  /v1/content/search

{
  "query": "Where are large models deployed?",
  "sources": ["web", "social"],
  "limit": 10,
  "language": "en"
}`

const RESPONSE_EXAMPLE = `{
  "code": "ok",
  "data": [{
    "title": "Example result",
    "url": "https://example.com/a",
    "score": 0.92,
    "source": "web"
  }]
}`

function CodePanel(props: { label: string; code: string; tinted?: boolean }) {
  return (
    <article
      className={`min-h-[25rem] rounded-[1.5rem] border border-[#bad0ef] p-6 sm:p-8 dark:border-white/12 ${props.tinted ? 'bg-[#f6faff] dark:bg-[#0d1a2d]' : 'bg-white dark:bg-[#0b1728]'}`}
    >
      <p className='text-[9px] font-semibold tracking-[0.14em] text-[#7183a1] uppercase dark:text-white/36'>
        {props.label}
      </p>
      <pre className='mt-7 [scrollbar-width:thin] overflow-x-auto font-mono text-xs leading-7 text-[#244d8c] dark:text-[#a9c8ff]'>
        <code>{props.code}</code>
      </pre>
    </article>
  )
}
