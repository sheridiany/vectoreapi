/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'
import { getLobeIcon } from '@/lib/lobe-icon'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

const PROVIDER_ROWS = [
  {
    direction: 'left',
    items: [
      { label: 'Anthropic', icon: 'Anthropic' },
      { label: 'Home provider qwen', icon: 'Qwen.Color' },
      { label: 'DeepSeek', icon: 'DeepSeek.Color' },
      { label: 'Google Gemini', icon: 'Gemini.Color' },
      { label: 'MiniMax', icon: 'Minimax.Color' },
      { label: 'Kimi', icon: 'Kimi.Color' },
      { label: 'OpenAI', icon: 'OpenAI' },
      { label: 'Home provider xiaomi mimo', icon: 'XiaomiMiMo' },
      { label: 'Home provider zhipu', icon: 'Zhipu.Color' },
    ],
  },
  {
    direction: 'right',
    items: [
      { label: 'Cherry Studio', icon: 'CherryStudio.Color' },
      { label: 'ClackyAI', icon: 'ClackyAI' },
      { label: 'Cline', icon: 'Cline' },
      { label: 'Codex', icon: 'Codex.Color' },
      { label: 'Grok', icon: 'Grok' },
      { label: 'Mastra', icon: 'Mastra' },
      { label: 'Obsidian', icon: 'Obsidian.Color' },
      { label: 'OpenRouter', icon: 'OpenRouter.Color' },
    ],
  },
]

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'

  return (
    <section
      id='hero'
      className='border-border/70 relative scroll-mt-28 overflow-x-clip border-b bg-[#f6f6f5] px-4 pt-24 pb-14 sm:px-6 md:pt-32 md:pb-16 dark:bg-[#11110f]'
    >
      <div
        aria-hidden
        className='pointer-events-none absolute inset-0 opacity-70 dark:opacity-20'
        style={{
          background:
            'radial-gradient(circle at 50% 10%, rgba(183,121,31,0.12), transparent 33%), radial-gradient(circle at 15% 45%, rgba(255,255,255,0.9), transparent 26%)',
        }}
      />
      <div
        aria-hidden
        className='home-hero-diagonal-grid pointer-events-none absolute inset-[-25%] opacity-50 dark:opacity-20'
      />

      <div className='relative mx-auto max-w-7xl'>
        <div className='mx-auto mt-12 max-w-4xl text-center md:mt-16'>
          <div className='text-sm font-medium tracking-tight text-[#b66d1e] dark:text-amber-300'>
            {t('Home hero eyebrow')}
          </div>
          <h1 className='mt-4 font-serif text-[clamp(3rem,4.7vw,5.25rem)] leading-[1] font-medium tracking-[-0.04em] text-balance text-[#211d18] dark:text-[#f5f1e9]'>
            {t('Home hero title lead')}
            <br />
            {t('Home hero title prefix')}{' '}
            <span className='text-[#c87326] dark:text-orange-300'>
              {t('Home hero title highlight')}
            </span>
          </h1>
          <p className='mx-auto mt-6 max-w-2xl text-sm leading-7 text-[#6f675d] md:text-base dark:text-white/60'>
            {t('Home hero description')}
          </p>

          <div className='mt-6 flex flex-wrap items-center justify-center gap-4'>
            {props.isAuthenticated ? (
              <Button
                className='group h-10 rounded-xl bg-[#1f1b17] px-6 text-sm text-white hover:opacity-90 dark:bg-[#f5f1e9] dark:text-[#1f1b17] dark:hover:bg-white'
                render={<Link to='/dashboard' />}
              >
                {t('Home hero enter console')}
                <ArrowRight className='ml-1.5 size-4 transition-transform group-hover:translate-x-0.5' />
              </Button>
            ) : (
              <Button
                className='group h-10 rounded-xl bg-[#1f1b17] px-6 text-sm text-white hover:opacity-90 dark:bg-[#f5f1e9] dark:text-[#1f1b17] dark:hover:bg-white'
                render={<Link to='/sign-in' />}
              >
                {t('Home hero enter console')}
                <ArrowRight className='ml-1.5 size-4 transition-transform group-hover:translate-x-0.5' />
              </Button>
            )}
            <Button
              variant='outline'
              className='h-10 rounded-xl border-[#d8d1c4] bg-white/40 px-6 text-sm text-[#3c352e] hover:border-[#b7791f] hover:bg-white dark:border-white/15 dark:bg-white/[0.04] dark:text-white/80 dark:hover:border-amber-300/60 dark:hover:bg-white/[0.08]'
              render={
                <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
              }
            >
              {t('Home hero read docs')}
            </Button>
          </div>
        </div>

        <div
          id='capabilities'
          className='mx-auto mt-6 flex max-w-4xl scroll-mt-28 flex-wrap items-center justify-center gap-3 text-center text-xs text-[#6f675d] sm:text-sm dark:text-white/60'
        >
          {[
            'Home capability unified api',
            'Home capability provider routing',
            'Home capability model mapping',
            'Home capability traffic visibility',
          ].map((label) => (
            <span
              key={label}
              className='rounded-full border border-[#d8d1c4] px-3 py-1 dark:border-white/10'
            >
              {t(label)}
            </span>
          ))}
        </div>

        <div
          id='providers'
          className='relative mt-16 scroll-mt-28 border-y border-[#ded8ce] pt-28 pb-8 md:pt-32 dark:border-white/10'
        >
          <p className='mb-4 text-center text-[10px] font-semibold tracking-[0.2em] text-[#948a7d] uppercase dark:text-white/40'>
            {t('Home providers label')}
          </p>
          <div className='relative space-y-3 overflow-hidden text-[#746b61] dark:text-white/45'>
            <div
              aria-hidden='true'
              className='pointer-events-none absolute inset-y-0 left-0 z-10 w-12 bg-gradient-to-r from-[#f6f6f5] to-transparent dark:from-[#11110f]'
            />
            <div
              aria-hidden='true'
              className='pointer-events-none absolute inset-y-0 right-0 z-10 w-12 bg-gradient-to-l from-[#f6f6f5] to-transparent dark:from-[#11110f]'
            />
            {PROVIDER_ROWS.map(({ direction, items }) => (
              <div
                key={direction}
                className='home-logo-marquee overflow-hidden'
              >
                <div
                  className={`home-logo-marquee__track home-logo-marquee__track--${direction}`}
                >
                  {[0, 1, 2].map((copy) => (
                    <div
                      key={`${direction}-${copy}`}
                      className='home-logo-marquee__set'
                      aria-hidden={copy > 0}
                    >
                      {items.map(({ label, icon }) => (
                        <span
                          key={`${direction}-${copy}-${label}`}
                          className='home-logo-marquee__item flex items-center gap-3 text-base font-semibold tracking-tight whitespace-nowrap'
                        >
                          <span className='home-logo-marquee__mark opacity-85 transition-opacity duration-200 hover:opacity-100'>
                            {getLobeIcon(icon, 42)}
                          </span>
                          {t(label)}
                        </span>
                      ))}
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
