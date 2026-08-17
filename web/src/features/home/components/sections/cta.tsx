/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { Link } from '@tanstack/react-router'
import { ArrowRight, BookOpen, ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

interface CTAProps {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [openIndex, setOpenIndex] = useState(0)
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const faqs = [
    ['Home FAQ question 1', 'Home FAQ answer 1'],
    ['Home FAQ question 2', 'Home FAQ answer 2'],
    ['Home FAQ question 3', 'Home FAQ answer 3'],
    ['Home FAQ question 4', 'Home FAQ answer 4'],
  ]

  return (
    <section
      id='faq'
      className='border-border/70 scroll-mt-28 border-b bg-[#f6f6f5] px-4 pt-20 pb-20 sm:px-6 md:pt-28 md:pb-24 dark:bg-[#11110f]'
    >
      <div className='mx-auto max-w-7xl'>
        <div className='grid gap-12 lg:grid-cols-[0.8fr_1.2fr] lg:gap-20'>
          <AnimateInView>
            <p className='text-[10px] font-semibold tracking-[0.2em] text-[#a46d1c] uppercase dark:text-amber-300'>
              {t('Home FAQ label')}
            </p>
            <h2 className='mt-4 font-sans text-3xl leading-[1.08] font-semibold tracking-[-0.05em] text-[#211d18] md:text-5xl dark:text-[#f5f1e9]'>
              {t('Home FAQ heading')}
            </h2>
            <p className='mt-5 max-w-md text-sm leading-7 text-[#766d63] dark:text-white/55'>
              {t('Home FAQ intro')}
            </p>
            <div className='mt-8 flex flex-wrap gap-3'>
              <Button
                variant='outline'
                className='h-10 rounded-full border-[#cfc6b8] bg-white/40 px-4 text-sm hover:border-[#b7791f] dark:border-white/15 dark:bg-white/[0.04]'
                render={<Link to='/pricing' />}
              >
                {t('View Pricing')}
              </Button>
              <Button
                variant='ghost'
                className='h-10 rounded-full px-4 text-sm text-[#6f655a] hover:bg-white/60 dark:text-white/55 dark:hover:bg-white/[0.06]'
                render={
                  <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
                }
              >
                <BookOpen className='mr-1.5 size-4' />
                {t('Docs')}
              </Button>
            </div>
          </AnimateInView>

          <div className='border-t border-[#ded8ce] dark:border-white/10'>
            {faqs.map(([questionKey, answerKey], index) => {
              const open = openIndex === index
              return (
                <button
                  key={questionKey}
                  type='button'
                  onClick={() => setOpenIndex(open ? -1 : index)}
                  className='block w-full border-b border-[#ded8ce] py-5 text-left dark:border-white/10'
                  aria-expanded={open}
                >
                  <span className='flex items-center justify-between gap-5 text-sm font-semibold'>
                    {t(questionKey)}
                    <ChevronDown
                      className={`size-4 shrink-0 text-[#b7791f] transition-transform dark:text-amber-300 ${open ? 'rotate-180' : ''}`}
                    />
                  </span>
                  <span
                    className={`block overflow-hidden pr-8 text-sm leading-6 text-[#82786c] transition-all dark:text-white/45 ${open ? 'mt-3 max-h-40 opacity-100' : 'max-h-0 opacity-0'}`}
                  >
                    {t(answerKey)}
                  </span>
                </button>
              )
            })}
          </div>
        </div>

        <div className='mt-16 text-center'>
          <p className='text-[10px] font-semibold tracking-[0.2em] text-[#a46d1c] uppercase dark:text-amber-300'>
            {t('Home CTA label')}
          </p>
          <h2 className='mx-auto mt-4 max-w-3xl font-sans text-3xl leading-[1.05] font-semibold tracking-[-0.05em] text-[#211d18] md:text-5xl dark:text-[#f5f1e9]'>
            {t('Home CTA heading')}
          </h2>
          <p className='mx-auto mt-5 max-w-xl text-sm leading-7 text-[#766d63] dark:text-white/55'>
            {t('Home CTA description')}
          </p>
          <div className='mt-8 flex flex-wrap justify-center gap-3'>
            <Button
              className='group h-10 rounded-full bg-[#1f1b17] px-5 text-sm text-white hover:bg-[#3b332b] dark:bg-[#f5f1e9] dark:text-[#1f1b17] dark:hover:bg-white'
              render={
                <Link to={props.isAuthenticated ? '/dashboard' : '/sign-up'} />
              }
            >
              {props.isAuthenticated ? t('Go to Dashboard') : t('Get Started')}
              <ArrowRight className='ml-1.5 size-4 transition-transform group-hover:translate-x-0.5' />
            </Button>
            <Button
              variant='outline'
              className='h-10 rounded-full border-[#cfc6b8] bg-white/40 px-5 text-sm hover:border-[#b7791f] dark:border-white/15 dark:bg-white/[0.04]'
              render={<Link to='/pricing' />}
            >
              {t('View Pricing')}
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}
