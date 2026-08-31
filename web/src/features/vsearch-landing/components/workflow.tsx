/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import {
  File02Icon,
  Image01Icon,
  Message01Icon,
  News01Icon,
  PlayIcon,
  Search01Icon,
  UserIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

const PRIMARY_CAPABILITIES = [
  {
    eyebrow: 'Content search',
    title: 'Cross-source content search',
    description:
      'Search web pages, posts, videos, and images through one contract.',
    operation: 'vSearch.query',
    icon: Search01Icon,
    className:
      'border-[#bad2fb] bg-[#e8f1ff] dark:border-[#1769ff]/25 dark:bg-[#1769ff]/10',
    iconClass: 'bg-[#1769ff] text-white dark:bg-[#83b2ff] dark:text-[#07101e]',
  },
  {
    eyebrow: 'Page extract',
    title: 'Structured page extraction',
    description: 'Return body text, metadata, and stable fields in one pass.',
    operation: 'vSearch.extract',
    icon: File02Icon,
    className:
      'border-[#ffd4bb] bg-[#fff0e5] dark:border-[#ff8b4a]/25 dark:bg-[#ff8b4a]/10',
    iconClass: 'bg-[#ff8b4a] text-white',
  },
  {
    eyebrow: 'Creator profile',
    title: 'Multi-dimensional creator profiles',
    description: 'Understand domains, audiences, and content influence.',
    operation: 'vSearch.creator',
    icon: UserIcon,
    className:
      'border-[#d6c8ff] bg-[#f0eaff] dark:border-[#8c6ff7]/25 dark:bg-[#8c6ff7]/10',
    iconClass: 'bg-[#8c6ff7] text-white',
  },
] as const

const SECONDARY_CAPABILITIES = [
  {
    eyebrow: 'News',
    title: 'News and trends',
    description: 'Track time, source, and momentum.',
    icon: News01Icon,
    color: '#28c68b',
    decoration: 'trend',
    className:
      'border-[#bcebd9] bg-[#e9f9f3] dark:border-[#28c68b]/25 dark:bg-[#28c68b]/10',
  },
  {
    eyebrow: 'Social',
    title: 'Communities and forums',
    description: 'Follow topics, comments, and discussion threads.',
    icon: Message01Icon,
    color: '#ff5c8a',
    decoration: 'network',
    className:
      'border-[#ffd0df] bg-[#fff0f5] dark:border-[#ff5c8a]/25 dark:bg-[#ff5c8a]/10',
  },
  {
    eyebrow: 'Video',
    title: 'Video and clips',
    description: 'Normalize titles, authors, subtitles, and clips.',
    icon: PlayIcon,
    color: '#ffb62e',
    decoration: 'timeline',
    className:
      'border-[#ffe1a8] bg-[#fff7e6] dark:border-[#ffb62e]/25 dark:bg-[#ffb62e]/10',
  },
  {
    eyebrow: 'Media',
    title: 'Images and galleries',
    description: 'Keep dimensions, provenance, and visual context.',
    icon: Image01Icon,
    color: '#45a3ff',
    decoration: 'gallery',
    className:
      'border-[#bfddfb] bg-[#eaf4ff] dark:border-[#45a3ff]/25 dark:bg-[#45a3ff]/10',
  },
] as const

export function Workflow() {
  const { t } = useTranslation()

  return (
    <section
      id='workflow'
      className='scroll-mt-28 border-b border-[#cbd7e8] bg-white px-4 py-20 sm:px-6 md:py-28 dark:border-white/12 dark:bg-[#08101c]'
    >
      <div className='mx-auto max-w-[88rem]'>
        <div className='max-w-4xl'>
          <p className='text-[10px] font-semibold tracking-[0.18em] text-[#1769ff] uppercase dark:text-[#83b2ff]'>
            {t('Capability atlas')}
          </p>
          <h2 className='mt-6 text-[clamp(2.7rem,5vw,5.5rem)] leading-[0.94] font-semibold tracking-[-0.065em] text-balance text-[#0b1324] dark:text-[#f1f6ff]'>
            {t('Users see capabilities, not providers.')}
          </h2>
          <p className='mt-6 max-w-2xl text-sm leading-7 text-[#586b86] md:text-base dark:text-white/50'>
            {t(
              'Every card is a stable capability contract. Upstream providers can change without changing how your product calls it.'
            )}
          </p>
        </div>

        <div className='mt-14 grid gap-5 lg:grid-cols-3'>
          {PRIMARY_CAPABILITIES.map((capability) => (
            <article
              key={capability.operation}
              className={`flex min-h-[19rem] flex-col rounded-[1.6rem] border p-6 sm:p-7 ${capability.className}`}
            >
              <div className='flex items-center gap-3'>
                <span
                  className={`flex size-11 items-center justify-center rounded-full ${capability.iconClass}`}
                >
                  <HugeiconsIcon
                    icon={capability.icon}
                    className='size-5'
                    strokeWidth={1.8}
                    aria-hidden='true'
                  />
                </span>
                <p className='text-[9px] font-semibold tracking-[0.13em] text-[#596f92] uppercase dark:text-white/42'>
                  {t(capability.eyebrow)}
                </p>
              </div>
              <h3 className='mt-8 text-2xl font-semibold tracking-[-0.035em] text-[#17243a] dark:text-white/88'>
                {t(capability.title)}
              </h3>
              <p className='mt-3 text-sm leading-7 text-[#607392] dark:text-white/48'>
                {t(capability.description)}
              </p>
              <code className='mt-auto w-fit rounded-full bg-white/65 px-3 py-2 font-mono text-[9px] text-[#155eef] dark:bg-white/[0.055] dark:text-[#9bc0ff]'>
                {capability.operation}
              </code>
            </article>
          ))}
        </div>

        <div className='mt-5 grid gap-5 sm:grid-cols-2 lg:grid-cols-4'>
          {SECONDARY_CAPABILITIES.map((capability) => (
            <article
              key={capability.eyebrow}
              className={`relative min-h-[15rem] overflow-hidden rounded-[1.5rem] border p-6 ${capability.className}`}
            >
              <span
                className='flex size-10 items-center justify-center rounded-full text-white'
                style={{ backgroundColor: capability.color }}
              >
                <HugeiconsIcon
                  icon={capability.icon}
                  className='size-5'
                  strokeWidth={1.8}
                  aria-hidden='true'
                />
              </span>
              <p className='mt-6 text-[9px] font-semibold tracking-[0.12em] text-[#637696] uppercase dark:text-white/40'>
                {t(capability.eyebrow)}
              </p>
              <h3 className='mt-2 text-xl font-semibold tracking-[-0.03em] text-[#17243a] dark:text-white/86'>
                {t(capability.title)}
              </h3>
              <p className='mt-3 text-sm leading-6 text-[#667895] dark:text-white/45'>
                {t(capability.description)}
              </p>
              <CapabilityDecoration
                type={capability.decoration}
                color={capability.color}
              />
            </article>
          ))}
        </div>
      </div>
    </section>
  )
}

function CapabilityDecoration(props: {
  type: (typeof SECONDARY_CAPABILITIES)[number]['decoration']
  color: string
}) {
  if (props.type === 'trend') {
    return (
      <svg
        aria-hidden='true'
        viewBox='0 0 128 34'
        className='absolute right-5 bottom-5 h-8 w-28'
      >
        <path
          d='M2 29 C25 27 31 21 48 22 C69 23 82 11 99 14 C111 16 116 9 126 5'
          fill='none'
          stroke={props.color}
          strokeWidth='3'
          strokeLinecap='round'
        />
      </svg>
    )
  }

  if (props.type === 'network') {
    return (
      <svg
        aria-hidden='true'
        viewBox='0 0 128 34'
        className='absolute right-5 bottom-5 h-8 w-28'
      >
        <path
          d='M20 17 H108'
          stroke={props.color}
          strokeWidth='2'
          opacity='.45'
        />
        {[20, 64, 108].map((x) => (
          <circle key={x} cx={x} cy='17' r='6' fill={props.color} />
        ))}
      </svg>
    )
  }

  if (props.type === 'gallery') {
    return (
      <div
        aria-hidden='true'
        className='absolute right-5 bottom-5 flex items-end gap-2'
      >
        {[0.45, 0.72, 1].map((opacity, index) => (
          <span
            key={opacity}
            className='h-5 rounded-md'
            style={{
              backgroundColor: props.color,
              opacity,
              width: `${2.4 + index * 0.35}rem`,
            }}
          />
        ))}
      </div>
    )
  }

  return (
    <div
      aria-hidden='true'
      className='absolute right-5 bottom-6 h-3 w-32 overflow-hidden rounded-full bg-white/60 dark:bg-white/10'
    >
      <span
        className='absolute inset-y-0 left-0 w-[62%] rounded-full'
        style={{ backgroundColor: props.color, opacity: 0.48 }}
      />
      <span
        className='absolute top-1/2 left-[62%] size-3 -translate-x-1/2 -translate-y-1/2 rounded-full'
        style={{ backgroundColor: props.color }}
      />
    </div>
  )
}
